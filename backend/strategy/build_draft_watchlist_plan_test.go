package strategy_test

import (
	"errors"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWatchlistDraftTestDB(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	original := db.Dao
	dsn := "file:watchlist_draft_" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=10000"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = original })

	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	require.NoError(t, data.EnsureTradePlanTables())
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func TestBuildDraftWatchlistTradePlan_CreatesBuyDraft(t *testing.T) {
	acc := setupWatchlistDraftTestDB(t)
	batch := "2026-09-06|close|default"
	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		StockCode:    "sz300620",
		Secucode:     "300620.SZ",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)

	plan, err := strategy.BuildDraftWatchlistTradePlan(strategy.WatchlistDraftRequest{
		AccountID:     acc.ID,
		TradeDate:     "2026-09-07",
		StockCode:     "sz300620",
		StockName:     "测试票",
		OpportunityID: row.OpportunityID,
		ScanBatchKey:  batch,
		Actor:         "test",
	})
	require.NoError(t, err)
	require.NotZero(t, plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, "buy", plan.Side)
	require.False(t, plan.EnableExecute)
	require.Equal(t, models.TradePlanSourceWatchlist, plan.SourceSession)
	require.Equal(t, 1, plan.MaxNames)
	require.Greater(t, plan.AmountPerStock, 0.0)
	require.Nil(t, plan.ApprovedAt)
	require.Nil(t, plan.FreezeAt)
	require.Len(t, plan.Items, 1)
	require.Equal(t, "sz300620", plan.Items[0].StockCode)
	require.Equal(t, "buy", plan.Items[0].Side)
	require.Equal(t, int64(0), plan.Items[0].TargetVolume)
	require.Equal(t, 0.0, plan.Items[0].LimitPrice)
	require.Equal(t, plan.AmountPerStock, plan.Items[0].TargetAmount)
}

func TestBuildDraftWatchlistTradePlan_RejectsWhenPlanExists(t *testing.T) {
	acc := setupWatchlistDraftTestDB(t)
	existing := &models.TradePlan{
		TradeDate: "2026-09-06", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(existing, []models.TradePlanItem{{
		TradeDate: "2026-09-06", StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	batch := "2026-09-06|close|default"
	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID: acc.ID, ScanBatchKey: batch, StockCode: "sz300620", Secucode: "300620.SZ",
		Action: opportunity.ActionWatch,
	})
	require.NoError(t, err)

	_, err = strategy.BuildDraftWatchlistTradePlan(strategy.WatchlistDraftRequest{
		AccountID: acc.ID, TradeDate: "2026-09-07", StockCode: "sz300620",
		OpportunityID: row.OpportunityID, ScanBatchKey: batch, Actor: "test",
	})
	require.Error(t, err)
	var conflict *strategy.WatchlistDraftConflict
	require.True(t, errors.As(err, &conflict))
	require.Equal(t, existing.ID, conflict.PlanID)
	require.Equal(t, opportunity.WatchlistTradePlanDraft, conflict.TradePlanStatus)
}

func TestBuildDraftWatchlistTradePlan_RejectsWhenNotWatching(t *testing.T) {
	acc := setupWatchlistDraftTestDB(t)
	batch := "2026-09-06|close|default"
	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID: acc.ID, ScanBatchKey: batch, StockCode: "sz300620", Secucode: "300620.SZ",
		Action: opportunity.ActionWatch,
	})
	require.NoError(t, err)
	_, err = opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID: acc.ID, ScanBatchKey: batch, OpportunityID: row.OpportunityID,
		StockCode: "sz300620", Action: opportunity.ActionIgnore,
	})
	require.NoError(t, err)

	_, err = strategy.BuildDraftWatchlistTradePlan(strategy.WatchlistDraftRequest{
		AccountID: acc.ID, TradeDate: "2026-09-07", StockCode: "sz300620",
		OpportunityID: row.OpportunityID, ScanBatchKey: batch, Actor: "test",
	})
	require.ErrorIs(t, err, strategy.ErrWatchlistDraftNotWatching)
}
