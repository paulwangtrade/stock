package main

import (
	"context"
	"encoding/json"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)


// 判断是否运行集成测试
func skipIntegrationTest(t *testing.T) {
	t.Helper()

	// 默认跳过需要真实环境/Wails环境的测试
	if os.Getenv("RUN_INTEGRATION_TEST") != "1" {
		t.Skip("skip integration test")
	}
}


// @Author spark
// @Date 2025/2/24
// @Desc


func TestIsHKTradingTime(t *testing.T) {
	f := IsHKTradingTime(time.Now())
	t.Log(f)
}


func TestIsUSTradingTime(t *testing.T) {

	date := time.Now()
	hour, minute, _ := date.Clock()

	logger.SugaredLogger.Infof(
		"当前时间: %d:%d",
		hour,
		minute,
	)

	t.Log(IsUSTradingTime(time.Now()))
}



func TestCheckStockBaseInfo(t *testing.T) {

	if testing.Short() {
        t.Skip("skip network test")
    }

	skipIntegrationTest(t)

	db.Init("./data/stock.db")

	NewApp().CheckStockBaseInfo(
		context.Background(),
	)
}



func TestJson(t *testing.T) {

	jsonStr := `{
	"id":3334,
	"code":"PUK.US",
	"name":"英国保诚集团",
	"exchange":"NASDAQ",
	"type":"stock"
	}`


	v := &models.StockInfoUS{}

	err := json.Unmarshal(
		[]byte(jsonStr),
		v,
	)

	if err != nil {
		t.Fatal(err)
	}


	logger.SugaredLogger.Infof(
		"v:%+v",
		v,
	)
}



func TestUpdateCheck(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("app_update.go")
	require.NoError(t, err)
	require.NotContains(t, string(b), "api.github.com/repos/ArvinLovegood")
}



func TestGetScreenResolution(t *testing.T) {

	x,y,w,h,err:=getScreenResolution()

	if err != nil {
		logger.SugaredLogger.Errorf(
			"get screen resolution error:%s",
			err.Error(),
		)
		return
	}


	logger.SugaredLogger.Infof(
		"x:%d,y:%d,w:%d,h:%d",
		x,y,w,h,
	)

}



func TestCheckUpdate(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("app_update.go")
	require.NoError(t, err)
	require.Contains(t, string(b), "version.CheckWithProviders")
	require.NotContains(t, string(b), "ArvinLovegood")
}




func TestGetAiRecommendStocksList(t *testing.T){

	skipIntegrationTest(t)

	db.Init("./data/stock.db")


	str:=`{
	"startDate":"2026-03-20 00:00:00",
	"endDate":"2026-03-27 23:59:59",
	"page":1,
	"pageSize":5000
	}`


	query:=&models.AiRecommendStocksQuery{}

	json.Unmarshal(
		[]byte(str),
		query,
	)


	pageData,err:=data.
		NewAiRecommendStocksService().
		GetAiRecommendStocksList(query)


	if err!=nil{
		t.Fatal(err)
	}


	logger.SugaredLogger.Infof(
		"pageData:%+v",
		pageData.List,
	)



	var export []models.AiRecommendStocksMdExport


	for _,v:=range pageData.List{
		export=append(
			export,
			v.ToMdExportStruct(),
		)
	}


	content:=util.MarkdownTableWithTitle(
		"近期AI分析/推荐股票明细列表",
		export,
	)


	logger.SugaredLogger.Infof(
		"content:%s",
		content,
	)

}




func TestSummaryStockNews(t *testing.T){

	if testing.Short() {
        t.Skip("skip summary stock news test")
    }
	skipIntegrationTest(t)

	db.Init("./data/stock.db")


	question:=
		"分析今日的市场行情走势是否和券商的观点一致"


	app:=NewApp()


	msgs:=data.
		NewDeepSeekOpenAi(
			app.ctx,
			0,
		).
		NewSummaryStockNewsStreamWithTools(
			question,
			nil,
			app.AiTools,
			true,
			nil,
		)



	content:=strings.Builder{}


	for msg:=range msgs{

		logger.SugaredLogger.Infof(
			"msg:%+v",
			msg,
		)


		content.WriteString(
			msg["content"].(string),
		)
	}


	logger.SugaredLogger.Infof(
		"content:%s",
		content.String(),
	)

}





func TestCalculateNextRunTime(t *testing.T){

	t.Log(
		NewApp().
			CalculateNextRunTime(
				"0 0 0 * * ?",
			),
	)

}




func TestFetchAiModels(t *testing.T){
	if testing.Short() {
        t.Skip("skip AI model api test")
    }

	skipIntegrationTest(t)

	app:=NewApp()

	models:=app.FetchAiModels(
		"https://ark.cn-beijing.volces.com/api/v3",
		"",
	)

	t.Log(models)

}