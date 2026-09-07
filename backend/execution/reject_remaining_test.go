package execution

import (
	"context"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/execution/audit"

	"github.com/stretchr/testify/require"
)

func TestRejectRemaining_P1_PartialRejectCancelsRemainder(t *testing.T) {
	real := NewRealBroker()
	mem := audit.NewMemorySink()
	real.SetAuditEmitter(audit.NewEmitter(mem))
	accountant := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, accountant)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000901", Side: "buy", Price: 12.34, Volume: 100,
	})
	require.NoError(t, err)
	wantPrice, wantVolume := order.Price, order.Volume

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "rr-p1-trade", ExecID: "RR-P1-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 12.30,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "rr-p1-reject",
		ClientOrderID: order.ClientOrderID, RejectCode: "REMAINING_REJECTED",
		RejectReason: "broker rejected remaining quantity",
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(40), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)
	require.InDelta(t, 12.30, got.FilledPrice, 1e-9)
	require.Equal(t, wantPrice, got.Price, "Reject Remaining must not change Spec price")
	require.Equal(t, wantVolume, got.Volume, "Reject Remaining must not change target volume")

	fills, err := real.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, "RR-P1-E1", fills[0].ExecID)
	require.Equal(t, int64(40), fills[0].FillQty)

	position, ok := accountant.Position("1", "sz000901")
	require.True(t, ok)
	require.Equal(t, int64(40), position.Volume, "Reject Remaining must not roll back position")

	var terminal audit.Event
	for _, event := range mem.Events() {
		if event.EventType == audit.TypeOrderTerminal &&
			event.CausationEventID == "rr-p1-reject" {
			terminal = event
		}
	}
	require.Equal(t, audit.TypeOrderTerminal, terminal.EventType)
	require.Equal(t, data.PaperOrderStatusCancelled, terminal.Payload["terminal_status"])
	require.Equal(t, "reject_remaining", terminal.Payload["reason"])
	require.Equal(t, int64(40), terminal.Payload["filled_volume"])
}

func TestRejectRemaining_P2_PartialRejectHydrates(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	consumer := NewExecutionReportConsumer(real1, nil)

	order, err := real1.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000902", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "rr-p2-trade", ExecID: "RR-P2-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 10,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "rr-p2-reject",
		ClientOrderID: order.ClientOrderID, RejectCode: "REMAINING_REJECTED",
		RejectReason: "remaining rejected before restart",
	}))

	real2, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	got, err := real2.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(30), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)

	fills, err := real2.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, "RR-P2-E1", fills[0].ExecID)
}

func TestRejectRemaining_P3_ZeroFillRejectRemainsRejected(t *testing.T) {
	real := NewRealBroker()
	consumer := NewExecutionReportConsumer(real, nil)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000903", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "rr-p3-reject",
		ClientOrderID: order.ClientOrderID, RejectCode: "FULL_REJECT",
		RejectReason: "broker rejected full order",
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusRejected, got.Status)
	require.Equal(t, data.BrokerStatusRejected, got.BrokerStatus)
	require.Equal(t, int64(0), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)
	fills, err := real.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Empty(t, fills)
}

func TestRejectRemaining_N1_DuplicateRejectDoesNotReapply(t *testing.T) {
	real := NewRealBroker()
	accountant := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, accountant)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000904", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "rr-n1-trade", ExecID: "RR-N1-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 25, LastPrice: 10,
	}))
	reject := ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "rr-n1-reject",
		ClientOrderID: order.ClientOrderID, RejectCode: "REMAINING_REJECTED",
		RejectReason: "duplicate reject",
	}
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), reject))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), reject))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, int64(25), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)

	fills, err := real.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, "RR-N1-E1", fills[0].ExecID)

	position, ok := accountant.Position("1", "sz000904")
	require.True(t, ok)
	require.Equal(t, int64(25), position.Volume)
}
