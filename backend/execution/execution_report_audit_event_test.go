package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution/audit"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Phase6.5.8.2.2 — ExecutionReport / Fill / Order Terminal audit via AuditSink.
// Audit emission only; no OMS / Fill / Position / Spec mutations.

func setupReportAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:report_audit_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.MigrateAuditEvents(database))
	return database
}

func newReportAuditFixture(t *testing.T) (*RealBroker, *ExecutionReportConsumer, *audit.MemorySink, *gorm.DB, *InMemoryPositionAccountant) {
	t.Helper()
	database := setupReportAuditDB(t)
	mem := audit.NewMemoryAuditSink()
	real := NewRealBroker()
	real.SetAuditEmitter(audit.NewPersistenceEmitter(mem, database))
	acc := NewInMemoryPositionAccountant()
	real.SetPositionAccountant(acc)
	consumer := NewExecutionReportConsumer(real, acc)
	return real, consumer, mem, database, acc
}

func submitForReportAudit(t *testing.T, real *RealBroker, code string, price float64, vol int64) (cid, specHash string) {
	t.Helper()
	specHash = audit.SpecFingerprint(code, "buy", price, vol)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: code, Side: "buy",
		Price: price, Volume: vol, SpecHash: specHash,
	})
	require.NoError(t, err)
	return order.ClientOrderID, specHash
}

func countEventType(events []audit.Event, typ string) int {
	n := 0
	for _, e := range events {
		if e.EventType == typ {
			n++
		}
	}
	return n
}

func requireReportPayload(t *testing.T, e audit.Event, reportID, reportType, orderID string) {
	t.Helper()
	require.Equal(t, reportID, e.ReportID)
	require.Equal(t, reportID, e.Payload["report_id"])
	require.Equal(t, reportType, e.Payload["report_type"])
	require.Equal(t, orderID, e.LocalOrderID)
	require.Equal(t, orderID, e.Payload["order_id"])
}

func TestReportAudit_P1_AckTradeFilledChain(t *testing.T) {
	real, consumer, mem, database, acc := newReportAuditFixture(t)
	cid, specHash := submitForReportAudit(t, real, "sz000401", 10.0, 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p1-ack", ClientOrderID: cid,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p1-tr", ExecID: "P1E1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10.25,
	}))

	events := mem.Events()
	// SUBMIT_ATTEMPT + SUBMIT_ACCEPTED + ACK REPORT + TRADE REPORT + FILL + TERMINAL
	require.Equal(t, []string{
		audit.TypeSubmitAttempt,
		audit.TypeSubmitAccepted,
		audit.TypeReportReceived,
		audit.TypeReportReceived,
		audit.TypeFillApplied,
		audit.TypeOrderTerminal,
	}, auditEventTypes(mem))

	ack := events[2]
	trade := events[3]
	fill := events[4]
	term := events[5]

	requireReportPayload(t, ack, "p1-ack", ExecReportTypeACK, "1")
	requireReportPayload(t, trade, "p1-tr", ExecReportTypeTRADE, "1")
	require.Equal(t, "P1E1", trade.ExecID)
	require.Equal(t, "P1E1", trade.Payload["exec_id"])

	require.Equal(t, audit.IDFillApplied("P1E1"), fill.EventID)
	require.Equal(t, "P1E1", fill.Payload["exec_id"])
	require.EqualValues(t, 100, fill.Payload["fill_qty"])
	require.EqualValues(t, 10.25, fill.Payload["fill_price"])
	require.Equal(t, specHash, fill.SpecHash)
	require.Equal(t, specHash, fill.Payload["spec_hash"])

	require.Equal(t, data.PaperOrderStatusFilled, term.Payload["terminal_status"])
	require.Equal(t, "1", term.Payload["order_id"])

	for _, e := range events {
		require.Equal(t, specHash, e.SpecHash, e.EventType)
	}

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.InDelta(t, 10.0, got.Price, 1e-9)
	require.Equal(t, int64(100), got.Volume)
	pos, ok := acc.Position("1", "sz000401")
	require.True(t, ok)
	require.Equal(t, int64(100), pos.Volume)

	var rows []models.AuditEvent
	require.NoError(t, database.Order("id asc").Find(&rows).Error)
	require.GreaterOrEqual(t, len(rows), 6)
	var decoded audit.Event
	require.NoError(t, json.Unmarshal([]byte(rows[len(rows)-2].PayloadJSON), &decoded))
	require.Equal(t, audit.TypeFillApplied, decoded.EventType)
	require.Equal(t, "P1E1", decoded.Payload["exec_id"])
}

func TestReportAudit_P2_PartialThenFilled(t *testing.T) {
	real, consumer, mem, _, acc := newReportAuditFixture(t)
	cid, specHash := submitForReportAudit(t, real, "sz000402", 10.0, 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p2-a", ExecID: "P2E1",
		ClientOrderID: cid, LastQty: 40, LastPrice: 10.0,
	}))
	require.Equal(t, 1, countEventType(mem.Events(), audit.TypeFillApplied))
	require.Equal(t, 0, countEventType(mem.Events(), audit.TypeOrderTerminal))

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p2-b", ExecID: "P2E2",
		ClientOrderID: cid, LastQty: 60, LastPrice: 10.1,
	}))

	fills := make([]audit.Event, 0, 2)
	for _, e := range mem.Events() {
		if e.EventType == audit.TypeFillApplied {
			fills = append(fills, e)
			require.Equal(t, specHash, e.SpecHash)
			require.Equal(t, specHash, e.Payload["spec_hash"])
		}
	}
	require.Len(t, fills, 2)
	require.EqualValues(t, 40, fills[0].Payload["fill_qty"])
	require.EqualValues(t, 60, fills[1].Payload["fill_qty"])
	require.Equal(t, 1, countEventType(mem.Events(), audit.TypeOrderTerminal))
	pos, ok := acc.Position("1", "sz000402")
	require.True(t, ok)
	require.Equal(t, int64(100), pos.Volume)

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.InDelta(t, 10.0, got.Price, 1e-9)
}

