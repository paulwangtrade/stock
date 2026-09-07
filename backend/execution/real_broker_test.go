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

func richCashLoader(cash float64) preTradeSnapshotLoader {
	return func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: cash, Positions: map[string]preTradePosition{}}, nil
	}
}

func TestRealBroker_ImplementsExecutionPortSwitch(t *testing.T) {
	real := NewRealBroker()
	svc := NewExecutionService(real).withSnapshotLoader(richCashLoader(1_000_000))

	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", StockName: "浦发", Side: "buy",
		LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, AutoFill: true, AccountID: 1})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.ExecBackendRealStub, order.ExecBackend)
	require.Equal(t, data.BrokerStatusAccepted, order.BrokerStatus)
	require.True(t, strings.HasPrefix(order.ClientOrderID, data.RealClientOrderIDPrefix))
}

func TestRealBroker_SubmitDoesNotFill(t *testing.T) {
	real := NewRealBroker()
	order, err := real.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sz000001", Side: "buy",
		Price: 10, Volume: 100, AutoFill: true, // 显式要求成交，骨架仍忽略
	})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, int64(0), order.FilledVolume)
	require.Equal(t, 0.0, order.FilledPrice)
	require.True(t, order.FilledAt.IsZero())

	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, int64(0), got.FilledVolume)
}

func TestRealBroker_SubmitDoesNotCreatePaperFillsOrBrokerOrderID(t *testing.T) {
	setupCutoverTestDB(t)
	var fillBefore int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&fillBefore).Error)

	real := NewRealBroker()
	svc := NewExecutionService(real).withSnapshotLoader(richCashLoader(1_000_000))
	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sz000002", Side: "buy", LimitPrice: 12, TargetVolume: 200,
	}, ExecutePlanItemOpts{Price: 12, Volume: 200, AutoFill: true})
	require.NoError(t, err)
	require.Empty(t, order.BrokerOrderID, "Real 骨架不得伪造 broker_order_id")
	require.Empty(t, order.ExternalOrderID)
	require.NotEqual(t, data.PaperBrokerStatusPaper, order.BrokerStatus)
	require.NotEqual(t, data.PaperExecBackendPaper, order.ExecBackend)

	var fillAfter, orderCount int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&fillAfter).Error)
	require.NoError(t, db.Dao.Model(&data.PaperOrder{}).Count(&orderCount).Error)
	require.Equal(t, fillBefore, fillAfter, "Real Submit 不得产生 paper_fills")
	require.Zero(t, orderCount, "Real 骨架不得写入 paper_orders")
}

func TestRealBroker_CancelPending(t *testing.T) {
	real := NewRealBroker()
	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sh600519", Side: "buy", Price: 1800, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, real.Cancel(context.Background(), order.ID))
	got, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	// 撤单请求非终态：OMS 保持 pending，仅通道细态转 cancel_pending。
	require.Equal(t, data.PaperOrderStatusPending, got.Status)
	require.Equal(t, data.BrokerStatusCancelPending, got.BrokerStatus)
	require.ErrorIs(t, real.Cancel(context.Background(), order.ID), data.ErrOrderNotCancellable)
}

func TestPaperPath_StillWorksAlongsideRealBroker(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	_, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	paper := NewPaperBroker(nil)
	svc := NewExecutionService(paper) // 默认 PreTrade 走 DB 快照
	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", StockName: "浦发", Side: "buy",
		LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{
		Price: 10, Volume: 100, AutoFill: true, StrategyTag: models.PaperStrategyTagTradePlan,
	})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusFilled, order.Status)
	require.Equal(t, data.PaperExecBackendPaper, order.ExecBackend)
	require.Equal(t, data.PaperBrokerStatusPaper, order.BrokerStatus)
	require.Empty(t, order.BrokerOrderID)
	require.True(t, strings.HasPrefix(order.ClientOrderID, data.PaperClientOrderIDPrefix))

	var fills int64
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&fills).Error)
	require.Equal(t, int64(1), fills)
}
