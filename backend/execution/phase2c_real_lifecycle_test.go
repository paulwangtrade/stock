package execution

import (
	"context"
	"strings"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestPhase2C_Lifecycle_SubmitAckGeneratesBrokerOrderID(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusAccepted, order.BrokerStatus)
	require.Empty(t, order.BrokerOrderID)

	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "lc-ack-1",
		ClientOrderID: order.ClientOrderID,
		// 不传 BrokerOrderID → RealStub 自动生成
	}))

	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got.BrokerOrderID, data.RealStubBrokerOrderIDPrefix))
	require.Equal(t, data.BrokerStatusWorking, got.BrokerStatus)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)

	byBroker, err := real.QueryOrder(context.Background(), got.BrokerOrderID)
	require.NoError(t, err)
	require.Equal(t, order.ClientOrderID, byBroker.ClientOrderID)
}

func TestPhase2C_Lifecycle_AckPartialFillThenFill(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "lc-ack-2", ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "lc-t1", ExecID: "LE1",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10,
	}))
	mid, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, mid.Status)
	require.Equal(t, data.BrokerStatusPartiallyFilled, mid.BrokerStatus)
	require.Equal(t, int64(40), mid.FilledVolume)

	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "lc-t2", ExecID: "LE2",
		ClientOrderID: order.ClientOrderID, LastQty: 60, LastPrice: 11,
	}))
	fin, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, fin.Status)
	require.Equal(t, data.BrokerStatusFilled, fin.BrokerStatus)
	require.Equal(t, int64(100), fin.FilledVolume)
	require.Equal(t, int64(0), fin.LeavesQuantity)
}

func TestPhase2C_Lifecycle_PartialFillThenCancel(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000003", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "lc-ack-3", ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "lc-t30", ExecID: "LC30",
		ClientOrderID: order.ClientOrderID, LastQty: 30, LastPrice: 10,
	}))

	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "lc-cx", ClientOrderID: order.ClientOrderID,
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, got.Status)
	require.Equal(t, data.BrokerStatusCancelled, got.BrokerStatus)
	require.Equal(t, int64(30), got.FilledVolume, "撤单保留已成交")
	require.Equal(t, int64(0), got.LeavesQuantity)
}

func TestPhase2C_Lifecycle_Reject(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000004", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeREJECT, ReportID: "lc-rj",
		ClientOrderID: order.ClientOrderID, RejectCode: data.PaperOrderRejectInvalidOrder,
		RejectReason: "stub reject",
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusRejected, got.Status)
	require.Equal(t, data.BrokerStatusRejected, got.BrokerStatus)
	require.Equal(t, int64(0), got.FilledVolume)
}

func TestPhase2C_Lifecycle_RestartHydratesAll(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real1)

	order, err := real1.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000005", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "hy-ack", ClientOrderID: order.ClientOrderID,
	}))
	ack, err := real1.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.NotEmpty(t, ack.BrokerOrderID)

	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "hy-t", ExecID: "HY1",
		ClientOrderID: order.ClientOrderID, LastQty: 25, LastPrice: 10.5,
	}))

	real2, err := NewRealBrokerPersistent()
	require.NoError(t, err)

	byClient, err := real2.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, ack.BrokerOrderID, byClient.BrokerOrderID)
	require.Equal(t, int64(25), byClient.FilledVolume)
	require.Equal(t, data.BrokerStatusPartiallyFilled, byClient.BrokerStatus)

	byBroker, err := real2.QueryOrder(context.Background(), ack.BrokerOrderID)
	require.NoError(t, err)
	require.Equal(t, order.ClientOrderID, byBroker.ClientOrderID)

	var fills int64
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Where("exec_id = ?", "HY1").Count(&fills).Error)
	require.Equal(t, int64(1), fills)
}

func TestPhase2C_Lifecycle_PaperUnaffected(t *testing.T) {
	setupCutoverTestDB(t)
	real, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	h := NewFakeExecutionReportHandler(real)
	ro, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000088", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p-ack", ClientOrderID: ro.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeTRADE, ReportID: "p-t", ExecID: "PE1",
		ClientOrderID: ro.ClientOrderID, LastQty: 100, LastPrice: 10,
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
	require.Equal(t, data.PaperExecBackendPaper, po.ExecBackend)
	require.Empty(t, po.BrokerOrderID)

	var paperOrders, paperFills, stubOrders, stubFills int64
	require.NoError(t, db.Dao.Model(&data.PaperOrder{}).Count(&paperOrders).Error)
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&paperFills).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubOrder{}).Count(&stubOrders).Error)
	require.NoError(t, db.Dao.Model(&data.RealStubFill{}).Count(&stubFills).Error)
	require.Equal(t, int64(1), paperOrders)
	require.Equal(t, int64(1), paperFills)
	require.Equal(t, int64(1), stubOrders)
	require.Equal(t, int64(1), stubFills)
}