func TestReportAudit_P3_DuplicateReportNoSecondFillApplied(t *testing.T) {
	real, consumer, mem, database, acc := newReportAuditFixture(t)
	cid, _ := submitForReportAudit(t, real, "sz000403", 10.0, 100)

	report := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p3-tr", ExecID: "P3E1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10.0,
	}
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), report))
	beforeFill := countEventType(mem.Events(), audit.TypeFillApplied)
	beforeReport := countEventType(mem.Events(), audit.TypeReportReceived)
	var beforeDB int64
	require.NoError(t, database.Model(&models.AuditEvent{}).
		Where("event_type = ?", audit.TypeFillApplied).Count(&beforeDB).Error)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), report))
	require.Equal(t, beforeFill, countEventType(mem.Events(), audit.TypeFillApplied),
		"duplicate report must not emit a second FILL_APPLIED")
	require.Equal(t, beforeReport, countEventType(mem.Events(), audit.TypeReportReceived),
		"duplicate report_id reuses same REPORT_RECEIVED event_id (idempotent)")
	pos, ok := acc.Position("1", "sz000403")
	require.True(t, ok)
	require.Equal(t, int64(100), pos.Volume)

	var afterDB int64
	require.NoError(t, database.Model(&models.AuditEvent{}).
		Where("event_type = ?", audit.TypeFillApplied).Count(&afterDB).Error)
	require.Equal(t, beforeDB, afterDB)

	// Same exec_id, different report_id → still no second FILL_APPLIED.
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p3-tr-2", ExecID: "P3E1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10.0,
	}))
	require.Equal(t, beforeFill, countEventType(mem.Events(), audit.TypeFillApplied))
	pos, ok = acc.Position("1", "sz000403")
	require.True(t, ok)
	require.Equal(t, int64(100), pos.Volume)
}

func TestReportAudit_P4_RejectAudit(t *testing.T) {
	real, consumer, mem, _, _ := newReportAuditFixture(t)
	cid, specHash := submitForReportAudit(t, real, "sz000404", 10.0, 50)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "p4-rej", ClientOrderID: cid,
		RejectCode: "R1", RejectReason: "broker reject",
	}))

	types := auditEventTypes(mem)
	require.Contains(t, types, audit.TypeReportReceived)
	require.Contains(t, types, audit.TypeOrderTerminal)
	require.Equal(t, 0, countEventType(mem.Events(), audit.TypeFillApplied))

	var reportEv, termEv audit.Event
	for _, e := range mem.Events() {
		switch e.EventType {
		case audit.TypeReportReceived:
			if e.ReportID == "p4-rej" {
				reportEv = e
			}
		case audit.TypeOrderTerminal:
			termEv = e
		}
	}
	requireReportPayload(t, reportEv, "p4-rej", ExecReportTypeREJECT, "1")
	require.Equal(t, "applied", reportEv.Payload["apply_result"])
	require.Equal(t, data.PaperOrderStatusRejected, termEv.Payload["terminal_status"])
	require.Equal(t, "1", termEv.Payload["order_id"])
	require.Equal(t, specHash, reportEv.SpecHash)
	require.Equal(t, specHash, termEv.SpecHash)

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusRejected, got.Status)
	require.InDelta(t, 10.0, got.Price, 1e-9)
}

func TestReportAudit_P5_CancelAudit(t *testing.T) {
	real, consumer, mem, _, _ := newReportAuditFixture(t)
	cid, specHash := submitForReportAudit(t, real, "sz000405", 10.0, 80)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p5-ack", ClientOrderID: cid,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "p5-cx", ClientOrderID: cid,
	}))

	require.Equal(t, 0, countEventType(mem.Events(), audit.TypeFillApplied))
	require.Equal(t, 1, countEventType(mem.Events(), audit.TypeOrderTerminal))

	var cancelEv, termEv audit.Event
	for _, e := range mem.Events() {
		if e.EventType == audit.TypeReportReceived && e.ReportID == "p5-cx" {
			cancelEv = e
		}
		if e.EventType == audit.TypeOrderTerminal {
			termEv = e
		}
	}
	requireReportPayload(t, cancelEv, "p5-cx", ExecReportTypeCANCEL, "1")
	require.Equal(t, data.PaperOrderStatusCancelled, termEv.Payload["terminal_status"])
	require.Equal(t, "1", termEv.Payload["order_id"])
	require.Equal(t, specHash, cancelEv.SpecHash)
	require.Equal(t, specHash, termEv.SpecHash)

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.InDelta(t, 10.0, got.Price, 1e-9)
}

func TestReportAudit_WriteFailureDoesNotAffectTradeState(t *testing.T) {
	real := NewRealBroker()
	real.SetAuditEmitter(audit.NewEmitter(failingAuditSink{}))
	acc := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, acc)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000406", Side: "buy", Price: 10, Volume: 100,
		SpecHash: audit.SpecFingerprint("sz000406", "buy", 10, 100),
	})
	require.NoError(t, err)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "fail-tr", ExecID: "FAIL1",
		ClientOrderID: order.ClientOrderID, LastQty: 100, LastPrice: 10,
	}))
	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, int64(100), got.FilledVolume)
	pos, ok := acc.Position("1", "sz000406")
	require.True(t, ok)
	require.Equal(t, int64(100), pos.Volume)
}
