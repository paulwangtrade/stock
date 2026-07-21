package data

import (
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestNextPlanVersion_EmptyDayStartsAtOne(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()

	v, err := repo.NextPlanVersion("2026-07-28")
	require.NoError(t, err)
	require.Equal(t, 1, v)
}

func TestNextPlanVersion_IncrementsFromMax(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()

	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-28", GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		EnableExecute: false, SourceSession: models.TradePlanSourceAfterClose,
	}, nil))
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-28", GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 3,
		EnableExecute: false, SourceSession: models.TradePlanSourceAfterClose,
	}, nil))
	// Different day must not affect version sequence.
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-29", GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 9,
		EnableExecute: false, SourceSession: models.TradePlanSourceAfterClose,
	}, nil))

	v, err := repo.NextPlanVersion("2026-07-28")
	require.NoError(t, err)
	require.Equal(t, 4, v)
}

func TestCreatePlanWithItems_DraftDoesNotSupersedePriorDraft(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()

	first := &models.TradePlan{
		TradeDate: "2026-07-28", GeneratedAt: now, PoolID: 11,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		EnableExecute: false, SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(first, []models.TradePlanItem{
		{StockCode: "sz000001", Status: models.TradePlanItemPending},
	}))
	second := &models.TradePlan{
		TradeDate: "2026-07-28", GeneratedAt: now, PoolID: 12,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		EnableExecute: false, SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(second, []models.TradePlanItem{
		{StockCode: "sh600519", Status: models.TradePlanItemPending},
	}))

	gotFirst, err := repo.GetByID(first.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, gotFirst.Status)
	require.Equal(t, 1, gotFirst.PlanVersion)

	gotSecond, err := repo.GetByID(second.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, gotSecond.Status)
	require.Equal(t, 2, gotSecond.PlanVersion)
	require.NotEqual(t, gotFirst.ID, gotSecond.ID)
}
