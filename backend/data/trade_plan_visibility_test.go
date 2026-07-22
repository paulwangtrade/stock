package data

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestGetUpcomingTradePlan_FrozenPreferredOverDraft(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	freezeAt := now.Add(-time.Hour)
	today := "2026-07-22"

	draft := &models.TradePlan{
		TradeDate: "2026-07-22", GeneratedAt: now, PoolID: 1,
		Status: models.TradePlanStatusDraft, PlanVersion: 9,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(draft, []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Priority: 1, TargetAmount: 10000, Status: models.TradePlanItemPending},
	}))

	frozen := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 2,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freezeAt, FreezeBy: "ops", FreezeReason: "eod freeze",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:   risk.PlanRiskStatusPassed,
		ApprovedAt:   &now, ApprovedBy: "approver",
	}
	require.NoError(t, repo.CreatePlanWithItems(frozen, []models.TradePlanItem{
		{StockCode: "sh600519", Side: "buy", Priority: 1, TargetAmount: 20000, Status: models.TradePlanItemPending},
	}))

	view, err := repo.GetUpcomingTradePlan(today)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, frozen.ID, view.ID)
	require.Equal(t, "2026-07-23", view.TradeDate)
	require.True(t, view.IsFrozen)
	require.Equal(t, models.TradePlanStatusReady, view.Status)
	require.Equal(t, "ops", view.FreezeBy)
	require.True(t, view.RiskPassed)
	require.Len(t, view.Items, 1)
	require.Equal(t, "sh600519", view.Items[0].StockCode)
}

func TestGetUpcomingTradePlan_DraftFallbackWhenNoFrozen(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	today := "2026-07-22"

	// Naked ready (no FreezeAt) must NOT be selected as Frozen.
	nakedReady := &models.TradePlan{
		TradeDate: "2026-07-22", GeneratedAt: now, PoolID: 3,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		SourceSession: models.TradePlanSourceMorningRebuild,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(nakedReady, nil))

	draft := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 4,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPartial,
		RiskSummary:   "1 accepted 1 rejected",
	}
	require.NoError(t, repo.CreatePlanWithItems(draft, []models.TradePlanItem{
		{
			StockCode: "sz001309", Side: "buy", Priority: 1, TargetAmount: 10000,
			Status: models.TradePlanItemSkipped, RiskCode: "LIMIT_UP", RiskMessage: "涨停",
		},
	}))

	view, err := repo.GetUpcomingTradePlan(today)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, draft.ID, view.ID)
	require.Equal(t, models.TradePlanStatusDraft, view.Status)
	require.False(t, view.IsFrozen)
	require.False(t, view.RiskPassed)
	require.Contains(t, view.RiskReasons, "LIMIT_UP: 涨停")
}

func TestGetUpcomingTradePlan_MultiVersionPicksHighestPlanVersion(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	today := "2026-07-22"
	tradeDate := "2026-07-23"

	v1 := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 10,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(v1, []models.TradePlanItem{
		{StockCode: "sz000001", Priority: 1, Status: models.TradePlanItemPending},
	}))
	v3 := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 11,
		Status: models.TradePlanStatusDraft, PlanVersion: 3,
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(v3, []models.TradePlanItem{
		{StockCode: "sz000002", Priority: 1, Status: models.TradePlanItemPending},
	}))
	v2 := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 12,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(v2, []models.TradePlanItem{
		{StockCode: "sz000003", Priority: 1, Status: models.TradePlanItemPending},
	}))

	view, err := repo.GetUpcomingTradePlan(today)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, v3.ID, view.ID)
	require.Equal(t, 3, view.PlanVersion)
	require.Equal(t, uint(11), view.PoolID)
}

