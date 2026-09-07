package execution

// Phase6.5.7.5.2.1 Execution Report Runtime Acceptance Harness (tests only).
//
// Drives ExecutionReportConsumer.ConsumeExecutionReport with scripted report sequences and
// asserts Order runtime / Fill count / Position delta / Frozen Spec immutability.
//
// Does not modify ExecutionReportConsumer, the report handler, OMS enum, schema,
// TradePlan, Intent, or Frozen Spec. No migration. No real broker.

import (
	"context"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// ---------- harness types ----------

// acceptanceSpec is the frozen source used to build the SubmitIntent (read-only).
type acceptanceSpec struct {
	AccountID   uint
	StockCode   string
	StockName   string
	Side        string
	LimitPrice  float64
	TargetVolum int64
}

func (s acceptanceSpec) toItem() models.TradePlanItem {
	return models.TradePlanItem{
		ID:           7,
		PlanID:       1,
		StockCode:    s.StockCode,
		StockName:    s.StockName,
		Side:         s.Side,
		LimitPrice:   s.LimitPrice,
		TargetVolume: s.TargetVolum,
		Status:       models.TradePlanItemPending,
	}
}

// acceptanceSnapshot is the observable runtime state after a step.
type acceptanceSnapshot struct {
	OMS           string
	BrokerStatus  string
	BrokerOrderID string
	FilledVolume  int64
	Leaves        int64
	FillCount     int
	Position      int64
	// Spec projection (must stay immutable across the whole script).
	Price     float64
	Volume    int64
	Side      string
	StockCode string
}

// acceptanceStep is one scripted report plus its expectations.
type acceptanceStep struct {
	name    string
	report  ExecutionReport
	wantErr error // nil = must succeed

	wantOMS          string
	wantBrokerStatus string
	wantFilledVolume int64
	wantLeaves       int64
	wantFillCount    int
	wantPosDelta     int64 // position change caused by this step
}

// acceptanceRun holds harness wiring for one scenario.
type acceptanceRun struct {
	broker    *RealBroker
	consumer  *ExecutionReportConsumer
	accountnt *InMemoryPositionAccountant
	clientID  string
	accountID string
	spec      acceptanceSpec
	specItem  models.TradePlanItem
	base      acceptanceSnapshot
}

func newAcceptanceRun(t *testing.T, spec acceptanceSpec) *acceptanceRun {
	t.Helper()
	rb := NewRealBroker()
	acc := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(rb, acc)
	require.NotNil(t, consumer)

	specItem := spec.toItem()
	order, err := rb.Submit(context.Background(), SubmitIntent{
		AccountID: spec.AccountID,
		StockCode: spec.StockCode,
		StockName: spec.StockName,
		Side:      spec.Side,
		Price:     spec.LimitPrice,  // Frozen Spec projection
		Volume:    spec.TargetVolum, // Frozen Spec projection
	})
	require.NoError(t, err)

	run := &acceptanceRun{
		broker:    rb,
		consumer:  consumer,
		accountnt: acc,
		clientID:  order.ClientOrderID,
		accountID: order.AccountID,
		spec:      spec,
		specItem:  specItem,
	}
	run.base = run.snapshot(t)

	require.Equal(t, data.PaperOrderStatusPending, run.base.OMS)
	require.Equal(t, int64(0), run.base.FilledVolume)
	require.Equal(t, 0, run.base.FillCount)
	require.Equal(t, int64(0), run.base.Position)
	return run
}

func (r *acceptanceRun) snapshot(t *testing.T) acceptanceSnapshot {
	t.Helper()
	ctx := context.Background()
	order, err := r.broker.QueryOrder(ctx, r.clientID)
	require.NoError(t, err)
	fills, err := r.broker.ListFills(ctx, r.clientID)
	require.NoError(t, err)
	pos, _ := r.accountnt.Position(r.accountID, r.spec.StockCode)

	return acceptanceSnapshot{
		OMS:           order.Status,
		BrokerStatus:  order.BrokerStatus,
		BrokerOrderID: order.BrokerOrderID,
		FilledVolume:  order.FilledVolume,
		Leaves:        order.LeavesQuantity,
		FillCount:     len(fills),
		Position:      pos.Volume,
		Price:         order.Price,
		Volume:        order.Volume,
		Side:          order.Side,
		StockCode:     order.StockCode,
	}
}

// assertSpecImmutable proves the report path never rewrote the Frozen Spec projection,
// nor the in-memory Spec item the harness built the intent from.
func (r *acceptanceRun) assertSpecImmutable(t *testing.T, step string, snap acceptanceSnapshot) {
	t.Helper()
	require.InDelta(t, r.spec.LimitPrice, snap.Price, 1e-9, "%s: limit_price projection mutated", step)
	require.Equal(t, r.spec.TargetVolum, snap.Volume, "%s: target_volume projection mutated", step)
	require.Equal(t, r.spec.Side, snap.Side, "%s: side mutated", step)
	require.Equal(t, r.spec.StockCode, snap.StockCode, "%s: stock_code mutated", step)

	require.Equal(t, r.spec.toItem(), r.specItem, "%s: Frozen Spec item mutated", step)
	require.InDelta(t, r.spec.LimitPrice, r.specItem.LimitPrice, 1e-9)
	require.Equal(t, r.spec.TargetVolum, r.specItem.TargetVolume)
	require.Equal(t, models.TradePlanItemPending, r.specItem.Status, "%s: Intent/item status mutated", step)
}

// run executes the script and asserts every step.
func (r *acceptanceRun) run(t *testing.T, steps []acceptanceStep) acceptanceSnapshot {
	t.Helper()
	prev := r.base
	for _, step := range steps {
		report := step.report
		if report.ClientOrderID == "" {
			report.ClientOrderID = r.clientID
		}

		err := r.consumer.ConsumeExecutionReport(context.Background(), report)
		if step.wantErr != nil {
			require.ErrorIs(t, err, step.wantErr, "%s: expected rejection", step.name)
		} else {
			require.NoError(t, err, "%s: expected success", step.name)
		}

		snap := r.snapshot(t)

		require.Equal(t, step.wantOMS, snap.OMS, "%s: OMS", step.name)
		require.Equal(t, step.wantBrokerStatus, snap.BrokerStatus, "%s: broker_status", step.name)
		require.Equal(t, step.wantFilledVolume, snap.FilledVolume, "%s: filled_volume", step.name)
		require.Equal(t, step.wantLeaves, snap.Leaves, "%s: leaves", step.name)
		require.Equal(t, step.wantFillCount, snap.FillCount, "%s: fill count", step.name)

		// Position delta must equal the fill quantity applied by this step.
		require.Equal(t, step.wantPosDelta, snap.Position-prev.Position, "%s: position delta", step.name)

		// OMS enum must not be extended by report consumption.
		require.True(t, data.IsPaperOMSStatus(snap.OMS), "%s: OMS outside frozen enum", step.name)
		for _, forbidden := range []string{
			data.BrokerStatusAccepted, data.BrokerStatusWorking, data.BrokerStatusPartiallyFilled,
			data.BrokerStatusTimeout, data.BrokerStatusUnknown,
		} {
			require.NotEqual(t, forbidden, snap.OMS, "%s: broker detail leaked into OMS", step.name)
		}

		r.assertSpecImmutable(t, step.name, snap)
		prev = snap
	}

	// Global identity: position == sum of accepted unique fills.
	fills, err := r.broker.ListFills(context.Background(), r.clientID)
	require.NoError(t, err)
	var sum int64
	for _, f := range fills {
		sum += f.FillQty
	}
	require.Equal(t, sum, prev.Position, "position must equal sum of unique fills")
	require.Equal(t, prev.FilledVolume, sum, "order cum qty must equal sum of unique fills")
	return prev
}

func defaultAcceptanceSpec(code string) acceptanceSpec {
	return acceptanceSpec{
		AccountID:   1,
		StockCode:   code,
		StockName:   "验收标的",
		Side:        "buy",
		LimitPrice:  10.0,
		TargetVolum: 100,
	}
}

// ---------- P1: ACK → TRADE(full) → FILLED ----------

func TestAcceptance_P1_AckTradeFilled(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000101"))
	final := run.run(t, []acceptanceStep{
		{
			name:             "ACK",
			report:           ExecutionReport{ReportType: ExecReportTypeACK, ReportID: "p1-ack"},
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusWorking,
			wantLeaves:       100,
		},
		{
			name: "TRADE full",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "p1-tr", ExecID: "P1E1",
				LastQty: 100, LastPrice: 10,
			},
			wantOMS:          data.PaperOrderStatusFilled,
			wantBrokerStatus: data.BrokerStatusFilled,
			wantFilledVolume: 100,
			wantLeaves:       0,
			wantFillCount:    1,
			wantPosDelta:     100,
		},
	})
	require.NotEmpty(t, final.BrokerOrderID, "ACK must bind broker_order_id")
}

