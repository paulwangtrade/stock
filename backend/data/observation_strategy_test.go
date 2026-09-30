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
	require.Equal(t, 4, page.Total)
	require.Len(t, page.Data, 4)

	byID := map[string]models.StockStrategy{}
	for _, row := range page.Data {
		id := observationStrategyID(&row)
		byID[id] = row
		require.False(t, row.Enable, id)
		require.Empty(t, row.CronExpr, id)
		require.False(t, observationFeedsTradePlan(&row), id)
		require.Equal(t, ObservationQueryType, row.QueryType)
	}
	for _, id := range []string{"ext_ma_pullback", "ext_vol_breakout", "ext_dd_bounce", "ext_rounded_bottom_v1"} {
		require.Contains(t, byID, id)
	}
	rounded := byID["ext_rounded_bottom_v1"]
	require.Equal(t, "圆弧底近似（观察）", rounded.Name)
	require.Contains(t, rounded.Description, "1.3倍")
	require.NotContains(t, rounded.Description, "高胜率")

	main := api.List(&models.StockStrategyQuery{Page: 1, PageSize: 20})
	require.Equal(t, 1, main.Total)
	require.Equal(t, "eastmoney_nl", main.Data[0].QueryType)

	for _, id := range []string{"ext_ma_pullback", "ext_vol_breakout", "ext_dd_bounce", "ext_rounded_bottom_v1"} {
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
	hit := maPullbackHitBars()
	require.True(t, evalMaPullback(hit))
	require.False(t, evalMaPullback(maPullbackFarBars()))
	require.False(t, evalMaPullback(hit[:10]))
	flat := append([]observationBar(nil), hit...)
	for i := range flat {
		flat[i].Open = 9.9
		flat[i].Close = 10
		flat[i].High = 10.2
		flat[i].Low = 9.95
	}
	require.False(t, evalMaPullback(flat), "flat MA is not rising")
	noOpen := append([]observationBar(nil), hit...)
	noOpen[len(noOpen)-1].Open = 0
	require.True(t, evalMaPullback(noOpen), "missing open uses close > prior close")
	yin := append([]observationBar(nil), hit...)
	last := len(yin) - 1
	yin[last].Open = yin[last].Close + 0.2
	require.False(t, evalMaPullback(yin), "touch without yang recovery")

	require.True(t, evalVolBreakout(volBreakoutHitBars()))
	require.False(t, evalVolBreakout(volBreakoutQuietBars()))
	require.False(t, evalVolBreakout(volBreakoutHitBars()[:10]))
	zeroLast := volBreakoutHitBars()
	zeroLast[len(zeroLast)-1].Volume = 0
	require.False(t, evalVolBreakout(zeroLast))
	zeroPrior := volBreakoutHitBars()
	zeroPrior[10].Volume = 0
	require.False(t, evalVolBreakout(zeroPrior))

	dd := ddBounceHitBars()
	require.True(t, evalDdBounce(dd))
	rsi, ok := rsiAt(dd, observationRSIPeriod)
	require.True(t, ok)
	require.GreaterOrEqual(t, rsi, observationRSIMin)
	require.False(t, evalDdBounce(dd[:30]))
	newLow := append([]observationBar(nil), dd...)
	newLow[len(newLow)-1].Low = 1
	require.False(t, evalDdBounce(newLow), "60d new low overlaps ice")
	oversold := ddOversoldBars()
	require.False(t, evalDdBounce(oversold), "RSI below 30 is ice oversold")
	rsiOver, okOver := rsiAt(oversold, observationRSIPeriod)
	require.True(t, okOver)
	require.Less(t, rsiOver, observationRSIMin)
}

func maPullbackHitBars() []observationBar {
	const n = 30
	bars := make([]observationBar, n)
	for i := 0; i < n; i++ {
		c := 10 + 0.15*float64(i)
		bars[i] = observationBar{Open: c - 0.05, Close: c, High: c + 0.08, Low: c - 0.05, Volume: 1000}
	}
	ma := smaClose(bars, n-1, observationMaPeriod)
	bars[n-1].Low = ma * 1.01
	bars[n-1].Open = bars[n-1].Low + 0.05
	if bars[n-1].High < bars[n-1].Close {
		bars[n-1].High = bars[n-1].Close
	}
	return bars
}

