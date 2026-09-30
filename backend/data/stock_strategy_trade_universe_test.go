package data

import (
	"fmt"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupStockStrategyTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:stock_strategy_gate_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, testDB.AutoMigrate(&models.StockStrategy{}, &models.StockStrategyRun{}, &FollowedStock{}))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestCreateStockStrategy_EnableCronDoesNotFeedTradePlan(t *testing.T) {
	setupStockStrategyTestDB(t)
	api := NewStockStrategyApi()

	observe := &models.StockStrategy{
		Name:      "观察策略",
		QueryType: "eastmoney_nl",
		QueryText: "RSI小于30",
		Enable:    true,
		CronExpr:  "0 35 9 * * 1-5",
	}
	require.NoError(t, api.Create(observe))

	got, err := api.GetByID(observe.ID)
	require.NoError(t, err)
	require.True(t, got.Enable)
	require.False(t, got.FeedsTradePlan, "new strategy defaults feedsTradePlan=false even when cron is enabled")

	strat, uerr := api.GetTradeUniverseStrategy()
	require.Error(t, uerr)
	require.Nil(t, strat)

	enabled := api.GetAllEnabled()
	require.Len(t, enabled, 1)
	require.Equal(t, observe.ID, enabled[0].ID)

	hooked, id, name, msg := api.TradeUniverseStatus()
	require.False(t, hooked)
	require.Zero(t, id)
	require.Empty(t, name)
	require.Equal(t, TradeUniverseUnhookedMessage, msg)

	page := api.List(&models.StockStrategyQuery{Page: 1, PageSize: 20})
	require.False(t, page.TradeUniverseHooked)
	require.Equal(t, TradeUniverseUnhookedMessage, page.TradeUniverseMessage)
	require.Len(t, page.Data, 1)
	require.False(t, page.Data[0].FeedsTradePlan)
}

func TestGetTradeUniverseStrategy_RequiresFeedsTradePlan(t *testing.T) {
	setupStockStrategyTestDB(t)
	api := NewStockStrategyApi()

	cronOnly := &models.StockStrategy{
		Name: "cron-only", QueryType: "eastmoney_nl", Enable: true, CronExpr: "0 35 9 * * 1-5",
	}
	paper := &models.StockStrategy{
		Name: "paper-universe", QueryType: "eastmoney_nl", Enable: false, FeedsTradePlan: true,
	}
	require.NoError(t, api.Create(cronOnly))
	require.NoError(t, api.Create(paper))

	got, err := api.GetTradeUniverseStrategy()
	require.NoError(t, err)
	require.Equal(t, paper.ID, got.ID)
	require.NotEqual(t, cronOnly.ID, got.ID)
	require.False(t, got.Enable)
	require.True(t, got.FeedsTradePlan)

	firstEnabled, err := api.GetFirstEnabled()
	require.NoError(t, err)
	require.Equal(t, cronOnly.ID, firstEnabled.ID, "cron enable stays a separate query")
}

func TestGetTradeUniverseStrategy_UsesLowestIDWhenMultiple(t *testing.T) {
	setupStockStrategyTestDB(t)
	api := NewStockStrategyApi()
	first := &models.StockStrategy{Name: "first", QueryType: "technical", FeedsTradePlan: true}
	second := &models.StockStrategy{Name: "second", QueryType: "technical", Enable: true, CronExpr: "0 5 15 * * 1-5", FeedsTradePlan: true}
	require.NoError(t, api.Create(first))
	require.NoError(t, api.Create(second))

	got, err := api.GetTradeUniverseStrategy()
	require.NoError(t, err)
	require.Equal(t, first.ID, got.ID)
	require.Equal(t, 2, api.CountFeedsTradePlan())
}