// ---------- P2: partial TRADE → full TRADE ----------

func TestAcceptance_P2_PartialThenFull(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000102"))
	run.run(t, []acceptanceStep{
		{
			name: "TRADE partial 30",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "p2-a", ExecID: "P2E1",
				LastQty: 30, LastPrice: 10,
			},
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusPartiallyFilled,
			wantFilledVolume: 30,
			wantLeaves:       70,
			wantFillCount:    1,
			wantPosDelta:     30,
		},
		{
			name: "TRADE remaining 70",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "p2-b", ExecID: "P2E2",
				LastQty: 70, LastPrice: 11,
			},
			wantOMS:          data.PaperOrderStatusFilled,
			wantBrokerStatus: data.BrokerStatusFilled,
			wantFilledVolume: 100,
			wantLeaves:       0,
			wantFillCount:    2,
			wantPosDelta:     70,
		},
	})

	// Weighted average is recomputed locally; the limit price stays untouched.
	order, err := run.broker.QueryOrder(context.Background(), run.clientID)
	require.NoError(t, err)
	require.InDelta(t, 10.7, order.FilledPrice, 1e-9)
	require.InDelta(t, 10.0, order.Price, 1e-9)

	pos, ok := run.accountnt.Position(run.accountID, run.spec.StockCode)
	require.True(t, ok)
	require.InDelta(t, order.FilledPrice, pos.AvgCost, 1e-9, "position avg cost must match order avg fill")
}

