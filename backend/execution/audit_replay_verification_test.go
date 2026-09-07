package execution

// Phase6.5.7.6.3 Execution Audit Replay Verification Harness (tests only).
//
// Rebuilds Order runtime / Fill quantity / Position delta from L0 Audit Events,
// then compares replay output with live runtime. It does not modify production
// logic, schemas, OMS enums, or Frozen Spec.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/execution/audit"

	"github.com/stretchr/testify/require"
)

type auditReplayOrder struct {
	StockCode      string
	Side           string
	Price          float64
	Volume         int64
	Status         string
	BrokerStatus   string
	FilledVolume   int64
	FilledPrice    float64
	LeavesQuantity int64
	RejectCode     string
	RejectReason   string
}

type auditReplayResult struct {
	Order         auditReplayOrder
	FillQty       int64
	FillCount     int
	PositionDelta int64
	SpecHash      string
	EventsApplied int
}

// replayAuditEvents is an isolated, deterministic test oracle.
// REPORT_RECEIVED is the external input fact; FILL_APPLIED and ORDER_TERMINAL
// are effect events used to reconstruct and verify the materialized runtime.
func replayAuditEvents(events []audit.Event) (auditReplayResult, error) {
	var out auditReplayResult
	seenEventIDs := make(map[string]struct{}, len(events))
	seenExecIDs := make(map[string]struct{})

	for _, ev := range events {
		if ev.EventID == "" {
			return out, errors.New("audit replay: empty event_id")
		}
		if _, duplicate := seenEventIDs[ev.EventID]; duplicate {
			continue
		}
		seenEventIDs[ev.EventID] = struct{}{}
		out.EventsApplied++

		if out.SpecHash == "" {
			out.SpecHash = ev.SpecHash
		} else if ev.SpecHash != out.SpecHash {
			return out, fmt.Errorf("audit replay: spec_hash drift %q != %q", ev.SpecHash, out.SpecHash)
		}

		switch ev.EventType {
		case audit.TypeSubmitAttempt:
			out.Order.StockCode = payloadString(ev.Payload, "symbol")
			out.Order.Side = payloadString(ev.Payload, "side")
			out.Order.Price = payloadFloat64(ev.Payload, "limit_price")
			out.Order.Volume = payloadInt64(ev.Payload, "target_volume")
			out.Order.LeavesQuantity = out.Order.Volume

		case audit.TypeSubmitAccepted:
			out.Order.Status = payloadString(ev.Payload, "oms_status")
			out.Order.BrokerStatus = payloadString(ev.Payload, "broker_status")

		case audit.TypeSubmitRejected:
			out.Order.Status = payloadString(ev.Payload, "oms_status")
			out.Order.BrokerStatus = payloadString(ev.Payload, "broker_status")
			out.Order.RejectCode = payloadString(ev.Payload, "reject_code")
			out.Order.RejectReason = payloadString(ev.Payload, "reject_reason")

		case audit.TypeSubmitTimeout:
			// Contract event covers timeout|unknown; payload preserves exact L2 status.
			out.Order.Status = payloadString(ev.Payload, "oms_status")
			out.Order.BrokerStatus = payloadString(ev.Payload, "broker_status")

		case audit.TypeReportReceived:
			if payloadString(ev.Payload, "apply_result") != "applied" {
				continue
			}
			switch payloadString(ev.Payload, "report_type") {
			case ExecReportTypeACK:
				out.Order.Status = data.PaperOrderStatusPending
				out.Order.BrokerStatus = data.BrokerStatusWorking
			}

		case audit.TypeFillApplied:
			if _, duplicate := seenExecIDs[ev.ExecID]; duplicate {
				continue
			}
			seenExecIDs[ev.ExecID] = struct{}{}
			qty := payloadInt64(ev.Payload, "fill_qty")
			out.FillQty += qty
			out.FillCount++
			out.PositionDelta += payloadInt64(ev.Payload, "position_delta")
			out.Order.FilledVolume = payloadInt64(ev.Payload, "cum_qty")
			out.Order.FilledPrice = payloadFloat64(ev.Payload, "avg_price")
			out.Order.LeavesQuantity = payloadInt64(ev.Payload, "leaves_quantity")
			out.Order.Status = payloadString(ev.Payload, "oms_status_after")
			out.Order.BrokerStatus = payloadString(ev.Payload, "broker_status_after")

		case audit.TypeOrderTerminal:
			out.Order.Status = payloadString(ev.Payload, "terminal_status")
			out.Order.BrokerStatus = payloadString(ev.Payload, "broker_status")
			out.Order.FilledVolume = payloadInt64(ev.Payload, "filled_volume")
			out.Order.FilledPrice = payloadFloat64(ev.Payload, "avg_fill_price")
			out.Order.LeavesQuantity = payloadInt64(ev.Payload, "leaves_quantity")
			if v := payloadString(ev.Payload, "reject_code"); v != "" {
				out.Order.RejectCode = v
			}
			if v := payloadString(ev.Payload, "reject_reason"); v != "" {
				out.Order.RejectReason = v
			}
		}
	}
	return out, nil
}

