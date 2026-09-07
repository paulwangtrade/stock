package opportunity_test

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"

	"github.com/stretchr/testify/require"
)

func seedWatchlistTradePlan(t *testing.T, tradeDate, stockCode, planStatus, itemStatus string) *models.TradePlan {
	t.Helper()
	require.NoError(t, data.EnsureTradePlanTables())
	plan := &models.TradePlan{
		TradeDate:   tradeDate,
		GeneratedAt: time.Now(),
		Status:      planStatus,
		Side:        "buy",
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate,
		StockCode: stockCode,
		Side:      "buy",
		Status:    itemStatus,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	return plan
}

func TestListWatching_TradePlanBadge_NONE(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	batch := "2026-09-06|close|default"
	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		StockCode:    "sz300620",
		Secucode:     "300620.SZ",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	require.False(t, view.Items[0].InTradePlan)
	require.Equal(t, opportunity.WatchlistTradePlanNone, view.Items[0].TradePlanStatus)
	require.Equal(t, uint(0), view.Items[0].TradePlanID)
}

func TestListWatching_TradePlanBadge_DRAFT(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	plan := seedWatchlistTradePlan(t, "2026-09-06", "sz300620", models.TradePlanStatusDraft, models.TradePlanItemPending)

	batch := "2026-09-06|close|default"
	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		StockCode:    "sz300620",
		Secucode:     "300620.SZ",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.Len(t, view.Items, 1)
	require.True(t, view.Items[0].InTradePlan)
	require.Equal(t, opportunity.WatchlistTradePlanDraft, view.Items[0].TradePlanStatus)
	require.Equal(t, plan.ID, view.Items[0].TradePlanID)
}

func TestListWatching_TradePlanBadge_PreferNonTerminal(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	done := &models.TradePlan{
		TradeDate: "2026-09-01", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDone, Side: "buy",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(done, []models.TradePlanItem{{
		TradeDate: "2026-09-01", StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemFilled,
	}}))

	draft := &models.TradePlan{
		TradeDate: "2026-09-05", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(draft, []models.TradePlanItem{{
		TradeDate: "2026-09-05", StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	batch := "2026-09-06|close|default"
	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		StockCode:    "sz300620",
		Secucode:     "300620.SZ",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.True(t, view.Items[0].InTradePlan)
	require.Equal(t, opportunity.WatchlistTradePlanDraft, view.Items[0].TradePlanStatus)
	require.Equal(t, draft.ID, view.Items[0].TradePlanID)
}

func TestListWatching_TradePlanBadge_SkippedItemIgnored(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	seedWatchlistTradePlan(t, "2026-09-06", "sz300620", models.TradePlanStatusDraft, models.TradePlanItemSkipped)

	batch := "2026-09-06|close|default"
	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		StockCode:    "sz300620",
		Secucode:     "300620.SZ",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.False(t, view.Items[0].InTradePlan)
	require.Equal(t, opportunity.WatchlistTradePlanNone, view.Items[0].TradePlanStatus)
	require.Equal(t, uint(0), view.Items[0].TradePlanID)
}
