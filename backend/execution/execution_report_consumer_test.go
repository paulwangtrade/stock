package execution

import (
	"context"
	"testing"

	"go-stock/backend/data"

	"github.com/stretchr/testify/require"
)

// consumerFixture 构建内存 RealBroker + Consumer + 持仓会计（无 DB / 无 migration）。
func consumerFixture(t *testing.T) (*RealBroker, *ExecutionReportConsumer, *InMemoryPositionAccountant) {
	t.Helper()
	real := NewRealBroker()
	acc := NewInMemoryPositionAccountant()
	consumer := NewExecutionReportConsumer(real, acc)
	require.NotNil(t, consumer)
	return real, consumer, acc
}

func consumerSubmit(t *testing.T, real *RealBroker, code string, vol int64) string {
	t.Helper()
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: code, StockName: code, Side: "buy", Price: 10, Volume: vol,
	})
	require.NoError(t, err)
	return order.ClientOrderID
}

func TestConsumer_ACK_KeepsPending(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000001", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "ack-1", ClientOrderID: cid,
	}))

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusWorking, got.BrokerStatus)
	require.NotEmpty(t, got.BrokerOrderID)
	require.True(t, data.IsPaperOMSStatus(got.Status))
}

func TestConsumer_PartialTrade(t *testing.T) {
	real, consumer, acc := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000002", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "pt-1", ExecID: "E1",
		ClientOrderID: cid, LastQty: 30, LastPrice: 10,
	}))

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusPartiallyFilled, got.BrokerStatus)
	require.Equal(t, int64(30), got.FilledVolume)
	require.Equal(t, int64(70), got.LeavesQuantity)
	require.Equal(t, int64(100), got.Volume) // Spec 投影不变
	require.InDelta(t, 10.0, got.Price, 1e-9)

	pos, ok := acc.Position("1", "sz000002")
	require.True(t, ok)
	require.Equal(t, int64(30), pos.Volume)
	require.InDelta(t, 10.0, pos.AvgCost, 1e-9)
}

func TestConsumer_FullTrade(t *testing.T) {
	real, consumer, acc := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000003", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ft-1", ExecID: "F1",
		ClientOrderID: cid, LastQty: 40, LastPrice: 10,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ft-2", ExecID: "F2",
		ClientOrderID: cid, LastQty: 60, LastPrice: 11,
	}))

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, data.BrokerStatusFilled, got.BrokerStatus)
	require.Equal(t, int64(100), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)
	require.False(t, got.FilledAt.IsZero())

	pos, ok := acc.Position("1", "sz000003")
	require.True(t, ok)
	require.Equal(t, int64(100), pos.Volume)
	// avg = (40*10 + 60*11) / 100 = 10.6
	require.InDelta(t, 10.6, pos.AvgCost, 1e-9)
}

func TestConsumer_DuplicateTrade_ByReportID(t *testing.T) {
	real, consumer, acc := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000004", 100)

	tr := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "dup-1", ExecID: "D1",
		ClientOrderID: cid, LastQty: 50, LastPrice: 10,
	}
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), tr))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), tr)) // 重复

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, int64(50), got.FilledVolume, "重复 report 不得二次累计")

	fills, err := real.ListFills(context.Background(), cid)
	require.NoError(t, err)
	require.Len(t, fills, 1)

	pos, _ := acc.Position("1", "sz000004")
	require.Equal(t, int64(50), pos.Volume)
}

func TestConsumer_DuplicateTrade_ByExecID(t *testing.T) {
	real, consumer, acc := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000005", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ex-1", ExecID: "X1",
		ClientOrderID: cid, LastQty: 50, LastPrice: 10,
	}))
	// 相同 exec_id、不同 report_id → 不二次入账
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ex-2", ExecID: "X1",
		ClientOrderID: cid, LastQty: 50, LastPrice: 10,
	}))

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, int64(50), got.FilledVolume)
	pos, _ := acc.Position("1", "sz000005")
	require.Equal(t, int64(50), pos.Volume)
}

func TestConsumer_Cancel(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000006", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "cx-1", ExecID: "C1",
		ClientOrderID: cid, LastQty: 30, LastPrice: 10,
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "cx-2", ClientOrderID: cid,
	}))

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(30), got.FilledVolume, "撤单保留已成交")
	require.Equal(t, int64(0), got.LeavesQuantity)
}

func TestConsumer_CancelledThenTrade_Rejected(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000007", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "ct-1", ClientOrderID: cid,
	}))
	err := consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "ct-2", ExecID: "CT2",
		ClientOrderID: cid, LastQty: 10, LastPrice: 10,
	})
	require.ErrorIs(t, err, ErrReportInvalidTransition)

	got, _ := real.QueryOrder(context.Background(), cid)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
}

func TestConsumer_FilledThenReject_Rejected(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000008", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "fr-1", ExecID: "FR1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 10,
	}))
	got, _ := real.QueryOrder(context.Background(), cid)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)

	err := consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "fr-2", ClientOrderID: cid,
		RejectCode: "LATE", RejectReason: "late reject after fill",
	})
	require.ErrorIs(t, err, ErrReportInvalidTransition)

	got2, _ := real.QueryOrder(context.Background(), cid)
	require.Equal(t, data.PaperOrderStatusFilled, got2.Status, "已成交不得被迟到 REJECT 抹掉")
}

func TestConsumer_Reject_Pending(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000009", 100)

	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "rj-1", ClientOrderID: cid,
		RejectCode: "RISK", RejectReason: "risk block",
	}))
	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusRejected, got.Status)
	require.Equal(t, data.BrokerStatusRejected, got.BrokerStatus)
	require.Equal(t, int64(0), got.FilledVolume)
}

func TestConsumer_FrozenSpecImmutable(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000010", StockName: "十号", Side: "buy",
		Price: 12.34, Volume: 200,
	})
	require.NoError(t, err)
	cid := order.ClientOrderID
	wantPrice, wantVol := order.Price, order.Volume

	// 一系列回报（部分成交 + 迟到 ACK）后，委托价量（Spec 投影）不得改变。
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "im-1", ExecID: "IM1",
		ClientOrderID: cid, LastQty: 100, LastPrice: 12.5, // 成交价高于限价，也不得回写委托
	}))
	require.NoError(t, consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "im-2", ClientOrderID: cid,
	}))

	got, err := real.QueryOrder(context.Background(), cid)
	require.NoError(t, err)
	require.InDelta(t, wantPrice, got.Price, 1e-9, "limit_price 投影不可变")
	require.Equal(t, wantVol, got.Volume, "target_volume 投影不可变")
	require.Equal(t, int64(100), got.FilledVolume)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
}

func TestConsumer_UnknownReportType(t *testing.T) {
	real, consumer, _ := consumerFixture(t)
	cid := consumerSubmit(t, real, "sz000011", 100)
	err := consumer.ConsumeExecutionReport(context.Background(), ExecutionReport{
		ReportType: "HEARTBEAT", ReportID: "u-1", ClientOrderID: cid,
	})
	require.ErrorIs(t, err, ErrReportInvalidPayload)
}