func maPullbackFarBars() []observationBar {
	bars := maPullbackHitBars()
	n := len(bars)
	for i := n - 3; i < n; i++ {
		bars[i].Low = bars[i].Close * 0.995
	}
	return bars
}

func volBreakoutHitBars() []observationBar {
	bars := make([]observationBar, 21)
	for i := 0; i < 20; i++ {
		bars[i] = observationBar{Open: 9.8, Close: 10, High: 10, Low: 9.5, Volume: 100}
	}
	bars[20] = observationBar{Open: 10.2, Close: 11, High: 11, Low: 10.5, Volume: 200}
	return bars
}

func volBreakoutQuietBars() []observationBar {
	bars := volBreakoutHitBars()
	bars[20].Volume = 100
	return bars
}

func ddBounceHitBars() []observationBar {
	bars := make([]observationBar, observationNewLowBars)
	for i := range bars {
		bars[i] = observationBar{Open: 99.5, Close: 100, High: 100.2, Low: 99, Volume: 1000}
	}
	bars[0].Low = 70
	for i := 46; i <= 58; i++ {
		c := 90 + float64(i-45)*0.4
		bars[i] = observationBar{Open: c - 0.3, Close: c, High: c + 0.2, Low: c - 0.2, Volume: 1000}
	}
	bars[40] = observationBar{Open: 99, Close: 100, High: 100, Low: 98, Volume: 1000}
	bars[45] = observationBar{Open: 91, Close: 90, High: 92, Low: 88, Volume: 1000}
	bars[59] = observationBar{Open: 96, Close: 97, High: 97.5, Low: 95.5, Volume: 1200}
	return bars
}

func ddOversoldBars() []observationBar {
	bars := ddBounceHitBars()
	for i := 45; i <= 58; i++ {
		c := 100 - float64(i-44)
		bars[i].Open = c + 0.4
		bars[i].Close = c
		bars[i].High = c + 0.5
		bars[i].Low = c - 0.3
	}
	bars[45].Low = 88
	bars[59].Open = 87
	bars[59].Close = 88
	bars[59].High = 88.4
	bars[59].Low = 86.5
	return bars
}

func TestRoundedBottom_HitMissSkip(t *testing.T) {
	hit := roundedBottomHitBars()
	require.True(t, evalRoundedBottom(hit))
	require.GreaterOrEqual(t, len(hit), roundedBottomMinBars)

	quiet := roundedBottomHitBars()
	quiet[len(quiet)-1].Volume = 1299
	require.False(t, evalRoundedBottom(quiet), "volume below 1.3x prior average")

	wide := roundedBottomHitBars()
	wide[70].High = 110
	wide[70].Low = 80
	require.False(t, evalRoundedBottom(wide), "recent range is not narrower")

	shallow := roundedBottomHitBars()
	for i := 21; i <= 68; i++ {
		if shallow[i].Close < 97 {
			shallow[i].Close = 97
		}
		if shallow[i].High < shallow[i].Close {
			shallow[i].High = shallow[i].Close
		}
		if shallow[i].Low > shallow[i].High {
			shallow[i].Low = shallow[i].High
		}
	}
	require.False(t, evalRoundedBottom(shallow), "close drawdown under 12%")

	wick := roundedBottomHitBars()
	for i := 21; i <= 68; i++ {
		if wick[i].Close < 98 {
			wick[i].Close = 98
		}
		if wick[i].Open < wick[i].Close {
			wick[i].Open = wick[i].Close - 0.1
		}
		if wick[i].High < wick[i].Close {
			wick[i].High = wick[i].Close
		}
	}
	wick[55].Low = 85
	require.False(t, evalRoundedBottom(wick), "lower wick without close drawdown")

	require.False(t, evalRoundedBottom(hit[:40]), "too few bars")
	require.False(t, evalRoundedBottom(nil))

	missingVol := roundedBottomHitBars()
	missingVol[70].Volume = 0
	require.False(t, evalRoundedBottom(missingVol), "missing volume")

	badPrice := roundedBottomHitBars()
	badPrice[30].Close = 0
	require.False(t, evalRoundedBottom(badPrice), "invalid price")

	badHigh := roundedBottomHitBars()
	badHigh[40].High = 1
	badHigh[40].Low = 2
	require.False(t, evalRoundedBottom(badHigh), "high below low")
}

