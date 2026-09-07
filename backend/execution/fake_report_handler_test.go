package execution

import (
	"context"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestFakeReport_AckDoesNotFill(t *testing.T) {
	real := NewRealBroker()
	handler := NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100, AutoFill: true,
	})
	require.NoError(t, err)
	require.Empty(t, order.BrokerOrderID)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)

	require.NoError(t, handler.OnBrokerAck(context.Background(), BrokerAckReport{
		ClientOrderID:   order.ClientOrderID,
		BrokerOrderID:   "BRK-ACK-1",
		ExternalOrderID: "EXT-1",
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status, "ACK 不得推进 OMS 成交")
	require.Equal(t, "BRK-ACK-1", got.BrokerOrderID)
	require.Equal(t, "EXT-1", got.ExternalOrderID)
	require.Equal(t, data.BrokerStatusWorking, got.BrokerStatus)
	require.Equal(t, int64(0), got.FilledVolume)
	require.True(t, got.FilledAt.IsZero())
}

func TestFakeReport_FilledReportAdvancesToFilled(t *testing.T) {
	setupCutoverTestDB(t)
	var fillsBefore int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&fillsBefore).Error)

	real := NewRealBroker()
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sh600000", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, handler.OnBrokerAck(context.Background(), BrokerAckReport{
		ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-F1",
	}))
	require.NoError(t, handler.OnFilledReport(context.Background(), FilledReport{
		ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-F1",
		FillPrice:     10.05,
		FillQty:       100,
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, got.Status)
	require.Equal(t, data.BrokerStatusFilled, got.BrokerStatus)
	require.Equal(t, int64(100), got.FilledVolume)
	require.InDelta(t, 10.05, got.FilledPrice, 1e-9)
	require.False(t, got.FilledAt.IsZero())
	require.Equal(t, "BRK-F1", got.BrokerOrderID)

	var fillsAfter int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&fillsAfter).Error)
	require.Equal(t, fillsBefore, fillsAfter, "FilledReport 不得产生 paper_fills / 不得调 FillPaperOrder")
}

func TestFakeReport_RejectReportAdvancesToRejected(t *testing.T) {
	real := NewRealBroker()
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 11, Volume: 200,
	})
	require.NoError(t, err)

	require.NoError(t, handler.OnRejectReport(context.Background(), RejectReport{
		ClientOrderID: order.ClientOrderID,
		BrokerOrderID: "BRK-RJ-1",
		RejectCode:    data.PaperOrderRejectInvalidOrder,
		RejectReason:  "broker reject stub",
	}))

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusRejected, got.Status)
	require.Equal(t, data.BrokerStatusRejected, got.BrokerStatus)
	require.Equal(t, data.PaperOrderRejectInvalidOrder, got.RejectCode)
	require.Equal(t, "broker reject stub", got.RejectReason)
	require.Equal(t, "BRK-RJ-1", got.BrokerOrderID)
	require.Equal(t, int64(0), got.FilledVolume)
}

func TestFakeReport_LookupByClientOrderID(t *testing.T) {
	real := NewRealBroker()
	handler := NewFakeExecutionReportHandler(real)
	_, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000003", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)

	err = handler.OnBrokerAck(context.Background(), BrokerAckReport{
		ClientOrderID: "real_nonexistent",
		BrokerOrderID: "X",
	})
	require.ErrorIs(t, err, ErrReportOrderNotFound)
}

func TestFakeReport_PaperPathUnaffected(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	_, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	// Real 回报链路
	real := NewRealBroker()
	handler := NewFakeExecutionReportHandler(real)
	ro, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000099", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, handler.OnFilledReport(context.Background(), FilledReport{
		ClientOrderID: ro.ClientOrderID,
		BrokerOrderID: "BRK-P",
		FillPrice:     10,
		FillQty:       100,
	}))

	// Paper 路径仍走 PaperBroker + Fill
	paperSvc := NewExecutionService(NewPaperBroker(nil))
	po, err := paperSvc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", StockName: "浦发", Side: "buy",
		LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{
		Price: 10, Volume: 100, AutoFill: true, StrategyTag: models.PaperStrategyTagTradePlan,
	})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, po.Status)
	require.Equal(t, data.PaperExecBackendPaper, po.ExecBackend)
	require.Equal(t, data.PaperBrokerStatusPaper, po.BrokerStatus)
	require.Empty(t, po.BrokerOrderID)

	var fills int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&fills).Error)
	require.Equal(t, int64(1), fills, "仅 Paper 路径应产生一条 paper_fill")
}

func TestFakeReport_PartialFillAllowedInPR4(t *testing.T) {
	real := NewRealBroker()
	handler := NewFakeExecutionReportHandler(real)
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000004", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	err = handler.OnFilledReport(context.Background(), FilledReport{
		ClientOrderID: order.ClientOrderID,
		FillPrice:     10,
		FillQty:       50,
		ExecID:        "partial-1",
	})
	require.NoError(t, err)
	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusPartiallyFilled, got.BrokerStatus)
	require.Equal(t, int64(50), got.FilledVolume)
	require.Equal(t, int64(50), got.LeavesQuantity)
}
