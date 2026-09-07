package data

import (
	"fmt"
	"sync"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSentimentTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:sentiment_lazy_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, testDB.AutoMigrate(&StockBasic{}, &models.StockInfoHK{}, &models.Tags{}))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
		ResetSentimentDictForTest()
	})
	ResetSentimentDictForTest()
}

func TestAnalyzeSentiment_LazyLoadWithoutExplicitInit(t *testing.T) {
	setupSentimentTestDB(t)
	require.NoError(t, db.Dao.Create(&StockBasic{TsCode: "000001.SZ", Name: "平安银行", BKName: "银行"}).Error)

	// 不调用 InitAnalyzeSentiment，首次 AnalyzeSentiment 应懒加载
	result := AnalyzeSentiment("平安银行大涨利好")
	require.NotEmpty(t, result.Description)
	require.True(t, SentimentDictLoadedForTest())

	// 第二次应走缓存
	result2 := AnalyzeSentiment("市场下跌利空")
	require.NotEmpty(t, result2.Description)
}

func TestAnalyzeSentiment_EmptyDBNoError(t *testing.T) {
	setupSentimentTestDB(t)
	// 空表：仅内置词典，不应 panic
	result := AnalyzeSentiment("股价上涨")
	require.NotEmpty(t, result.Description)
}

func TestAnalyzeSentiment_NilDBNoError(t *testing.T) {
	original := db.Dao
	db.Dao = nil
	ResetSentimentDictForTest()
	t.Cleanup(func() {
		db.Dao = original
		ResetSentimentDictForTest()
	})

	result := AnalyzeSentiment("利好消息")
	require.NotEmpty(t, result.Description)
}

func TestAnalyzeSentiment_ConcurrentLazyLoad(t *testing.T) {
	setupSentimentTestDB(t)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = AnalyzeSentiment("上涨下跌利好利空")
		}()
	}
	wg.Wait()
	require.True(t, SentimentDictLoadedForTest())
}

func TestInitAnalyzeSentiment_Idempotent(t *testing.T) {
	setupSentimentTestDB(t)
	InitAnalyzeSentiment()
	require.True(t, SentimentDictLoadedForTest())
	InitAnalyzeSentiment() // 再次调用应直接返回
	result := AnalyzeSentiment("中性消息")
	require.NotEmpty(t, result.Description)
}

// SentimentDictLoadedForTest 暴露懒加载标志供测试断言。
func SentimentDictLoadedForTest() bool {
	sentimentDictMu.Lock()
	defer sentimentDictMu.Unlock()
	return sentimentDictLoaded
}
