package strategy

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupG13MetaTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:g13meta_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))
}

func seedReadyPoolForDraft(t *testing.T, tradeDate string) *models.CandidatePool {
	t.Helper()
	pool := &models.CandidatePool{
		TradeDate:   tradeDate,
		Status:      models.CandidatePoolStatusReady,
		GeneratedAt: time.Now(),
	}
	items := []models.CandidatePoolItem{
		{TradeDate: tradeDate, StockCode: "sz000001", StockName: "平安", Rank: 1, Score: 90, Reason: "r1"},
		{TradeDate: tradeDate, StockCode: "sz000002", StockName: "万科", Rank: 2, Score: 80, Reason: "r2"},
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	got, err := data.NewCandidatePoolRepo().GetByID(pool.ID)
	require.NoError(t, err)
	return got
}

func TestG13_DraftCreate_StampsLegacyProviderMetadata(t *testing.T) {
	setupG13MetaTestDB(t)
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled: false, MarketLevel: 3, AmountPerStock: amount, MaxNames: maxNames,
			Cash: 1_000_000, EquityBase: 1_000_000,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })

	pool := seedReadyPoolForDraft(t, "2026-08-22")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, models.TradePlanProviderModeOff, plan.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, plan.DecisionProvider)
	require.Equal(t, models.TradePlanDecisionVersionG21, plan.DecisionVersion)
	require.Equal(t, "", plan.AllocationVersion)

	reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeOff, reloaded.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, reloaded.DecisionProvider)
	require.Equal(t, models.TradePlanDecisionVersionG21, reloaded.DecisionVersion)
	require.Equal(t, "", reloaded.AllocationVersion)
}

func TestG13_CreatePlanWithItems_PersistsCallerMetadata(t *testing.T) {
	setupG13MetaTestDB(t)
	plan := &models.TradePlan{
		TradeDate:         "2026-08-22",
		GeneratedAt:       time.Now(),
		Status:            models.TradePlanStatusDraft,
		PlanVersion:       1,
		Side:              "buy",
		AmountPerStock:    100_000,
		ProviderMode:      models.TradePlanProviderModeOff,
		DecisionProvider:  models.TradePlanDecisionProviderFixed,
		DecisionVersion:   models.TradePlanDecisionVersionG21,
		AllocationVersion: "",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeOff, got.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, got.DecisionProvider)
	require.Equal(t, models.TradePlanDecisionVersionG21, got.DecisionVersion)
	require.Equal(t, "", got.AllocationVersion)
}

func TestG13_Freeze_DoesNotMutateProviderMetadata(t *testing.T) {
	setupG13MetaTestDB(t)
	plan := &models.TradePlan{
		TradeDate:         "2026-08-22",
		GeneratedAt:       time.Now(),
		Status:            models.TradePlanStatusDraft,
		PlanVersion:       1,
		Side:              "buy",
		AmountPerStock:    100_000,
		EnableExecute:     false,
		ProviderMode:      models.TradePlanProviderModeOff,
		DecisionProvider:  models.TradePlanDecisionProviderFixed,
		DecisionVersion:   models.TradePlanDecisionVersionG21,
		AllocationVersion: "",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	approved, err := ApproveTradePlan(got, "alice", "ok")
	require.NoError(t, err)

	frozen, err := FreezeTradePlan(approved, "bob", "freeze")
	require.NoError(t, err)
	require.True(t, frozen.IsFrozen())
	require.Equal(t, models.TradePlanProviderModeOff, frozen.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, frozen.DecisionProvider)
	require.Equal(t, models.TradePlanDecisionVersionG21, frozen.DecisionVersion)
	require.Equal(t, "", frozen.AllocationVersion)

	again, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeOff, again.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, again.DecisionProvider)
}

