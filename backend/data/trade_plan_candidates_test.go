package data

import (
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestListSameDayCandidates_MultiOrigin(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	day := "2026-09-07"

	afterClose := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		Side: "buy", SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus: risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(afterClose, []models.TradePlanItem{{
		TradeDate: day, StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	watchlist := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		Side: "buy", SourceSession: models.TradePlanSourceWatchlist,
	}
	require.NoError(t, repo.CreatePlanWithItems(watchlist, []models.TradePlanItem{{
		TradeDate: day, StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	superseded := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusSuperseded, PlanVersion: 0,
		Side: "buy", SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(superseded, nil))

	rows, err := repo.ListSameDayCandidates(day)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	byID := map[uint]SameDayCandidateView{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	require.Contains(t, byID, afterClose.ID)
	require.Contains(t, byID, watchlist.ID)
	require.Equal(t, "strategy", byID[afterClose.ID].Source)
	require.Equal(t, models.TradePlanSourceAfterClose, byID[afterClose.ID].SourceSession)
	require.Equal(t, "watchlist", byID[watchlist.ID].Source)
	require.Equal(t, models.TradePlanSourceWatchlist, byID[watchlist.ID].SourceSession)

	require.Equal(t, watchlist.ID, rows[0].ID)
	require.Equal(t, afterClose.ID, rows[1].ID)
}

func TestListSameDayCandidates_ReadyBeforeDraft(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	day := "2026-09-08"
	freeze := now

	draft := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 9,
		Side: "buy", SourceSession: models.TradePlanSourceWatchlist,
	}
	require.NoError(t, repo.CreatePlanWithItems(draft, nil))

	ready := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freeze, Side: "buy",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(ready, nil))

	rows, err := repo.ListSameDayCandidates(day)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, ready.ID, rows[0].ID)
	require.True(t, rows[0].IsFrozen)
	require.Equal(t, draft.ID, rows[1].ID)
}

func TestListSameDayCandidates_DoesNotChangeUpcoming(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	day := "2026-09-09"

	afterClose := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		Side: "buy", SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(afterClose, []models.TradePlanItem{{
		TradeDate: day, StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending,
	}}))
	watchlist := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		Side: "buy", SourceSession: models.TradePlanSourceWatchlist,
	}
	require.NoError(t, repo.CreatePlanWithItems(watchlist, []models.TradePlanItem{{
		TradeDate: day, StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	cands, err := repo.ListSameDayCandidates(day)
	require.NoError(t, err)
	require.Len(t, cands, 2)

	up, err := repo.GetUpcomingTradePlan(day)
	require.NoError(t, err)
	require.Equal(t, watchlist.ID, up.ID)
}
