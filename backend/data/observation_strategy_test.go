package data

import (
	"encoding/json"
	"fmt"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupObservationStrategyDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:observation_strategy_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, testDB.AutoMigrate(&models.StockStrategy{}, &models.StockStrategyRun{}))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestObservationStrategies_SeedDefaultsAndCronDoesNotFeed(t *testing.T) {
	setupObservationStrategyDB(t)
	api := NewStockStrategyApi()
	require.NoError(t, api.EnsureObservationStrategies())
	require.NoError(t, api.EnsureObservationStrategies())

	var rows []models.StockStrategy
	require.NoError(t, db.Dao.Where("query_type = ?", ObservationQueryType).Order("id ASC").Find(&rows).Error)
	require.Len(t, rows, 3)

	seen := map[string]bool{}
	for _, row := range rows {
		meta, ok := ParseObservationMeta(row.QueryJSON)
		require.True(t, ok)
		require.True(t, IsObservationStrategyID(meta.StrategyID))
		require.False(t, meta.FeedsTradePlan)
		require.False(t, meta.TrackA)
		require.True(t, meta.ObservationOnly)
		require.False(t, row.Enable)
		require.False(t, StrategyFeedsTradePlan(&row))
		require.NotEmpty(t, row.Name)
		require.NotEmpty(t, row.Description)
		seen[meta.StrategyID] = true
	}
	require.Equal(t, map[string]bool{
		StrategyIDMaPullback:  true,
		StrategyIDVolBreakout: true,
		StrategyIDDdBounce:    true,
	}, seen)

	row := rows[0]
	row.Enable = true
	row.CronExpr = ""
	require.NoError(t, api.Update(&row))
	got, err := api.GetByID(row.ID)
	require.NoError(t, err)
	require.True(t, got.Enable)
	require.Equal(t, ObservationCronExpr, got.CronExpr)
	meta, ok := ParseObservationMeta(got.QueryJSON)
	require.True(t, ok)
	require.False(t, meta.FeedsTradePlan)
	require.False(t, meta.TrackA)
	require.False(t, StrategyFeedsTradePlan(got))

	view := api.RunStrategy(got)
	require.Equal(t, 0, view.Code)
	require.Contains(t, view.Message, "未生成交易计划")
	require.Contains(t, view.Message, meta.StrategyID)
	require.Equal(t, 0, view.StockCount)

	legacy := &models.StockStrategy{
		Name:      "legacy-nl",
		QueryType: "eastmoney_nl",
		QueryText: "RSI小于30",
		Enable:    true,
		CronExpr:  "0 35 9 * * 1-5",
		PageSize:  20,
	}
	require.NoError(t, api.Create(legacy))
	src, err := api.GetFirstTradePlanSource()
	require.NoError(t, err)
	require.Equal(t, legacy.ID, src.ID)
	require.NotEqual(t, ObservationQueryType, src.QueryType)

	meta.FeedsTradePlan = true
	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	got.QueryJSON = string(raw)
	got.Enable = true
	require.NoError(t, api.Update(got))
	fed, err := api.GetByID(got.ID)
	require.NoError(t, err)
	require.True(t, StrategyFeedsTradePlan(fed))
	src, err = api.GetFirstTradePlanSource()
	require.NoError(t, err)
	require.Equal(t, fed.ID, src.ID)

	page := api.List(&models.StockStrategyQuery{Page: 1, PageSize: 20, ExcludeQueryType: ObservationQueryType})
	require.Equal(t, 1, page.Total)
	require.Equal(t, "legacy-nl", page.Data[0].Name)
	obs := api.List(&models.StockStrategyQuery{Page: 1, PageSize: 20, QueryType: ObservationQueryType})
	require.Equal(t, 3, obs.Total)
}

func TestObservationStrategy_RejectsUnknownID(t *testing.T) {
	setupObservationStrategyDB(t)
	err := NewStockStrategyApi().Create(&models.StockStrategy{
		Name:      "bad",
		QueryType: ObservationQueryType,
		QueryJSON: `{"strategyId":"ext_xsmom_v1","feedsTradePlan":true}`,
	})
	require.Error(t, err)
}

func TestStrategyFeedsTradePlan_InvalidJSONFailsClosed(t *testing.T) {
	s := &models.StockStrategy{
		Enable:    true,
		QueryType: ObservationQueryType,
		QueryJSON: `{`,
	}
	require.False(t, StrategyFeedsTradePlan(s))
	require.False(t, StrategyFeedsTradePlan(&models.StockStrategy{Enable: false, QueryType: "eastmoney_nl"}))
	require.True(t, StrategyFeedsTradePlan(&models.StockStrategy{Enable: true, QueryType: "eastmoney_nl"}))
}
