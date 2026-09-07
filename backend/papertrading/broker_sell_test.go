package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func sellItem(code, name string, vol int64) models.TradePlanItem {
	return models.TradePlanItem{
		StockCode: code, StockName: name, Side: "sell",
		Status: models.TradePlanItemPending, TargetVolume: vol, LimitPrice: 0,
	}
}

func seedFrozenSellPlan(t *testing.T, tradeDate string, items []models.TradePlanItem) *models.TradePlan {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:   tradeDate,
		GeneratedAt: now,
		Status:      models.TradePlanStatusReady,
		ApprovedAt:  &now,
		ApprovedBy:  "test",
		FreezeAt:    &now,
		FreezeBy:    "test",
		PlanVersion: 1,
		Side:        "sell",
		MaxNames:    5,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.True(t, reloaded.IsFrozen())
	return reloaded
}

// seedETFPosition buys sh510300 via fillBuy (T0 → available immediately).
func seedETFPosition(t *testing.T, vol int64) {
	t.Helper()
	plan := seedFrozenPlan(t, "2026-08-14", []models.TradePlanItem{buyItem("sh510300", "沪深300ETF", vol)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.0, LimitUp: 0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.FilledCount)
}

func TestPaperBroker_Sell_FillsWhenAvailableSufficient(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	const vol = int64(1000)
	seedETFPosition(t, vol)

	accBefore, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	cashBefore := accBefore.Cash

	plan := seedFrozenSellPlan(t, "2026-08-15", []models.TradePlanItem{sellItem("sh510300", "沪深300ETF", 300)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.2, LimitUp: 0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.OrdersTotal)
	require.Equal(t, 1, res.FilledCount)
	require.Equal(t, 0, res.RejectCount)
	require.Equal(t, 0, res.SkippedItems)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Len(t, status.Fills, 1)
	require.Equal(t, "sell", status.Fills[0].Side)
	require.Equal(t, 4.2, status.Fills[0].Price)
	require.Equal(t, int64(300), status.Fills[0].Volume)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	require.InDelta(t, cashBefore+4.2*300, acc.Cash, 1e-6)

	positions, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, vol-300, positions[0].TotalVolume)
	require.Equal(t, vol-300, positions[0].AvailableVolume)
	require.InDelta(t, 4.0, positions[0].AvgCost, 1e-9)
}

func TestPaperBroker_Sell_RejectedWhenAvailableInsufficient(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	seedETFPosition(t, 1000)

	plan := seedFrozenSellPlan(t, "2026-08-15", []models.TradePlanItem{sellItem("sh510300", "沪深300ETF", 1500)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.2}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)
	require.Equal(t, 0, res.FilledCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.OrderStatusRejected, status.Orders[0].Status)
	require.Equal(t, papertrading.RejectInsufficientAvailable, status.Orders[0].RejectReason)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	positions, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), positions[0].TotalVolume)
	require.Equal(t, int64(1000), positions[0].AvailableVolume)
	_ = acc
}

func TestPaperBroker_Sell_RejectedNoPosition(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenSellPlan(t, "2026-08-15", []models.TradePlanItem{sellItem("sh510300", "沪深300ETF", 100)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.2}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.RejectNoPosition, status.Orders[0].RejectReason)
}

func TestPaperBroker_SellPlan_LifecycleDone(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	seedETFPosition(t, 2000)

	plan := seedFrozenSellPlan(t, "2026-08-15", []models.TradePlanItem{sellItem("sh510300", "沪深300ETF", 500)})
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerManual,
		Actor:            "test",
		Price:            papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.1}}},
		SkipWeekdayCheck: true,
		Now:              sessionANow(),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDone, got.Status)
}
