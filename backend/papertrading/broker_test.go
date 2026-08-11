package papertrading_test

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/stockname"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
	require.NoError(t, data.EnsureTradePlanTables())

	// Inject config via cache (no disk / no chdir → avoids logger file-lock on cleanup).
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: false})
	t.Cleanup(papertrading.ResetConfigCache)
}

func enablePaperTrading(t *testing.T) {
	t.Helper()
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
}

// sessionANow is a fixed mid-morning clock for tests that must pass Session Policy.
func sessionANow() time.Time {
	return time.Date(2026, 8, 5, 10, 0, 0, 0, time.Local)
}

func seedFrozenPlan(t *testing.T, tradeDate string, items []models.TradePlanItem) *models.TradePlan {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		GeneratedAt:    now,
		Status:         models.TradePlanStatusReady,
		FreezeAt:       &now,
		FreezeBy:       "test",
		PlanVersion:    1,
		Side:           "buy",
		AmountPerStock: 100_000,
		MaxNames:       5,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.True(t, reloaded.IsFrozen(), "seed plan must be frozen")
	return reloaded
}

func buyItem(code, name string, vol int64) models.TradePlanItem {
	return models.TradePlanItem{
		StockCode: code, StockName: name, Side: "buy",
		Status: models.TradePlanItemPending, TargetVolume: vol, TargetAmount: 100_000, LimitPrice: 10,
	}
}

// 1. Feature flag OFF: no-op, no orders, no schema side effects.
func TestPaperBroker_FeatureFlagOff_NoOp(t *testing.T) {
	setupTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: false})

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})

	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.False(t, res.Enabled)
	require.Equal(t, 0, res.OrdersTotal)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.Equal(t, 0, status.OrdersTotal)
	// paper_sim tables must not have been created by a disabled run.
	require.False(t, db.Dao.Migrator().HasTable(&papertrading.PaperSimOrder{}))
}

// 2 & 3. Frozen plan → order; normal open price → fill.
func TestPaperBroker_FrozenPlan_FillsAtOpenPrice(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05, LimitUp: 11.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.True(t, res.Enabled)
	require.Equal(t, 1, res.OrdersTotal)
	require.Equal(t, 1, res.FilledCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, status.FilledCount)
	require.Len(t, status.Fills, 1)
	require.Equal(t, 10.05, status.Fills[0].Price)
	require.Equal(t, int64(1000), status.Fills[0].Volume)
	require.Equal(t, papertrading.FillReasonMarketOpen, status.Fills[0].FillReason)
	require.Equal(t, plan.ID, status.Orders[0].PlanID)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.InDelta(t, 1_000_000-10.05*1000, acc.Cash, 1e-6)
	require.InDelta(t, acc.Cash+10.05*1000, acc.Equity, 1e-6)

	positions, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, "sz000001", positions[0].StockCode)
	require.Equal(t, "平安银行", positions[0].StockName)
}

// 4. Limit up → rejected buy.
func TestPaperBroker_LimitUp_Rejected(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000002", "万科A", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000002": {Open: 11.0, LimitUp: 11.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.OrderStatusRejected, status.Orders[0].Status)
	require.Equal(t, papertrading.RejectLimitUpUnavailable, status.Orders[0].RejectReason)
}

// 5. Missing open price → rejected.
func TestPaperBroker_MissingPrice_Rejected(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000003", "国农科技", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.MissingPriceProvider{})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.RejectMissingOpenPrice, status.Orders[0].RejectReason)
}

// Invalid quantity → rejected.
func TestPaperBroker_InvalidQuantity_Rejected(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	item := models.TradePlanItem{
		StockCode: "sz000004", StockName: "国华网安", Side: "buy",
		Status: models.TradePlanItemPending, TargetVolume: 0, TargetAmount: 0, LimitPrice: 10,
	}
	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{item})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000004": {Open: 10.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
}

// 6. T+1: bought today is locked, settles to available next trading day.
func TestPaperBroker_T1_LockThenSettle(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0, LimitUp: 11.0}},
	})
	_, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	positions, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, int64(1000), positions[0].TotalVolume)
	require.Equal(t, int64(0), positions[0].AvailableVolume)
	require.Equal(t, int64(1000), positions[0].LockedVolume)

	require.NoError(t, papertrading.SettleNewTradingDay(acc.ID))
	positions, err = papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), positions[0].AvailableVolume)
	require.Equal(t, int64(0), positions[0].LockedVolume)
}

// Guard: non-frozen plan must not produce orders.
func TestPaperBroker_NotFrozen_Error(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(), Status: models.TradePlanStatusDraft,
		PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)}))

	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}},
	})
	_, err := broker.RunForPlan(plan.ID)
	require.Error(t, err)
}

func TestPaperBroker_EmptyStockName_ResolverSuccess(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	t.Cleanup(func() { stockname.SetResolveForTest(nil) })
	stockname.SetResolveForTest(func(symbol string, hint stockname.Hint) stockname.Result {
		require.Equal(t, "sz000001", symbol)
		return stockname.Result{Name: "解析平安", Source: stockname.SourceFollowed}
	})

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05, LimitUp: 11.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.FilledCount)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	positions, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, "sz000001", positions[0].StockCode)
	require.Equal(t, "解析平安", positions[0].StockName)
}

func TestPaperBroker_EmptyStockName_ResolverFail_SentinelStillFills(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	t.Cleanup(func() { stockname.SetResolveForTest(nil) })
	stockname.SetResolveForTest(func(symbol string, hint stockname.Hint) stockname.Result {
		return stockname.Result{Name: "", Source: ""}
	})

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "  ", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05, LimitUp: 11.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.FilledCount)
	require.Equal(t, 0, res.RejectCount)

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	positions, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	require.Equal(t, stockname.UnknownName, positions[0].StockName)
	require.NotEmpty(t, positions[0].StockName)
}