func TestGetUpcomingTradePlan_FrozenMultiVersionOnEarliestDate(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	freezeAt := now
	today := "2026-07-22"

	// Earlier date lower version, later date higher version → prefer earliest trade_date frozen.
	earlierLow := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 20,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freezeAt, FreezeBy: "a",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(earlierLow, nil))

	// Same earliest date, higher version must win.
	earlierHigh := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 21,
		Status: models.TradePlanStatusReady, PlanVersion: 2,
		FreezeAt: &freezeAt, FreezeBy: "b",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	// CreatePlanWithItems supersedes prior ready → re-freeze earlierLow would become superseded.
	// Insert second frozen via direct create to simulate multi-version frozen history without CAS.
	require.NoError(t, db.Dao.Create(earlierHigh).Error)

	later := &models.TradePlan{
		TradeDate: "2026-07-24", GeneratedAt: now, PoolID: 22,
		Status: models.TradePlanStatusReady, PlanVersion: 9,
		FreezeAt: &freezeAt, FreezeBy: "c",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, db.Dao.Create(later).Error)

	view, err := repo.GetUpcomingTradePlan(today)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, "2026-07-23", view.TradeDate)
	require.Equal(t, 2, view.PlanVersion)
	require.Equal(t, earlierHigh.ID, view.ID)
}

func TestGetUpcomingTradePlan_NoPlan(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()

	// Past-only plans must be ignored.
	past := &models.TradePlan{
		TradeDate: "2026-07-20", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
	}
	require.NoError(t, repo.CreatePlanWithItems(past, nil))

	view, err := repo.GetUpcomingTradePlan("2026-07-22")
	require.Error(t, err)
	require.True(t, IsNoUpcomingTradePlan(err))
	require.Nil(t, view)
}

func TestGetUpcomingTradePlan_ReadOnlyDoesNotMutateDB(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	freezeAt := now
	plan := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 99,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freezeAt, FreezeBy: "ops", FreezeReason: "freeze",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:   risk.PlanRiskStatusPassed,
		Message:      "immutable",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Priority: 1, TargetAmount: 5000, Status: models.TradePlanItemPending},
	}))

	var beforePlan models.TradePlan
	require.NoError(t, db.Dao.First(&beforePlan, plan.ID).Error)
	var beforePlanCount, beforeItemCount int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&beforePlanCount).Error)
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Count(&beforeItemCount).Error)

	view, err := repo.GetUpcomingTradePlan("2026-07-22")
	require.NoError(t, err)
	require.NotNil(t, view)

	var afterPlan models.TradePlan
	require.NoError(t, db.Dao.First(&afterPlan, plan.ID).Error)
	require.Equal(t, beforePlan.Status, afterPlan.Status)
	require.Equal(t, beforePlan.PlanVersion, afterPlan.PlanVersion)
	require.Equal(t, beforePlan.Message, afterPlan.Message)
	require.Equal(t, beforePlan.UpdatedAt.UnixNano(), afterPlan.UpdatedAt.UnixNano())
	if beforePlan.FreezeAt != nil && afterPlan.FreezeAt != nil {
		require.True(t, beforePlan.FreezeAt.Equal(*afterPlan.FreezeAt))
	}

	var afterPlanCount, afterItemCount int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&afterPlanCount).Error)
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Count(&afterItemCount).Error)
	require.Equal(t, beforePlanCount, afterPlanCount)
	require.Equal(t, beforeItemCount, afterItemCount)
}

func TestGetUpcomingTradePlan_ViewDoesNotExposeModelPointer(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	draft := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 7,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusBypassed,
		ApprovedAt:    &now, ApprovedBy: "alice",
	}
	require.NoError(t, repo.CreatePlanWithItems(draft, []models.TradePlanItem{
		{StockCode: "sh600000", StockName: "浦发", Side: "buy", Priority: 1, TargetAmount: 8000, Status: models.TradePlanItemPending, StrategyName: "s1"},
	}))

	view, err := repo.GetUpcomingTradePlan("2026-07-22")
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, draft.ID, view.ID)
	require.Equal(t, "after_close", view.SourceSession)
	require.Equal(t, uint(7), view.PoolID)
	require.True(t, view.RiskPassed)
	require.Equal(t, "alice", view.ApprovedBy)
	require.NotNil(t, view.ApprovedAt)
	require.Len(t, view.Items, 1)
	require.Equal(t, "sh600000", view.Items[0].StockCode)
	require.Equal(t, "s1", view.Items[0].StrategyName)
	// Ensure return type is visibility DTO (compile-time + runtime shape).
	var _ *TradePlanVisibilityView = view
}
