package data

import (
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupObservationStrategyDB(t *testing.T) *StockStrategyApi {
	t.Helper()
	original := db.Dao
	dsn := "file:obs_" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=10000"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = original })
	require.NoError(t, testDB.AutoMigrate(&models.StockStrategy{}, &models.StockStrategyRun{}))
	return NewStockStrategyApi()
}

func TestObservationStrategies_SeedDefaultsAndCronDoesNotFeed(t *testing.T) {
	api := setupObservationStrategyDB(t)
	ice := &models.StockStrategy{
		Name:      "冰点超跌",
		QueryType: "eastmoney_nl",
		QueryText: "RSI小于30",
		Enable:    true,
		CronExpr:  defaultObservationCron,
		PageSize:  50,
	}
	require.NoError(t, db.Dao.Create(ice).Error)

	require.NoError(t, api.EnsureObservationStrategies())
	require.NoError(t, api.EnsureObservationStrategies())

	page := api.List(&models.StockStrategyQuery{QueryType: ObservationQueryType, Page: 1, PageSize: 20})
	require.Equal(t, 3, page.Total)
	require.Len(t, page.Data, 3)

	byID := map[string]models.StockStrategy{}
	for _, row := range page.Data {
		id := observationStrategyID(&row)
		byID[id] = row
		require.False(t, row.Enable, id)
		require.Empty(t, row.CronExpr, id)
		require.False(t, observationFeedsTradePlan(&row), id)
		require.Equal(t, ObservationQueryType, row.QueryType)
	}
	for _, id := range []string{"ext_ma_pullback", "ext_vol_breakout", "ext_dd_bounce"} {
		require.Contains(t, byID, id)
	}

	main := api.List(&models.StockStrategyQuery{Page: 1, PageSize: 20})
	require.Equal(t, 1, main.Total)
	require.Equal(t, "eastmoney_nl", main.Data[0].QueryType)

	for _, id := range []string{"ext_ma_pullback", "ext_vol_breakout", "ext_dd_bounce"} {
		row := byID[id]
		row.Enable = true
		row.CronExpr = defaultObservationCron
		// 缺省 feedsTradePlan 不能因为打开定时而被写成 true。
		if id == "ext_vol_breakout" {
			row.QueryJSON = `{"strategyId":"ext_vol_breakout"}`
		}
		require.NoError(t, api.Update(&row))
		got, err := api.GetByID(row.ID)
		require.NoError(t, err)
		require.True(t, got.Enable, id)
		require.Equal(t, defaultObservationCron, got.CronExpr, id)
		require.False(t, observationFeedsTradePlan(got), id)

		view := api.RunStrategy(got)
		require.NotNil(t, view)
		require.Equal(t, 0, view.Code, view.Message)
		require.Equal(t, ObservationQueryType, view.QueryType)
		require.NotContains(t, view.Message, "不支持的策略类型")

		again, err := api.GetByID(row.ID)
		require.NoError(t, err)
		require.False(t, observationFeedsTradePlan(again), id)
		require.True(t, again.Enable, id)
	}

	require.Empty(t, api.ListObservationFeedingTradePlan())
	first, err := api.GetFirstEnabled()
	require.NoError(t, err)
	require.Equal(t, ice.ID, first.ID)
	require.Equal(t, 1, countEnabledStockStrategies())
}

func TestObservationFeedsTradePlan_OnlyExplicitBooleanTrue(t *testing.T) {
	api := setupObservationStrategyDB(t)
	require.NoError(t, api.EnsureObservationStrategies())
	page := api.List(&models.StockStrategyQuery{QueryType: ObservationQueryType, Page: 1, PageSize: 20})
	var row models.StockStrategy
	for _, item := range page.Data {
		if observationStrategyID(&item) == "ext_dd_bounce" {
			row = item
			break
		}
	}
	require.NotZero(t, row.ID)

	for _, raw := range []string{
		`{"strategyId":"ext_dd_bounce","feedsTradePlan":"true"}`,
		`{"strategyId":"ext_dd_bounce","feedsTradePlan":1}`,
		`{"strategyId":"ext_dd_bounce","feedsTradePlan":false}`,
	} {
		row.QueryJSON = raw
		row.Enable = true
		row.CronExpr = defaultObservationCron
		require.NoError(t, api.Update(&row))
		got, err := api.GetByID(row.ID)
		require.NoError(t, err)
		require.False(t, observationFeedsTradePlan(got), raw)
		require.True(t, got.Enable)
	}

	row.QueryJSON = `{"strategyId":"ext_dd_bounce","feedsTradePlan":true}`
	row.Enable = false
	row.CronExpr = ""
	require.NoError(t, api.Update(&row))
	got, err := api.GetByID(row.ID)
	require.NoError(t, err)
	require.True(t, observationFeedsTradePlan(got))
	require.False(t, got.Enable)
	require.Empty(t, got.CronExpr)
	require.Len(t, api.ListObservationFeedingTradePlan(), 1)

	first, err := api.GetFirstEnabled()
	require.Error(t, err)
	require.Nil(t, first)
}

func TestRunObservation_HitStaysOutOfTradePlanFlag(t *testing.T) {
	api := setupObservationStrategyDB(t)
	require.NoError(t, api.EnsureObservationStrategies())
	page := api.List(&models.StockStrategyQuery{QueryType: ObservationQueryType, Page: 1, PageSize: 20})
	var row models.StockStrategy
	for _, item := range page.Data {
		if observationStrategyID(&item) == "ext_ma_pullback" {
			row = item
			break
		}
	}
	require.NotZero(t, row.ID)

	original := loadObservationUniverse
	loadObservationUniverse = func(int) []observationSymbol {
		return []observationSymbol{
			{Code: "sh600036", Name: "招商银行", Bars: maPullbackHitBars()},
			{Code: "sh600000", Name: "*ST测试", Bars: maPullbackHitBars()},
		}
	}
	t.Cleanup(func() { loadObservationUniverse = original })

	view := api.RunStrategy(&row)
	require.Equal(t, 0, view.Code, view.Message)
	require.Equal(t, 1, view.StockCount)
	require.Equal(t, "success", view.Message)
	list, ok := view.DataList.([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "600036", list[0]["SECURITY_CODE"])
	require.Equal(t, "招商银行", list[0]["SECURITY_NAME_ABBR"])

	got, err := api.GetByID(row.ID)
	require.NoError(t, err)
	require.False(t, observationFeedsTradePlan(got))
	require.False(t, got.Enable)
	require.Empty(t, api.ListObservationFeedingTradePlan())
}

func TestObservationDayBarScreens(t *testing.T) {
	require.True(t, evalMaPullback(maPullbackHitBars()))
	require.False(t, evalMaPullback(maPullbackFarBars()))
	require.False(t, evalMaPullback(maPullbackHitBars()[:10]))

	require.True(t, evalVolBreakout(volBreakoutHitBars()))
	require.False(t, evalVolBreakout(volBreakoutQuietBars()))
	require.False(t, evalVolBreakout(volBreakoutHitBars()[:10]))

	require.True(t, evalDdBounce(ddBounceHitBars()))
	require.False(t, evalDdBounce(ddBounceHitBars()[:30]))
	flat := ddBounceHitBars()
	for i := range flat {
		flat[i] = observationBar{Close: 10, High: 10.1, Low: 9.9, Volume: 100}
	}
	require.False(t, evalDdBounce(flat))
}

func maPullbackHitBars() []observationBar {
	bars := make([]observationBar, 25)
	for i := range bars {
		bars[i] = observationBar{Close: 10, High: 10.2, Low: 9.9, Volume: 1000}
	}
	bars[22].Close = 10.05
	bars[23].Close = 10.02
	bars[24].Close = 10.08
	return bars
}

func maPullbackFarBars() []observationBar {
	bars := maPullbackHitBars()
	bars[22].Close = 12
	bars[23].Close = 12
	bars[24].Close = 12
	return bars
}

func volBreakoutHitBars() []observationBar {
	bars := make([]observationBar, 21)
	for i := 0; i < 20; i++ {
		bars[i] = observationBar{Close: 10, High: 10, Low: 9.5, Volume: 100}
	}
	bars[20] = observationBar{Close: 11, High: 11, Low: 10.5, Volume: 200}
	return bars
}

func volBreakoutQuietBars() []observationBar {
	bars := volBreakoutHitBars()
	bars[20].Volume = 100
	return bars
}

func ddBounceHitBars() []observationBar {
	bars := make([]observationBar, 60)
	for i := 0; i < 45; i++ {
		bars[i] = observationBar{Close: 100, High: 101, Low: 99, Volume: 1000}
	}
	for i := 45; i <= 58; i++ {
		c := 100 - float64(i-44)
		bars[i] = observationBar{Close: c, High: c + 0.2, Low: c - 0.2, Volume: 1000}
	}
	bars[59] = observationBar{Close: 88, High: 88.5, Low: 87.5, Volume: 1000}
	return bars
}
