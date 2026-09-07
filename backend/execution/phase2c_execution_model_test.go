package execution

import (
	"context"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// Phase2-C PR3-C：Real 成交模型冻结防回退。

func TestPhase2C_ExecModel_SubmitNoRealFill(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	_, err = real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100, AutoFill: true,
	})
	require.NoError(t, err)

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&n).Error)
	require.Zero(t, n, "Case1: Submit 后不得有 RealStubFill")
}

func TestPhase2C_ExecModel_AckNoFill(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, handler.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "ack-1",
		ClientOrderID: order.ClientOrderID, BrokerOrderID: "BRK-1",
	}))

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&n).Error)
	require.Zero(t, n, "Case2: ACK 不生成 Fill")
	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
}

func TestPhase2C_ExecModel_SingleTradeWritesRealStubFill(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sh600000", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	require.NoError(t, handler.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "tr-1", ExecID: "EXEC-1",
		ClientOrderID: order.ClientOrderID, BrokerOrderID: "BRK-T",
		LastQty: 100, LastPrice: 10.05, CumQty: 100, AvgPrice: 10.05,
	}))

	var fills []data.RealStubFill
	require.NoError(t, db.Dao.Find(&fills).Error)
	require.Len(t, fills, 1)
	require.Equal(t, "EXEC-1", fills[0].ExecID)
	require.Equal(t, order.ClientOrderID, fills[0].ClientOrderID)
	require.Equal(t, int64(100), fills[0].FillQty)
	require.InDelta(t, 10.05, fills[0].FillPrice, 1e-9)
	require.Equal(t, int64(100), fills[0].CumQty)
	require.InDelta(t, 10.05, fills[0].AvgPrice, 1e-9)

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, int64(100), got.FilledVolume)   // cum
	require.InDelta(t, 10.05, got.FilledPrice, 1e-9) // avg
}

func TestPhase2C_ExecModel_DuplicateExecIDNoDuplicateFill(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 11, Volume: 200,
	})
	require.NoError(t, err)

	tr := ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "tr-dup-1", ExecID: "EXEC-DUP",
		ClientOrderID: order.ClientOrderID, LastQty: 200, LastPrice: 11, CumQty: 200, AvgPrice: 11,
	}
	require.NoError(t, handler.OnExecutionReport(context.Background(), tr))
	tr.ReportID = "tr-dup-2" // 不同 report_id，相同 exec_id
	tr.LastPrice = 99
	require.NoError(t, handler.OnExecutionReport(context.Background(), tr))

	var n int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Where("exec_id = ?", "EXEC-DUP").Count(&n).Error)
	require.Equal(t, int64(1), n, "Case4: 重复 ExecID 不重复写 Fill")
}

func TestPhase2C_ExecModel_PaperStillOnlyPaperFill(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	_, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	svc := NewExecutionService(NewPaperBroker(nil))
	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", Side: "buy", LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, AutoFill: true, StrategyTag: models.PaperStrategyTagTradePlan})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, order.Status)

	var paperFills, realFills int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&paperFills).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&realFills).Error)
	require.Equal(t, int64(1), paperFills)
	require.Zero(t, realFills, "Case5: Paper 只产生 paper_fill")
}

func TestPhase2C_ExecModel_RealDoesNotPollutePaper(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000099", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, handler.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "tr-iso", ExecID: "EXEC-ISO",
		ClientOrderID: order.ClientOrderID, LastQty: 100, LastPrice: 10, CumQty: 100, AvgPrice: 10,
	}))

	var paperOrders, paperFills, stubOrders, stubFills int64
	require.NoError(t, db.Dao.Model(&data.PaperOrder{}).Count(&paperOrders).Error)
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&paperFills).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubOrder{}).Count(&stubOrders).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&stubFills).Error)
	require.Zero(t, paperOrders)
	require.Zero(t, paperFills)
	require.Equal(t, int64(1), stubOrders)
	require.Equal(t, int64(1), stubFills)
}
