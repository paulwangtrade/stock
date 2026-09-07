package execution

import (
	"context"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestPhase2C_PartialFill_FirstTradeKeepsPending(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.Equal(t, int64(100), order.LeavesQuantity)

	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "pf-1", ExecID: "E1",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 10,
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusPartiallyFilled, got.BrokerStatus)
	require.Equal(t, int64(30), got.FilledVolume)
	require.Equal(t, int64(70), got.LeavesQuantity)
	require.InDelta(t, 10.0, got.FilledPrice, 1e-9)

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestPhase2C_PartialFill_SecondTradeCompletes(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "pf-a", ExecID: "EA",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 10,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "pf-b", ExecID: "EB",
		ClientOrderID: order.ClientOrderID, LastQty: 70, LastPrice: 11,
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, data.BrokerStatusFilled, got.BrokerStatus)
	require.Equal(t, int64(100), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)
	// avg = (30*10 + 70*11) / 100 = 10.7
	require.InDelta(t, 10.7, got.FilledPrice, 1e-9)

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&n).Error)
	require.Equal(t, int64(2), n)
}

func TestPhase2C_PartialFill_AvgPriceFormula(t *testing.T) {
	filled, avg := applyCumulativeAvg(30, 10, 70, 11)
	require.Equal(t, int64(100), filled)
	require.InDelta(t, 10.7, avg, 1e-9)
	filled2, avg2 := applyCumulativeAvg(0, 0, 50, 12.5)
	require.Equal(t, int64(50), filled2)
	require.InDelta(t, 12.5, avg2, 1e-9)
}

func TestPhase2C_PartialFill_DuplicateExecIDNoDoubleCount(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000003", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	tr := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "dup-r1", ExecID: "EXEC-SAME",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10,
	}
	require.NoError(t, h.OnExecutionReport(context.Background(), tr))
	tr.ReportID = "dup-r2" // 不同 report，相同 exec
	tr.LastQty = 40
	tr.LastPrice = 99
	require.NoError(t, h.OnExecutionReport(context.Background(), tr))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, int64(40), got.FilledVolume)
	require.Equal(t, int64(60), got.LeavesQuantity)
	require.InDelta(t, 10.0, got.FilledPrice, 1e-9)

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Where("exec_id = ?", "EXEC-SAME").Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestPhase2C_PartialFill_OutOfOrderReportsStillAccumulate(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000004", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	// 乱序：先应用「看起来像第二笔」的回报（券商 CumQty 故意错误），本地仍按 LastQty 累加
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "oo-2", ExecID: "EO2",
		ClientOrderID: order.ClientOrderID, LastQty: 70, LastPrice: 11,
		CumQty: 100, AvgPrice: 999, // 忽略错误快照
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "oo-1", ExecID: "EO1",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 10,
		CumQty: 30, AvgPrice: 10,
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, int64(100), got.FilledVolume)
	require.Equal(t, int64(0), got.LeavesQuantity)
	// 应用顺序 70@11 再 30@10 → avg = (70*11+30*10)/100 = 10.7
	require.InDelta(t, 10.7, got.FilledPrice, 1e-9)
}

func TestPhase2C_PartialFill_PaperPathUnaffected(t *testing.T) {
	setupCutoverTestDB(t)

	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)
	ro, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000099", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "r-pf", ExecID: "ER",
		ClientOrderID: ro.ClientOrderID, LastQty: 50, LastPrice: 10,
	}))

	api := data.NewPaperTradingApi()
	_, err = api.ResetAccount(100_000)
	require.NoError(t, err)
	svc := NewExecutionService(NewPaperBroker(nil))
	po, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", Side: "buy", LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, AutoFill: true, StrategyTag: models.PaperStrategyTagTradePlan})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, po.Status)

	var paperFills, realFills int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&paperFills).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&realFills).Error)
	require.Equal(t, int64(1), paperFills)
	require.Equal(t, int64(1), realFills)
}
