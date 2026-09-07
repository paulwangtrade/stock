package strategy_test

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRescaleTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:rescale_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureTradePlanTables())
}

func TestAllocateCash_Sufficient_NoChange(t *testing.T) {
	pending := []models.TradePlanItem{
		{StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Score: 90},
		{StockCode: "sz000002", Priority: 2, TargetAmount: 100_000, Score: 80},
	}
	amt, idx, mode, err := strategy.AllocateCashToPlanItems(pending, 250_000, 1_000)
	require.NoError(t, err)
	require.Equal(t, "none", mode)
	require.Len(t, idx, 2)
	require.Equal(t, 100_000.0, amt)
}

func TestAllocateCash_ScaleDownKeepAll(t *testing.T) {
	pending := []models.TradePlanItem{
		{StockCode: "a", Priority: 1, TargetAmount: 100_000},
		{StockCode: "b", Priority: 2, TargetAmount: 100_000},
		{StockCode: "c", Priority: 3, TargetAmount: 100_000},
	}
	amt, idx, mode, err := strategy.AllocateCashToPlanItems(pending, 150_000, 1_000)
	require.NoError(t, err)
	require.Equal(t, "scale_down", mode)
	require.Len(t, idx, 3)
	require.InDelta(t, 50_000, amt, 0.01)
	require.LessOrEqual(t, amt*3, 150_000+1e-6)
}

func TestAllocateCash_TrimNames(t *testing.T) {
	pending := []models.TradePlanItem{
		{StockCode: "a", Priority: 1, TargetAmount: 100_000},
		{StockCode: "b", Priority: 2, TargetAmount: 100_000},
		{StockCode: "c", Priority: 3, TargetAmount: 100_000},
		{StockCode: "d", Priority: 4, TargetAmount: 100_000},
		{StockCode: "e", Priority: 5, TargetAmount: 100_000},
	}
	amt, idx, mode, err := strategy.AllocateCashToPlanItems(pending, 2_500, 1_000)
	require.NoError(t, err)
	require.Equal(t, "trim_names", mode)
	require.Len(t, idx, 2)
	require.InDelta(t, 1_250, amt, 0.01)
	require.Equal(t, 0, idx[0])
	require.Equal(t, 1, idx[1])
}