// ---------- P3: duplicate TRADE retransmit (same report_id + exec_id) ----------

func TestAcceptance_P3_DuplicateTradeRetransmit(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000103"))
	dup := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p3-tr", ExecID: "P3E1",
		LastQty: 40, LastPrice: 10,
	}
	run.run(t, []acceptanceStep{
		{
			name: "TRADE first", report: dup,
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusPartiallyFilled,
			wantFilledVolume: 40,
			wantLeaves:       60,
			wantFillCount:    1,
			wantPosDelta:     40,
		},
		{
			name: "TRADE retransmit", report: dup,
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusPartiallyFilled,
			wantFilledVolume: 40,
			wantLeaves:       60,
			wantFillCount:    1,
			wantPosDelta:     0, // no double accounting
		},
	})
}

// ---------- P4: duplicate ACK (same report_id) ----------

func TestAcceptance_P4_DuplicateAck(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000104"))
	ack := ExecutionReport{ReportType: ExecReportTypeACK, ReportID: "p4-ack"}

	first := []acceptanceStep{{
		name: "ACK first", report: ack,
		wantOMS:          data.PaperOrderStatusPending,
		wantBrokerStatus: data.BrokerStatusWorking,
		wantLeaves:       100,
	}}
	run.run(t, first)
	bid := run.snapshot(t).BrokerOrderID
	require.NotEmpty(t, bid)

	run.base = run.snapshot(t)
	run.run(t, []acceptanceStep{{
		name: "ACK duplicate", report: ack,
		wantOMS:          data.PaperOrderStatusPending,
		wantBrokerStatus: data.BrokerStatusWorking,
		wantLeaves:       100,
	}})
	require.Equal(t, bid, run.snapshot(t).BrokerOrderID, "duplicate ACK must not rotate broker_order_id")
}

// ---------- P5: CANCEL (keeps executed quantity) ----------

