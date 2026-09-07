package execution

import (
	"context"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/execution/audit"

	"github.com/stretchr/testify/require"
)

func countTerminalEvents(mem *audit.MemorySink) int {
	n := 0
	for _, ev := range mem.Events() {
		if ev.EventType == audit.TypeOrderTerminal {
			n++
		}
	}
	return n
}

// P1：Cancel Request 只是意图，不是事实终态。
func TestCancelPending_P1_CancelRequestKeepsOrderPending(t *testing.T) {
	real := NewRealBroker()
	mem := audit.NewMemorySink()
	real.SetAuditEmitter(audit.NewEmitter(mem))

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000911", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real.Cancel(context.Background(), order.ID))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusCancelPending, got.BrokerStatus)
	require.Equal(t, int64(0), got.FilledVolume)
	require.Equal(t, int64(100), got.LeavesQuantity, "撤单未确认前剩余量仍在场")
	require.Equal(t, 10.0, got.Price)
	require.Equal(t, int64(100), got.Volume)

	require.Zero(t, countTerminalEvents(mem), "Cancel Request 不得产生 ORDER_TERMINAL")
	require.ErrorIs(t, real.Cancel(context.Background(), order.ID), data.ErrOrderNotCancellable,
		"cancel_pending 期间重复撤单请求不再受理")
}

// P2：cancel_pending 期间的 TRADE 仍是事实成交，必须入账。
func TestCancelPending_P2_TradeAppliesWhileCancelPending(t *testing.T) {
	real := NewRealBroker()
	mem := audit.NewMemorySink()
	real.SetAuditEmitter(audit.NewEmitter(mem))
	accountant := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, accountant)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000912", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	wantPrice, wantVolume := order.Price, order.Volume
	require.NoError(t, real.Cancel(context.Background(), order.ID))

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "cp-p2-trade", ExecID: "CP-P2-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 9.9,
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusCancelPending, got.BrokerStatus, "成交不撤销撤单意图")
	require.Equal(t, int64(40), got.FilledVolume)
	require.Equal(t, int64(60), got.LeavesQuantity)
	require.InDelta(t, 9.9, got.FilledPrice, 1e-9)
	require.Equal(t, wantPrice, got.Price, "TRADE 不得改写 Spec 价格")
	require.Equal(t, wantVolume, got.Volume, "TRADE 不得改写目标数量")

	fills, err := real.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, int64(40), fills[0].FillQty)

	position, ok := accountant.Position("1", "sz000912")
	require.True(t, ok)
	require.Equal(t, int64(40), position.Volume)

	require.Zero(t, countTerminalEvents(mem), "部分成交 + cancel_pending 仍非终态")
}

// P3：CANCEL 回报才是撤单确认，落 OMS 终态并发 ORDER_TERMINAL。
func TestCancelPending_P3_CancelReportConfirmsCancelled(t *testing.T) {
	real := NewRealBroker()
	mem := audit.NewMemorySink()
	real.SetAuditEmitter(audit.NewEmitter(mem))
	consumer := NewExecutionReportConsumer(real, nil)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000913", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real.Cancel(context.Background(), order.ID))

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "cp-p3-cancel",
		ClientOrderID: order.ClientOrderID,
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(0), got.LeavesQuantity)

	var terminal audit.Event
	for _, ev := range mem.Events() {
		if ev.EventType == audit.TypeOrderTerminal {
			terminal = ev
		}
	}
	require.Equal(t, audit.TypeOrderTerminal, terminal.EventType)
	require.Equal(t, data.PaperOrderStatusCancelled, terminal.Payload["terminal_status"])
	require.Equal(t, "cancel_confirmed", terminal.Payload["reason"])
	require.Equal(t, 1, countTerminalEvents(mem))
}

// P4：cancel_pending → TRADE → CANCEL，成交事实必须保留。
func TestCancelPending_P4_TradeThenCancelKeepsFills(t *testing.T) {
	real := NewRealBroker()
	accountant := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, accountant)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000914", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real.Cancel(context.Background(), order.ID))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "cp-p4-trade", ExecID: "CP-P4-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 35, LastPrice: 10,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "cp-p4-cancel",
		ClientOrderID: order.ClientOrderID,
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(35), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)

	fills, err := real.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)

	position, ok := accountant.Position("1", "sz000914")
	require.True(t, ok)
	require.Equal(t, int64(35), position.Volume)
}

// P5：pending + cancel_pending + fills 是合法持久化组合，重启可 hydrate。
func TestCancelPending_P5_HydrateAcceptsCancelPendingWithFills(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	consumer := NewExecutionReportConsumer(real1, nil)

	order, err := real1.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000915", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real1.Cancel(context.Background(), order.ID))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "cp-p5-trade", ExecID: "CP-P5-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 20, LastPrice: 10,
	}))

	real2, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	got, err := real2.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusCancelPending, got.BrokerStatus)
	require.Equal(t, int64(20), got.FilledVolume)
	require.Equal(t, int64(80), got.LeavesQuantity)

	fills, err := real2.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Len(t, fills, 1)
	require.Equal(t, "CP-P5-E1", fills[0].ExecID)
}

// N1：撤单确认后是终态，迟到 TRADE 必须拒绝。
func TestCancelPending_N1_TradeAfterCancelledRejected(t *testing.T) {
	real := NewRealBroker()
	accountant := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, accountant)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000916", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real.Cancel(context.Background(), order.ID))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "cp-n1-cancel",
		ClientOrderID: order.ClientOrderID,
	}))

	err = consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "cp-n1-trade", ExecID: "CP-N1-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 10, LastPrice: 10,
	})
	require.ErrorIs(t, err, ErrReportInvalidTransition)

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, int64(0), got.FilledVolume)
	fills, err := real.ListFills(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Empty(t, fills)
	_, ok := accountant.Position("1", "sz000916")
	require.False(t, ok, "被拒 TRADE 不得入账")
}

// N2：重复 CANCEL 幂等（同 report_id 去重 + 已终态 noop）。
func TestCancelPending_N2_DuplicateCancelIsIdempotent(t *testing.T) {
	real := NewRealBroker()
	mem := audit.NewMemorySink()
	real.SetAuditEmitter(audit.NewEmitter(mem))
	consumer := NewExecutionReportConsumer(real, nil)

	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000917", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real.Cancel(context.Background(), order.ID))

	cancel := ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "cp-n2-cancel",
		ClientOrderID: order.ClientOrderID,
	}
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), cancel))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), cancel))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "cp-n2-cancel-2",
		ClientOrderID: order.ClientOrderID,
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(0), got.LeavesQuantity)
	require.Equal(t, 1, countTerminalEvents(mem), "重复 CANCEL 不得重复发终态事件")
}
