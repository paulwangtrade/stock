package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedAttrAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedAttrPlanItem(t *testing.T, tradeDate, code, name string, strategy string) (*models.TradePlan, *models.TradePlanItem) {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, Status: models.TradePlanStatusReady,
		PlanVersion: 1, ApprovedAt: &now, FreezeAt: &now,
	}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: tradeDate, StockCode: code, StockName: name,
		Side: "buy", Status: models.TradePlanItemPending, StrategyName: strategy,
		// order_id/fill_id intentionally left 0 — attribution must not need them
	}
	require.NoError(t, db.Dao.Create(item).Error)
	return plan, item
}

func seedAttrOrderFill(
	t *testing.T,
	accID uint,
	plan *models.TradePlan,
	item *models.TradePlanItem,
	price float64,
	vol int64,
) (*papertrading.PaperSimOrder, *papertrading.PaperSimFill) {
	t.Helper()
	now := time.Now()
	order := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: plan.TradeDate,
		StockCode: item.StockCode, StockName: item.StockName, Side: "buy",
		Quantity: vol, OrderPrice: price, Status: papertrading.OrderStatusFilled,
		FilledPrice: price, FilledVolume: vol, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: item.StockCode, StockName: item.StockName, Side: "buy",
		Price: price, Volume: vol, FillReason: papertrading.FillReasonMarketOpen, FilledAt: now,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	return order, fill
}

func TestAttribution_SinglePlanFull(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)
	plan, item := seedAttrPlanItem(t, "2026-08-05", "sh600363", "联创光电", "strat-a")
	_, fill := seedAttrOrderFill(t, acc.ID, plan, item, 10.5, 3000)

	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 3000, AvailableVolume: 0, LockedVolume: 3000,
		AvgCost: 10.5, MarkPrice: 11.0,
	}).Error)

	view, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{})
	require.NoError(t, err)
	require.True(t, view.Enabled)
	require.Len(t, view.Positions, 1)
	row := view.Positions[0]
	require.Equal(t, "sh600363", row.StockCode)
	require.Equal(t, int64(3000), row.TotalVolume)
	require.Equal(t, 11.0, row.CurrentPrice)
	require.InDelta(t, (11.0-10.5)*3000, row.PnL, 1e-6)
	require.Len(t, row.Lots, 1)
	require.Equal(t, plan.ID, row.Lots[0].PlanID)
	require.Equal(t, item.ID, row.Lots[0].PlanItemID)
	require.Equal(t, fill.ID, row.Lots[0].FillID)
	require.Equal(t, int64(3000), row.Lots[0].Volume)
	require.InDelta(t, 10.5*3000, row.Lots[0].CostAmount, 1e-6)
	require.Equal(t, papertrading.ReconcileStatusMatched, row.Reconcile.Status)
	require.Equal(t, int64(0), row.Reconcile.UnattributedVolume)
	require.Nil(t, row.Unattributed)
	require.Equal(t, []uint{plan.ID}, row.PlanIDs)
	require.Contains(t, row.SourceSummary, "plan #")
}

func TestAttribution_MultiPlanSameStock(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)

	plan32, item32 := seedAttrPlanItem(t, "2026-08-01", "sh600363", "联创光电", "p32")
	plan35, item35 := seedAttrPlanItem(t, "2026-08-03", "sh600363", "联创光电", "p35")
	_, f32 := seedAttrOrderFill(t, acc.ID, plan32, item32, 10.0, 3000)
	_, f35 := seedAttrOrderFill(t, acc.ID, plan35, item35, 12.0, 2000)

	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 5000, AvgCost: 10.8, MarkPrice: 11.5,
	}).Error)

	view, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{})
	require.NoError(t, err)
	require.Len(t, view.Positions, 1)
	row := view.Positions[0]
	require.Equal(t, int64(5000), row.TotalVolume)
	require.Len(t, row.Lots, 2, "both plans must appear as separate lots")
	require.ElementsMatch(t, []uint{plan32.ID, plan35.ID}, row.PlanIDs)
	require.Contains(t, row.SourceSummary, "plans")
	require.Contains(t, row.SourceSummary, "#")

	vols := map[uint]int64{}
	fillIDs := map[uint]bool{}
	for _, lot := range row.Lots {
		vols[lot.PlanID] = lot.Volume
		fillIDs[lot.FillID] = true
	}
	require.Equal(t, int64(3000), vols[plan32.ID])
	require.Equal(t, int64(2000), vols[plan35.ID])
	require.True(t, fillIDs[f32.ID])
	require.True(t, fillIDs[f35.ID])
	require.Equal(t, papertrading.ReconcileStatusMatched, row.Reconcile.Status)
}

func TestAttribution_PositionFillMismatch(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)
	plan, item := seedAttrPlanItem(t, "2026-08-05", "sz000001", "平安银行", "x")
	seedAttrOrderFill(t, acc.ID, plan, item, 10.0, 1000)

	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1500, AvgCost: 10, MarkPrice: 10,
	}).Error)

	view, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{})
	require.NoError(t, err)
	row := view.Positions[0]
	require.Equal(t, int64(1500), row.Reconcile.PositionVolume)
	require.Equal(t, int64(1000), row.Reconcile.AttributedVolume)
	require.Equal(t, int64(500), row.Reconcile.UnattributedVolume)
	require.Equal(t, papertrading.ReconcileStatusUnattributed, row.Reconcile.Status)
	require.NotNil(t, row.Unattributed)
	require.Equal(t, int64(500), row.Unattributed.Volume)
	require.Equal(t, "POSITION_GT_FILLS", row.Unattributed.ReasonCode)
	require.Len(t, row.Lots, 1)
	require.NotEqual(t, uint(0), row.Lots[0].FillID, "must not forge fill for unattributed")
	require.False(t, view.ReconcileAllMatched)
}

func TestAttribution_PositionWithoutFills(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600519", StockName: "贵州茅台",
		TotalVolume: 100, AvgCost: 1800, MarkPrice: 1800,
	}).Error)

	view, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{})
	require.NoError(t, err)
	require.Len(t, view.Positions, 1)
	row := view.Positions[0]
	require.Empty(t, row.Lots)
	require.Equal(t, int64(100), row.Reconcile.UnattributedVolume)
	require.Equal(t, papertrading.ReconcileStatusUnattributed, row.Reconcile.Status)
	require.NotNil(t, row.Unattributed)
	require.Equal(t, int64(100), row.Unattributed.Volume)
	require.Contains(t, row.SourceSummary, "unattributed")
}

func TestAttribution_DoesNotUseItemOrderFillBackfill(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)
	plan, item := seedAttrPlanItem(t, "2026-08-05", "sz000002", "万科A", "y")
	require.Equal(t, uint(0), item.OrderID)
	require.Equal(t, uint(0), item.FillID)
	_, fill := seedAttrOrderFill(t, acc.ID, plan, item, 8.0, 500)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000002", TotalVolume: 500, MarkPrice: 8,
	}).Error)

	view, err := papertrading.BuildPositionAttribution(papertrading.AttributionOptions{})
	require.NoError(t, err)
	require.Equal(t, fill.ID, view.Positions[0].Lots[0].FillID)
	require.Equal(t, papertrading.ReconcileStatusMatched, view.Positions[0].Reconcile.Status)
}