func TestRunRoundedBottom_SkipsSTAndDoesNotFeedTradePlan(t *testing.T) {
	api := setupObservationStrategyDB(t)
	require.NoError(t, api.EnsureObservationStrategies())
	page := api.List(&models.StockStrategyQuery{QueryType: ObservationQueryType, Page: 1, PageSize: 20})
	var row models.StockStrategy
	for _, item := range page.Data {
		if observationStrategyID(&item) == "ext_rounded_bottom_v1" {
			row = item
			break
		}
	}
	require.NotZero(t, row.ID)
	require.False(t, row.Enable)
	require.False(t, observationFeedsTradePlan(&row))

	original := loadObservationUniverse
	loadObservationUniverse = func(int) []observationSymbol {
		return []observationSymbol{
			{Code: "sz000001", Name: "平安银行", Bars: roundedBottomHitBars()},
			{Code: "sz000002", Name: "ST圆弧", Bars: roundedBottomHitBars()},
			{Code: "sh600000", Name: "缺量样本", Bars: func() []observationBar {
				bars := roundedBottomHitBars()
				bars[len(bars)-1].Volume = 0
				return bars
			}()},
		}
	}
	t.Cleanup(func() { loadObservationUniverse = original })

	view := api.RunStrategy(&row)
	require.Equal(t, 0, view.Code, view.Message)
	require.Equal(t, 1, view.StockCount)
	require.Equal(t, "success", view.Message)
	require.Contains(t, view.TraceInfo, "圆弧底近似（观察）")
	require.Contains(t, view.TraceInfo, "不是交易指令")
	require.NotContains(t, view.TraceInfo, "高胜率")
	list, ok := view.DataList.([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "000001", list[0]["SECURITY_CODE"])
	require.Equal(t, "平安银行", list[0]["SECURITY_NAME_ABBR"])

	got, err := api.GetByID(row.ID)
	require.NoError(t, err)
	require.False(t, observationFeedsTradePlan(got))
	require.False(t, got.Enable)
	require.Empty(t, got.CronExpr)
	require.Empty(t, api.ListObservationFeedingTradePlan())
}

func roundedBottomHitBars() []observationBar {
	const n = 80
	bars := make([]observationBar, n)
	for i := 0; i < n; i++ {
		bars[i] = observationBar{Open: 99.8, Close: 100, High: 100.2, Low: 99.6, Volume: 1000}
	}
	for i := 25; i <= 54; i++ {
		c := 99 - float64(i-25)*0.2
		bars[i].Open = c + 0.1
		bars[i].Close = c
		bars[i].High = c + 0.3
		bars[i].Low = c - 0.4
		if bars[i].Low < 92 {
			bars[i].Low = 92
		}
	}
	bars[55] = observationBar{Open: 88.2, Close: 88, High: 88.4, Low: 85, Volume: 1000}
	for i := 56; i <= 58; i++ {
		bars[i] = observationBar{Open: 87.8, Close: 88, High: 88.5, Low: 87.6, Volume: 1000}
	}
	for i := 59; i <= 78; i++ {
		bars[i] = observationBar{Open: 89.9, Close: 90, High: 90.4, Low: 89.7, Volume: 1000}
	}
	bars[79] = observationBar{Open: 90.6, Close: 92, High: 92.2, Low: 90.5, Volume: 1300}
	return bars
}