func payloadString(payload map[string]any, key string) string {
	v, _ := payload[key].(string)
	return v
}

func payloadInt64(payload map[string]any, key string) int64 {
	switch v := payload[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func payloadFloat64(payload map[string]any, key string) float64 {
	switch v := payload[key].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0
	}
}

func requireReplayMatchesRuntime(
	t *testing.T,
	replayed auditReplayResult,
	runtimeOrder *broker.TradeOrder,
	runtimeFillQty int64,
	runtimeFillCount int,
	runtimePositionDelta int64,
) {
	t.Helper()
	require.NotNil(t, runtimeOrder)
	require.Equal(t, runtimeOrder.StockCode, replayed.Order.StockCode)
	require.Equal(t, runtimeOrder.Side, replayed.Order.Side)
	require.InDelta(t, runtimeOrder.Price, replayed.Order.Price, 1e-9)
	require.Equal(t, runtimeOrder.Volume, replayed.Order.Volume)
	require.Equal(t, runtimeOrder.Status, replayed.Order.Status)
	require.Equal(t, runtimeOrder.BrokerStatus, replayed.Order.BrokerStatus)
	require.Equal(t, runtimeOrder.FilledVolume, replayed.Order.FilledVolume)
	require.InDelta(t, runtimeOrder.FilledPrice, replayed.Order.FilledPrice, 1e-9)
	require.Equal(t, runtimeOrder.LeavesQuantity, replayed.Order.LeavesQuantity)
	require.Equal(t, runtimeOrder.RejectCode, replayed.Order.RejectCode)
	require.Equal(t, runtimeOrder.RejectReason, replayed.Order.RejectReason)
	require.Equal(t, runtimeFillQty, replayed.FillQty)
	require.Equal(t, runtimeFillCount, replayed.FillCount)
	require.Equal(t, runtimePositionDelta, replayed.PositionDelta)
}

func auditReplayFixture(t *testing.T) (*RealBroker, *ExecutionReportConsumer, *audit.MemorySink, *InMemoryPositionAccountant) {
	t.Helper()
	mem := audit.NewMemorySink()
	rb := NewRealBroker()
	rb.SetAuditEmitter(audit.NewEmitter(mem))
	accountant := NewInMemoryPositionAccountant()
	rb.SetPositionAccountant(accountant)
	consumer := NewExecutionReportConsumer(rb, accountant)
	require.NotNil(t, consumer)
	return rb, consumer, mem, accountant
}

func TestAuditReplay_AcceptedReportFillTerminal_EqualsRuntime(t *testing.T) {
	rb, consumer, mem, accountant := auditReplayFixture(t)
	const (
		symbol      = "sz000301"
		limitPrice  = 10.0
		targetVol   = int64(100)
	)
	specHash := audit.SpecFingerprint(symbol, "buy", limitPrice, targetVol)
	order, err := rb.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: symbol, StockName: "Replay", Side: "buy",
		Price: limitPrice, Volume: targetVol, SpecHash: specHash,
	})
	require.NoError(t, err)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "replay-ack",
		ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "replay-trade-1", ExecID: "RE1",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 10.0,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "replay-trade-2", ExecID: "RE2",
		ClientOrderID: order.ClientOrderID, LastQty: 70, LastPrice: 11.0,
	}))

	runtimeOrder, err := rb.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	runtimeFills, err := rb.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	var runtimeFillQty int64
	for _, fill := range runtimeFills {
		runtimeFillQty += fill.FillQty
	}
	position, ok := accountant.Position(runtimeOrder.AccountID, symbol)
	require.True(t, ok)

	events := mem.Events()
	require.Equal(t, []string{
		audit.TypeSubmitAttempt,
		audit.TypeSubmitAccepted,
		audit.TypeReportReceived, // ACK
		audit.TypeReportReceived, // partial TRADE
		audit.TypeFillApplied,
		audit.TypeReportReceived, // full TRADE
		audit.TypeFillApplied,
		audit.TypeOrderTerminal,
	}, auditTypes(events))

	replayed, err := replayAuditEvents(events)
	require.NoError(t, err)
	require.Equal(t, specHash, replayed.SpecHash)
	requireReplayMatchesRuntime(
		t, replayed, runtimeOrder,
		runtimeFillQty, len(runtimeFills), position.Volume,
	)

	// Frozen Spec projection remains immutable even when last fill price differs.
	require.InDelta(t, limitPrice, replayed.Order.Price, 1e-9)
	require.Equal(t, targetVol, replayed.Order.Volume)
	require.InDelta(t, limitPrice, runtimeOrder.Price, 1e-9)
	require.Equal(t, targetVol, runtimeOrder.Volume)
}

