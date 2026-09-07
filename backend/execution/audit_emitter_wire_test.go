package execution

import (
	"context"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/execution/audit"

	"github.com/stretchr/testify/require"
)

func TestAuditEmitter_LifecycleOrderAndIdempotency(t *testing.T) {
	mem := audit.NewMemorySink()
	em := audit.NewDefaultEmitter(mem)
	real := NewRealBroker()
	real.SetAuditEmitter(em)
	acc := NewInMemoryPositionAccountant()
	real.SetPositionAccountant(acc)
	consumer := NewExecutionReportConsumer(real, acc)

	specHash := audit.SpecFingerprint("sz000201", "buy", 10.0, 100)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000201", StockName: "审计", Side: "buy",
		Price: 10.0, Volume: 100, SpecHash: specHash,
	})
	require.NoError(t, err)
	cid := order.ClientOrderID

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "ae-ack", ClientOrderID: cid,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ae-tr", ExecID: "AE1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10.25, // fill ≠ limit
	}))

	events := mem.Events()
	require.GreaterOrEqual(t, len(events), 5)

	types := make([]string, 0, len(events))
	for _, e := range events {
		types = append(types, e.EventType)
		require.Equal(t, specHash, e.SpecHash, "spec_hash must come from Frozen Spec fingerprint")
		require.Equal(t, audit.SpecHashVersion, e.SpecHashVersion)
		require.Equal(t, audit.SchemaVersion, e.SchemaVersion)
	}
	require.Equal(t, []string{
		audit.TypeSubmitAttempt,
		audit.TypeSubmitAccepted,
		audit.TypeReportReceived, // ACK
		audit.TypeReportReceived, // TRADE
		audit.TypeFillApplied,
		audit.TypeOrderTerminal,
	}, types)

	require.Equal(t, audit.IDSubmitAttempt(cid), events[0].EventID)
	require.Equal(t, audit.IDSubmitResult(cid, "accepted"), events[1].EventID)
	require.Equal(t, audit.IDReportReceived("ae-ack"), events[2].EventID)
	require.Equal(t, audit.IDReportReceived("ae-tr"), events[3].EventID)
	require.Equal(t, audit.IDFillApplied("AE1"), events[4].EventID)

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.InDelta(t, 10.0, got.Price, 1e-9)
	require.Equal(t, int64(100), got.Volume)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)

	before := mem.Len()
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ae-tr", ExecID: "AE1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10.25,
	}))
	require.Equal(t, before, mem.Len(), "duplicate report_id must not append new audit events")
}

func TestAuditEmitter_DuplicateExecID_NoSecondFillEvent(t *testing.T) {
	mem := audit.NewMemorySink()
	em := audit.NewEmitter(mem)
	real := NewRealBroker()
	real.SetAuditEmitter(em)
	consumer := NewExecutionReportConsumer(real, nil)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000202", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r1", ExecID: "SAME",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r2", ExecID: "SAME",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10,
	}))

	fillEvents := 0
	reportEvents := 0
	for _, e := range mem.Events() {
		switch e.EventType {
		case audit.TypeFillApplied:
			fillEvents++
		case audit.TypeReportReceived:
			reportEvents++
		}
	}
	require.Equal(t, 1, fillEvents)
	require.Equal(t, 2, reportEvents, "each report_id gets REPORT_RECEIVED")
}

func TestAuditEmitter_SubmitRejected_EmitsTerminal(t *testing.T) {
	mem := audit.NewMemorySink()
	em := audit.NewEmitter(mem)
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: "rejected", Message: "no"},
	}
	real := NewRealBrokerWithAdapter(fake)
	real.SetAuditEmitter(em)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000203", Side: "buy", Price: 10, Volume: 100,
	})
	require.Error(t, err)
	require.NotNil(t, order)

	types := auditEventTypes(mem)
	require.Equal(t, []string{
		audit.TypeSubmitAttempt,
		audit.TypeSubmitRejected,
		audit.TypeOrderTerminal,
	}, types)
	require.Equal(t, "rejected", mem.Events()[1].Payload["submit_outcome"])
}

func TestAuditEmitter_TimeoutUnknown_UsesSubmitTimeoutType(t *testing.T) {
	mem := audit.NewMemorySink()
	em := audit.NewEmitter(mem)
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: "unknown", Message: "wire"},
	}
	real := NewRealBrokerWithAdapter(fake)
	real.SetAuditEmitter(em)

	_, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000204", Side: "buy", Price: 10, Volume: 50,
	})
	require.Error(t, err)
	types := auditEventTypes(mem)
	require.Equal(t, []string{audit.TypeSubmitAttempt, audit.TypeSubmitTimeout}, types)
	require.Equal(t, "unknown", mem.Events()[1].Payload["submit_outcome"])
	require.Equal(t, true, mem.Events()[1].Payload["reconciliation_required"])
}

func auditEventTypes(mem *audit.MemorySink) []string {
	evs := mem.Events()
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.EventType
	}
	return out
}