func TestG13_TryBeginExecute_DoesNotMutateProviderMetadata(t *testing.T) {
	setupG13MetaTestDB(t)
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:        "2026-08-22",
		GeneratedAt:      now,
		Status:           models.TradePlanStatusReady,
		PlanVersion:      1,
		Side:             "buy",
		AmountPerStock:   100_000,
		EnableExecute:    true,
		FreezeAt:         &now,
		ProviderMode:     models.TradePlanProviderModeOff,
		DecisionProvider: models.TradePlanDecisionProviderFixed,
		DecisionVersion:  models.TradePlanDecisionVersionG21,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	ok, err := data.NewTradePlanRepo().TryBeginExecute(plan.ID)
	require.NoError(t, err)
	require.True(t, ok)
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusExecuting, got.Status)
	require.Equal(t, models.TradePlanProviderModeOff, got.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, got.DecisionProvider)
	require.Equal(t, models.TradePlanDecisionVersionG21, got.DecisionVersion)
}

func TestG13_MaterializeItemUpdate_DoesNotMutateProviderMetadata(t *testing.T) {
	setupG13MetaTestDB(t)
	plan := &models.TradePlan{
		TradeDate:        "2026-08-22",
		GeneratedAt:      time.Now(),
		Status:           models.TradePlanStatusDraft,
		PlanVersion:      1,
		Side:             "buy",
		AmountPerStock:   100_000,
		ProviderMode:     models.TradePlanProviderModeOff,
		DecisionProvider: models.TradePlanDecisionProviderFixed,
		DecisionVersion:  models.TradePlanDecisionVersionG21,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	item := got.Items[0]
	item.LimitPrice = 10.5
	require.NoError(t, data.NewTradePlanRepo().UpdateItemMorningLimitPrice(&item))
	item.TargetVolume = 100
	require.NoError(t, data.NewTradePlanRepo().UpdateItemMorningTargetVolume(&item))

	again, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeOff, again.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, again.DecisionProvider)
	require.Equal(t, 10.5, again.Items[0].LimitPrice)
	require.Equal(t, int64(100), again.Items[0].TargetVolume)
}

func TestG13_Rescale_CopiesLegacyMetadata(t *testing.T) {
	setupG13MetaTestDB(t)
	plan := &models.TradePlan{
		TradeDate:         "2026-08-22",
		GeneratedAt:       time.Now(),
		Status:            models.TradePlanStatusDraft,
		PlanVersion:       1,
		Side:              "buy",
		AmountPerStock:    100_000,
		MaxNames:          2,
		ProviderMode:      models.TradePlanProviderModeOff,
		DecisionProvider:  models.TradePlanDecisionProviderFixed,
		DecisionVersion:   models.TradePlanDecisionVersionG21,
		AllocationVersion: "",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
		{TradeDate: "2026-08-22", StockCode: "sz000002", Priority: 2, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	res, err := RescaleTradePlanForCash(CashRescaleRequest{PlanID: plan.ID, AvailableCash: 120_000, MinPerStock: 1_000})
	require.NoError(t, err)
	require.True(t, res.Changed)
	require.NotNil(t, res.NewPlan)
	require.Equal(t, models.TradePlanProviderModeOff, res.NewPlan.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, res.NewPlan.DecisionProvider)
	require.Equal(t, models.TradePlanDecisionVersionG21, res.NewPlan.DecisionVersion)
	require.Equal(t, "", res.NewPlan.AllocationVersion)
}

func TestG13_Rescale_RejectsPortfolioProvider(t *testing.T) {
	setupG13MetaTestDB(t)
	plan := &models.TradePlan{
		TradeDate:         "2026-08-22",
		GeneratedAt:       time.Now(),
		Status:            models.TradePlanStatusDraft,
		PlanVersion:       1,
		Side:              "buy",
		AmountPerStock:    100_000,
		DecisionProvider:  models.TradePlanDecisionProviderPortfolio,
		AllocationVersion: models.TradePlanAllocationVersionF1Equal,
		ProviderMode:      models.TradePlanProviderModeControlled,
		DecisionVersion:   models.TradePlanDecisionVersionG21,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	_, err := RescaleTradePlanForCash(CashRescaleRequest{PlanID: plan.ID, AvailableCash: 50_000, MinPerStock: 1_000})
	require.Error(t, err)
	require.Contains(t, err.Error(), "portfolio_allocation")
}
