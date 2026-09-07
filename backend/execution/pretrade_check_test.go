package execution

import (
	"context"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestPreTradeCheck_CashInsufficient_DoesNotCallPort(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "should-not"}}
	svc := NewExecutionService(port).withSnapshotLoader(func(accountID uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000, Positions: map[string]preTradePosition{}}, nil
	})
	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh603799", Side: "buy", LimitPrice: 38.50, TargetVolume: 2500,
	}, ExecutePlanItemOpts{Price: 38.50, Volume: 2500, AutoFill: true})
	require.Error(t, err)
	require.Nil(t, order)
	require.Equal(t, data.PaperOrderRejectCashInsufficient, PreTradeRejectCode(err))
	require.Equal(t, 0, port.submitCount())
}

func TestPreTradeCheck_OK_CallsPortAndFilled(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	_, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	port := &countingPort{inner: NewPaperBroker(nil)}
	svc := NewExecutionService(port) // 默认 DB 快照
	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", StockName: "浦发", Side: "buy",
		LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{
		Price: 10, Volume: 100, AutoFill: true, StrategyTag: models.PaperStrategyTagTradePlan,
	})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, 1, port.submitCalls)
	require.Equal(t, data.PaperOrderStatusFilled, order.Status)
}

func TestPreTradeCheck_SellNoPosition_DoesNotCallPort(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "nope"}}
	svc := NewExecutionService(port).withSnapshotLoader(func(accountID uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 100_000, Positions: map[string]preTradePosition{}}, nil
	})
	order, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sz000001", Side: "sell", LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, AutoFill: true})
	require.Error(t, err)
	require.Nil(t, order)
	require.Equal(t, data.PaperOrderRejectPositionInsufficient, PreTradeRejectCode(err))
	require.Equal(t, 0, port.submitCount())
}

func TestPreTradeCheck_InvalidOrder(t *testing.T) {
	port := &stubPort{}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1e9}, nil
	})
	// Missing symbol → Safety Gate blocks before Port (Spec integrity).
	_, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{Side: "buy"}, ExecutePlanItemOpts{Price: 10, Volume: 100})
	require.Error(t, err)
	require.Contains(t, err.Error(), "safetygate")
	require.Equal(t, 0, port.calls)
}

func (s *stubPort) submitCount() int {
	return s.calls
}