func TestAuditReplay_SubmitRejectedTerminal_EqualsRuntime(t *testing.T) {
	mem := audit.NewMemorySink()
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeRejected, Message: "risk reject"},
	}
	rb := NewRealBrokerWithAdapter(fake)
	rb.SetAuditEmitter(audit.NewEmitter(mem))

	order, err := rb.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000302", Side: "buy", Price: 12, Volume: 200,
	})
	require.ErrorIs(t, err, ErrBrokerRejected)
	require.NotNil(t, order)

	events := mem.Events()
	require.Equal(t, []string{
		audit.TypeSubmitAttempt,
		audit.TypeSubmitRejected,
		audit.TypeOrderTerminal,
	}, auditTypes(events))

	replayed, err := replayAuditEvents(events)
	require.NoError(t, err)
	requireReplayMatchesRuntime(t, replayed, order, 0, 0, 0)
	require.Equal(t, data.PaperOrderStatusRejected, replayed.Order.Status)
	require.Equal(t, data.BrokerStatusRejected, replayed.Order.BrokerStatus)
}

func TestAuditReplay_SubmitTimeout_EqualsRuntime(t *testing.T) {
	mem := audit.NewMemorySink()
	fake := &programmableAdapter{err: context.DeadlineExceeded}
	rb := NewRealBrokerWithAdapter(fake)
	rb.SetAuditEmitter(audit.NewEmitter(mem))

	order, err := rb.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000303", Side: "buy", Price: 9.5, Volume: 50,
	})
	require.ErrorIs(t, err, ErrBrokerTimeout)
	require.NotNil(t, order)

	events := mem.Events()
	require.Equal(t, []string{
		audit.TypeSubmitAttempt,
		audit.TypeSubmitTimeout,
	}, auditTypes(events))
	require.Equal(t, SubmitOutcomeTimeout, events[1].Payload["submit_outcome"])

	replayed, err := replayAuditEvents(events)
	require.NoError(t, err)
	requireReplayMatchesRuntime(t, replayed, order, 0, 0, 0)
	require.Equal(t, data.PaperOrderStatusPending, replayed.Order.Status)
	require.Equal(t, data.BrokerStatusTimeout, replayed.Order.BrokerStatus)
}

func TestAuditReplay_DuplicateReportAndExec_DoNotDoubleCount(t *testing.T) {
	rb, consumer, mem, accountant := auditReplayFixture(t)
	order, err := rb.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000304", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	first := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "dup-r1", ExecID: "DUP-E",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10,
	}
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), first))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), first))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "dup-r2", ExecID: "DUP-E",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10,
	}))

	runtimeOrder, err := rb.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	fills, err := rb.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	position, ok := accountant.Position(runtimeOrder.AccountID, runtimeOrder.StockCode)
	require.True(t, ok)

	replayed, err := replayAuditEvents(mem.Events())
	require.NoError(t, err)
	requireReplayMatchesRuntime(t, replayed, runtimeOrder, 40, len(fills), position.Volume)
	require.Equal(t, 1, replayed.FillCount)
	require.Equal(t, int64(40), replayed.PositionDelta)
}

func auditTypes(events []audit.Event) []string {
	out := make([]string, len(events))
	for i, ev := range events {
		out[i] = ev.EventType
	}
	return out
}