func TestAcceptance_P5_Cancel(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000105"))
	run.run(t, []acceptanceStep{
		{
			name: "TRADE partial 30",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "p5-tr", ExecID: "P5E1",
				LastQty: 30, LastPrice: 10,
			},
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusPartiallyFilled,
			wantFilledVolume: 30,
			wantLeaves:       70,
			wantFillCount:    1,
			wantPosDelta:     30,
		},
		{
			name:             "CANCEL",
			report:           ExecutionReport{ReportType: ExecReportTypeCANCEL, ReportID: "p5-cx"},
			wantOMS:          data.PaperOrderStatusCancelled,
			wantBrokerStatus: data.BrokerStatusCancelled,
			wantFilledVolume: 30,
			wantLeaves:       0,
			wantFillCount:    1,
			wantPosDelta:     0,
		},
	})
}

// ---------- N1: REJECT after filled ----------

func TestAcceptance_N1_RejectAfterFilled(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000106"))
	run.run(t, []acceptanceStep{
		{
			name: "TRADE full",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "n1-tr", ExecID: "N1E1",
				LastQty: 100, LastPrice: 10,
			},
			wantOMS:          data.PaperOrderStatusFilled,
			wantBrokerStatus: data.BrokerStatusFilled,
			wantFilledVolume: 100,
			wantLeaves:       0,
			wantFillCount:    1,
			wantPosDelta:     100,
		},
		{
			name: "late REJECT",
			report: ExecutionReport{
				ReportType: ExecReportTypeREJECT, ReportID: "n1-rj",
				RejectCode: "LATE", RejectReason: "late reject after fill",
			},
			wantErr:          ErrReportInvalidTransition,
			wantOMS:          data.PaperOrderStatusFilled,
			wantBrokerStatus: data.BrokerStatusFilled,
			wantFilledVolume: 100,
			wantLeaves:       0,
			wantFillCount:    1,
			wantPosDelta:     0,
		},
	})
}

// ---------- N2: TRADE after cancelled ----------

func TestAcceptance_N2_TradeAfterCancelled(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000107"))
	run.run(t, []acceptanceStep{
		{
			name:             "CANCEL",
			report:           ExecutionReport{ReportType: ExecReportTypeCANCEL, ReportID: "n2-cx"},
			wantOMS:          data.PaperOrderStatusCancelled,
			wantBrokerStatus: data.BrokerStatusCancelled,
			wantLeaves:       0,
		},
		{
			name: "late TRADE",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "n2-tr", ExecID: "N2E1",
				LastQty: 20, LastPrice: 10,
			},
			wantErr:          ErrReportInvalidTransition,
			wantOMS:          data.PaperOrderStatusCancelled,
			wantBrokerStatus: data.BrokerStatusCancelled,
			wantLeaves:       0,
			wantFillCount:    0,
			wantPosDelta:     0,
		},
	})
}

// ---------- N3: duplicate exec_id under a different report_id ----------

func TestAcceptance_N3_DuplicateExecID(t *testing.T) {
	run := newAcceptanceRun(t, defaultAcceptanceSpec("sz000108"))
	run.run(t, []acceptanceStep{
		{
			name: "TRADE exec E",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "n3-r1", ExecID: "N3E",
				LastQty: 50, LastPrice: 10,
			},
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusPartiallyFilled,
			wantFilledVolume: 50,
			wantLeaves:       50,
			wantFillCount:    1,
			wantPosDelta:     50,
		},
		{
			name: "same exec_id, new report_id",
			report: ExecutionReport{
				ReportType: ExecReportTypeTRADE, ReportID: "n3-r2", ExecID: "N3E",
				LastQty: 50, LastPrice: 10,
			},
			wantOMS:          data.PaperOrderStatusPending,
			wantBrokerStatus: data.BrokerStatusPartiallyFilled,
			wantFilledVolume: 50,
			wantLeaves:       50,
			wantFillCount:    1,
			wantPosDelta:     0, // exec_id dedup
		},
	})
}

// ---------- Paper / RealStub parity ----------

// Paper has no ExecutionReport adapter yet (design 6.5.7.5 §5.2), so the same
// ConsumeExecutionReport script cannot be replayed on the Paper backend.
// This case stays explicit instead of silently passing.
func TestAcceptance_PaperRealStubParity(t *testing.T) {
	t.Skip("N/A: PaperReportAdapter not implemented; parity deferred to the Paper adapter slice")
}