func TestAllocateCash_TooLow_Fails(t *testing.T) {
	pending := []models.TradePlanItem{
		{StockCode: "a", Priority: 1, TargetAmount: 100_000},
	}
	_, _, _, err := strategy.AllocateCashToPlanItems(pending, 500, 1_000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot create executable plan")
}

func TestRescaleTradePlanForCash_CreatesNewVersionKeepsOriginal(t *testing.T) {
	setupRescaleTestDB(t)
	repo := data.NewTradePlanRepo()
	now := time.Now()
	src := &models.TradePlan{
		TradeDate: "2026-08-18", GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		AmountPerStock: 100_000, MaxNames: 3, EnableExecute: false,
		Side: "buy", SourceSession: models.TradePlanSourceAfterClose,
		Message: "original oversize plan",
	}
	items := []models.TradePlanItem{
		{StockCode: "sz000001", StockName: "A", Priority: 1, TargetAmount: 100_000, Score: 90, Status: models.TradePlanItemPending, Side: "buy"},
		{StockCode: "sz000002", StockName: "B", Priority: 2, TargetAmount: 100_000, Score: 80, Status: models.TradePlanItemPending, Side: "buy"},
		{StockCode: "sz000003", StockName: "C", Priority: 3, TargetAmount: 100_000, Score: 70, Status: models.TradePlanItemPending, Side: "buy"},
	}
	require.NoError(t, repo.CreatePlanWithItems(src, items))

	res, err := strategy.RescaleTradePlanForCash(strategy.CashRescaleRequest{
		PlanID:        src.ID,
		AvailableCash: 150_000,
		Actor:         "test",
	})
	require.NoError(t, err)
	require.True(t, res.Changed)
	require.Equal(t, "scale_down", res.Mode)
	require.Equal(t, src.ID, res.SourcePlanID)
	require.Equal(t, 1, res.SourcePlanVersion)
	require.Equal(t, 2, res.NewPlanVersion)
	require.NotZero(t, res.NewPlanID)
	require.InDelta(t, 50_000, res.AmountPerStockNew, 0.01)
	require.Equal(t, 3, res.NamesAfter)
	require.LessOrEqual(t, res.RequiredAfter, 150_000+1e-6)

	// Original intact
	orig, err := repo.GetByID(src.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, orig.Status)
	require.Equal(t, 1, orig.PlanVersion)
	require.Equal(t, 100_000.0, orig.AmountPerStock)
	require.Len(t, orig.Items, 3)
	for _, it := range orig.Items {
		require.Equal(t, 100_000.0, it.TargetAmount)
	}

	// New plan
	neu, err := repo.GetByID(res.NewPlanID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, neu.Status)
	require.Equal(t, 2, neu.PlanVersion)
	require.False(t, neu.EnableExecute)
	require.Contains(t, neu.Message, "cash_rescale")
	require.Equal(t, models.TradePlanSourceCashRescale, neu.SourceSession)
	require.Equal(t, models.TradePlanSourceCashRescale, neu.SourceKind)
	require.Equal(t, src.ID, neu.ParentPlanID)
	require.Equal(t, "scale_down", neu.RescaleMode)
	require.InDelta(t, 150_000, neu.AvailableCashUsed, 0.01)
	require.InDelta(t, 300_000, neu.RequiredCashBefore, 0.01)
	require.InDelta(t, 150_000, neu.RequiredCashAfter, 0.01)
	require.InDelta(t, 0.5, neu.ScaleRatio, 1e-6)
	require.NotEqual(t, models.TradePlanSourceMorningRebuild, neu.SourceSession)
	require.Equal(t, 50_000.0, neu.AmountPerStock)
	pending := 0
	for _, it := range neu.Items {
		if it.Status == models.TradePlanItemPending {
			pending++
			require.Equal(t, 50_000.0, it.TargetAmount)
			require.Equal(t, int64(0), it.TargetVolume)
		}
	}
	require.Equal(t, 3, pending)
}

func TestRescaleTradePlanForCash_TrimWhenCashTight(t *testing.T) {
	setupRescaleTestDB(t)
	repo := data.NewTradePlanRepo()
	src := &models.TradePlan{
		TradeDate: "2026-08-19", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		AmountPerStock: 100_000, EnableExecute: false, Side: "buy",
	}
	require.NoError(t, repo.CreatePlanWithItems(src, []models.TradePlanItem{
		{StockCode: "sz1", Priority: 1, TargetAmount: 100_000, Score: 99, Status: models.TradePlanItemPending},
		{StockCode: "sz2", Priority: 2, TargetAmount: 100_000, Score: 88, Status: models.TradePlanItemPending},
		{StockCode: "sz3", Priority: 3, TargetAmount: 100_000, Score: 77, Status: models.TradePlanItemPending},
	}))

	res, err := strategy.RescaleTradePlanForCash(strategy.CashRescaleRequest{
		PlanID: src.ID, AvailableCash: 2_500, Actor: "test",
	})
	require.NoError(t, err)
	require.True(t, res.Changed)
	require.Equal(t, "trim_names", res.Mode)
	require.Equal(t, 2, res.NamesAfter)

	neu, err := repo.GetByID(res.NewPlanID)
	require.NoError(t, err)
	var pendingCodes []string
	var trimmed int
	for _, it := range neu.Items {
		if it.Status == models.TradePlanItemPending {
			pendingCodes = append(pendingCodes, it.StockCode)
		}
		if it.RiskCode == "CASH_RESCALE_TRIMMED" {
			trimmed++
		}
	}
	require.Equal(t, []string{"sz1", "sz2"}, pendingCodes)
	require.Equal(t, 1, trimmed)

	require.Equal(t, models.TradePlanSourceCashRescale, neu.SourceKind)
	require.Equal(t, "trim_names", neu.RescaleMode)
	require.Equal(t, src.ID, neu.ParentPlanID)
	var trimmedAmt float64
	for _, it := range neu.Items {
		if it.RiskCode == "CASH_RESCALE_TRIMMED" {
			require.Equal(t, 0.0, it.TargetAmount)
			require.InDelta(t, 100_000, it.OriginalTargetAmount, 0.01)
			trimmedAmt = it.OriginalTargetAmount
		}
	}
	require.InDelta(t, 100_000, trimmedAmt, 0.01)
}

func TestRescaleTradePlanForCash_CashOK_NoNewPlan(t *testing.T) {
	setupRescaleTestDB(t)
	repo := data.NewTradePlanRepo()
	src := &models.TradePlan{
		TradeDate: "2026-08-20", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		AmountPerStock: 50_000, EnableExecute: false, Side: "buy",
	}
	require.NoError(t, repo.CreatePlanWithItems(src, []models.TradePlanItem{
		{StockCode: "sz1", Priority: 1, TargetAmount: 50_000, Status: models.TradePlanItemPending},
	}))
	res, err := strategy.RescaleTradePlanForCash(strategy.CashRescaleRequest{
		PlanID: src.ID, AvailableCash: 80_000, Actor: "test",
	})
	require.NoError(t, err)
	require.False(t, res.Changed)
	require.Equal(t, "none", res.Mode)
	require.Zero(t, res.NewPlanID)
}
