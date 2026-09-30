package strategy

import (
	"fmt"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupObservationFeedDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:observation_feed_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, testDB.AutoMigrate(&models.StockStrategy{}, &models.StockStrategyRun{}, &data.FollowedStock{}))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestCollectUniverse_ObservationCronDoesNotFeedTradePlan(t *testing.T) {
	setupObservationFeedDB(t)
	api := data.NewStockStrategyApi()
	require.NoError(t, api.EnsureObservationStrategies())

	var obs models.StockStrategy
	require.NoError(t, db.Dao.Where("query_type = ?", data.ObservationQueryType).Order("id ASC").First(&obs).Error)
	obs.Enable = true
	obs.CronExpr = data.ObservationCronExpr
	require.NoError(t, api.Update(&obs))
	require.NoError(t, db.Dao.Create(&models.StockStrategyRun{
		StrategyID: obs.ID,
		StockCount: 1,
		ResultJSON: `{"dataList":[{"SECURITY_CODE":"600036","SECURITY_NAME_ABBR":"招商银行"}]}`,
	}).Error)

	legacy := &models.StockStrategy{
		Name:      "legacy-nl",
		QueryType: "eastmoney_nl",
		QueryText: "RSI小于30",
		Enable:    true,
		PageSize:  20,
	}
	require.NoError(t, api.Create(legacy))
	require.NoError(t, db.Dao.Create(&models.StockStrategyRun{
		StrategyID: legacy.ID,
		StockCount: 1,
		ResultJSON: `{"dataList":[{"SECURITY_CODE":"600000","SECURITY_NAME_ABBR":"浦发银行"}]}`,
	}).Error)

	got := collectUniverse()
	require.Equal(t, legacy.ID, got.StrategyID)
	require.Equal(t, "legacy-nl", got.StrategyName)
	require.Len(t, got.Items, 1)
	require.Equal(t, "sh600000", got.Items[0].StockCode)
}
