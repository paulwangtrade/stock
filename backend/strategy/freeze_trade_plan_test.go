package strategy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func setupFreezeTestDB(t *testing.T) {
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

func seedApprovedDraft(t *testing.T, tradeDate string) *models.TradePlan {
	t.Helper()
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled:        false,
			MarketLevel:    3,
			AmountPerStock: amount,
			MaxNames:       maxNames,
			Cash:           1_000_000,
			EquityBase:     1_000_000,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })

	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		GeneratedAt:    time.Now(),
		Status:         models.TradePlanStatusDraft,
		PlanVersion:    1,
		EnableExecute:  false,
		AmountPerStock: 100_000,
		MaxNames:       5,
		SourceSession:  models.TradePlanSourceAfterClose,
		Side:           "buy",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{
			TradeDate:    tradeDate,
			StockCode:    "sz000001",
			StockName:    "平安",
			Priority:     1,
			TargetAmount: 100_000,
			Status:       models.TradePlanItemPending,
		},
	}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	approved, err := ApproveTradePlan(got, "alice", "ok to freeze")
	require.NoError(t, err)
	require.True(t, approved.IsDraft())
	require.NotNil(t, approved.ApprovedAt)
	return approved
}

func TestFreezeTradePlan_ApprovedDraftSucceeds(t *testing.T) {
	setupFreezeTestDB(t)
	plan := seedApprovedDraft(t, "2026-07-28")

	got, err := FreezeTradePlan(plan, "bob", "cutoff freeze")
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, got.Status)
	require.NotNil(t, got.FreezeAt)
	require.Equal(t, "bob", got.FreezeBy)
	require.Equal(t, "cutoff freeze", got.FreezeReason)
	require.True(t, got.IsFrozen())
	require.True(t, got.IsExecutableStatus())
	require.True(t, got.EnableExecute)
	require.NotNil(t, got.ApprovedAt)
	require.Equal(t, "alice", got.ApprovedBy)
}

func TestFreezeTradePlan_RejectsUnapprovedDraft(t *testing.T) {
	setupFreezeTestDB(t)
	plan := &models.TradePlan{
		TradeDate:      "2026-07-28",
		GeneratedAt:    time.Now(),
		Status:         models.TradePlanStatusDraft,
		PlanVersion:    1,
		EnableExecute:  false,
		AmountPerStock: 100_000,
		MaxNames:       5,
		Side:           "buy",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-07-28", StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)

	_, err = FreezeTradePlan(got, "bob", "nope")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ApprovedAt is nil")

	again, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, again.Status)
	require.Nil(t, again.FreezeAt)
	require.False(t, again.IsFrozen())
}

func TestFreezeTradePlan_RepeatFreezeIdempotent(t *testing.T) {
	setupFreezeTestDB(t)
	plan := seedApprovedDraft(t, "2026-07-28")

	first, err := FreezeTradePlan(plan, "bob", "first freeze")
	require.NoError(t, err)
	require.True(t, first.IsFrozen())
	firstAt := *first.FreezeAt

	second, err := FreezeTradePlan(first, "carol", "second freeze")
	require.NoError(t, err)
	require.True(t, second.IsFrozen())
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, models.TradePlanStatusReady, second.Status)
	require.Equal(t, "bob", second.FreezeBy)
	require.Equal(t, "first freeze", second.FreezeReason)
	require.True(t, second.FreezeAt.Equal(firstAt))
}

func TestFreezeTradePlan_IsFrozenAfterFreeze(t *testing.T) {
	setupFreezeTestDB(t)
	plan := seedApprovedDraft(t, "2026-07-29")
	require.False(t, plan.IsFrozen())

	got, err := FreezeTradePlan(plan, "bob", "assert frozen")
	require.NoError(t, err)
	require.True(t, got.IsFrozen())
	require.True(t, got.IsReady())
	require.False(t, got.IsDraft())
}

func TestFreezeTradePlanAt_UsesProvidedTime(t *testing.T) {
	setupFreezeTestDB(t)
	plan := seedApprovedDraft(t, "2026-08-17")
	want := time.Date(2026, 8, 17, 11, 12, 0, 0, time.Local)

	got, err := FreezeTradePlanAt(plan.ID, "sim", "late freeze", want)
	require.NoError(t, err)
	require.True(t, got.IsFrozen())
	require.NotNil(t, got.FreezeAt)
	require.True(t, got.FreezeAt.Equal(want))
}

func TestFreezeTradePlan_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("freeze_trade_plan.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"TryBeginExecute(",
		"RunDailyCandidateAndPlan(",
		"RunPaperOpenBuyOnce(",
		"TradingPreflight",
		"PreTradeCheck(",
		"TradePlanStatusFrozen",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("freeze service must not reference %s", token)
		}
	}
	require.Contains(t, src, "PromoteDraftToFrozen")
	require.Contains(t, src, "IsFrozen")
}
