package strategy

import (
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCollectUniverse_ObservationCronDoesNotFeed(t *testing.T) {
	original := db.Dao
	dsn := "file:universe_obs_" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=10000"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = original })
	require.NoError(t, testDB.AutoMigrate(&models.StockStrategy{}, &models.StockStrategyRun{}))

	api := data.NewStockStrategyApi()
	require.NoError(t, api.EnsureObservationStrategies())

	ice := &models.StockStrategy{
		Name:      "冰点超跌",
		QueryType: "eastmoney_nl",
		QueryText: "RSI小于30",
		Enable:    true,
		CronExpr:  "0 5 15 * * 1-5",
		PageSize:  50,
	}
	require.NoError(t, testDB.Create(ice).Error)
	require.NoError(t, testDB.Create(&models.StockStrategyRun{
		StrategyID: ice.ID,
		StockCount: 1,
		ResultJSON: `{"queryType":"eastmoney_nl","dataList":[{"SECURITY_CODE":"600036","SECURITY_NAME_ABBR":"招商银行"}]}`,
	}).Error)

	page := api.List(&models.StockStrategyQuery{QueryType: data.ObservationQueryType, Page: 1, PageSize: 20})
	var obs models.StockStrategy
	for _, row := range page.Data {
		if row.QueryText == "ext_ma_pullback" {
			obs = row
			break
		}
	}
	require.NotZero(t, obs.ID)
	obs.Enable = true
	obs.CronExpr = "0 5 15 * * 1-5"
	require.NoError(t, api.Update(&obs))
	require.NoError(t, testDB.Create(&models.StockStrategyRun{
		StrategyID: obs.ID,
		StockCount: 1,
		ResultJSON: `{"queryType":"observation","dataList":[{"SECURITY_CODE":"000001","SECURITY_NAME_ABBR":"平安银行"}]}`,
	}).Error)

	primary := collectUniverse()
	require.Len(t, primary.Items, 1)
	require.Equal(t, "sh600036", primary.Items[0].StockCode)
	require.Equal(t, ice.ID, primary.StrategyID)

	reloaded, err := api.GetByID(obs.ID)
	require.NoError(t, err)
	reloaded.QueryJSON = `{"strategyId":"ext_ma_pullback","feedsTradePlan":true}`
	reloaded.Enable = true
	reloaded.CronExpr = "0 5 15 * * 1-5"
	require.NoError(t, api.Update(reloaded))

	fed := collectUniverse()
	require.Len(t, fed.Items, 2)
	require.Equal(t, "sh600036", fed.Items[0].StockCode)
	require.Equal(t, "sz000001", fed.Items[1].StockCode)
	require.Equal(t, "observation_feed", fed.Items[1].Reason)
	require.Equal(t, ice.ID, fed.StrategyID)
}
