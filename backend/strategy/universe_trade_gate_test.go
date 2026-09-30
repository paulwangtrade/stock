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

func setupUniverseGateDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:universe_gate_%s?mode=memory&cache=shared", t.Name())
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

func seedStrategyRun(t *testing.T, s *models.StockStrategy, code, name string) {
	t.Helper()
	api := data.NewStockStrategyApi()
	require.NoError(t, api.Create(s))
	raw := fmt.Sprintf(`{"code":0,"dataList":[{"SECURITY_CODE":"%s","SECURITY_NAME_ABBR":"%s"}]}`, code, name)
	run := &models.StockStrategyRun{StrategyID: s.ID, StockCount: 1, Message: "success", ResultJSON: raw}
	require.NoError(t, db.Dao.Create(run).Error)
}

func TestCollectUniverse_EnableCronAloneDoesNotFeedTradePlan(t *testing.T) {
	setupUniverseGateDB(t)
	observe := &models.StockStrategy{
		Name: "观察", QueryType: "eastmoney_nl", Enable: true, CronExpr: "0 35 9 * * 1-5",
	}
	seedStrategyRun(t, observe, "600036", "招商银行")

	uni := collectUniverse()
	require.Equal(t, models.CandidatePoolSourceFollow, uni.Source)
	require.Zero(t, uni.StrategyID)
	require.NotContains(t, uni.Message, "招商银行")
	for _, it := range uni.Items {
		require.NotEqual(t, "sh600036", it.StockCode)
	}
}

func TestCollectUniverse_FeedsTradePlanRequiredEvenIfCronOff(t *testing.T) {
	setupUniverseGateDB(t)
	observe := &models.StockStrategy{
		Name: "观察", QueryType: "eastmoney_nl", Enable: true, CronExpr: "0 35 9 * * 1-5",
	}
	paper := &models.StockStrategy{
		Name: "模拟宇宙", QueryType: "eastmoney_nl", Enable: false, FeedsTradePlan: true,
	}
	seedStrategyRun(t, observe, "600036", "招商银行")
	seedStrategyRun(t, paper, "000001", "平安银行")

	uni := collectUniverse()
	require.Equal(t, models.CandidatePoolSourceStrategyRun, uni.Source)
	require.Equal(t, paper.ID, uni.StrategyID)
	require.Equal(t, "模拟宇宙", uni.StrategyName)
	require.Len(t, uni.Items, 1)
	require.Equal(t, "sz000001", uni.Items[0].StockCode)
	require.Equal(t, "平安银行", uni.Items[0].StockName)
}
