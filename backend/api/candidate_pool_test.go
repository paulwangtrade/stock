package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestCandidatePoolAPI_ListEmpty(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Empty(t, resp.Data)
	require.Equal(t, 0, resp.Total)
	require.Equal(t, 60.0, resp.Threshold)
}

func TestCandidatePoolAPI_ListFiltersAndSorts(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	days0 := 0
	created := time.Date(2026, 7, 19, 15, 0, 0, 0, time.UTC)
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "600036.SH", SECURITY_CODE: "600036", SECURITY_NAME_ABBR: "招商银行", Tag: "强", DaysAgo: &days0, RSI: 28, StatusText: "今日强化买点", NEW_PRICE: "40.1"},
			{SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安银行", Tag: "冰", DaysAgo: &days0, RSI: 40, StatusText: "冰点区内", NEW_PRICE: "11"},
			{SECUCODE: "300001.SZ", SECURITY_CODE: "300001", SECURITY_NAME_ABBR: "特锐德", Tag: "趋", DaysAgo: &days0, RSI: 32, StatusText: "趋势买点", NEW_PRICE: "12"},
		},
		HitTotal: 3,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  created,
		TradeDate:  "2099-04-01",
		Session:    "close",
		Status:     "done",
		HitTotal:   3,
		ResultJSON: string(raw),
	}).Error)

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.GreaterOrEqual(t, resp.Total, 2)
	require.Len(t, resp.Data, resp.Total)
	require.Equal(t, "sh600036", resp.Data[0].StockCode)
	require.Equal(t, "招商银行", resp.Data[0].StockName)
	require.Equal(t, "买入", resp.Data[0].Direction)
	require.Equal(t, "今日强化买点", resp.Data[0].Reason)
	require.Equal(t, created.UTC().Format(time.RFC3339), resp.SnapshotTime)
	require.Equal(t, 60.0, resp.Threshold)
	for i := 1; i < len(resp.Data); i++ {
		require.GreaterOrEqual(t, resp.Data[i-1].SignalScore, resp.Data[i].SignalScore)
	}
	for _, it := range resp.Data {
		require.Greater(t, it.SignalScore, 60.0)
	}
}

func TestCandidatePoolAPI_ThresholdConfig(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	require.NoError(t, db.Dao.Create(&data.Settings{CandidatePoolScoreThreshold: 90}).Error)
	data.ResetSettingCacheForTest()
	require.Equal(t, 90.0, data.GetCandidatePoolScoreThreshold())

	days0 := 0
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "600036.SH", SECURITY_CODE: "600036", SECURITY_NAME_ABBR: "招商银行", Tag: "强", DaysAgo: &days0, RSI: 28, StatusText: "强", NEW_PRICE: "40"},
			{SECUCODE: "300001.SZ", SECURITY_CODE: "300001", SECURITY_NAME_ABBR: "特锐德", Tag: "趋", DaysAgo: &days0, RSI: 32, StatusText: "趋", NEW_PRICE: "12"},
		},
		HitTotal: 2,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-04-02",
		Session:    "close",
		Status:     "done",
		HitTotal:   2,
		ResultJSON: string(raw),
	}).Error)

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 90.0, resp.Threshold)
	require.Equal(t, 1, resp.Total)
	require.Equal(t, "sh600036", resp.Data[0].StockCode)
	require.Greater(t, resp.Data[0].SignalScore, 90.0)
}

func TestCandidatePoolAPI_FlexibleResultsFormat(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	raw := `{"results":[{"code":"600036","name":"招商银行","score":82.3,"direction":"买入","reason":"均线多头排列"},{"code":"000001","name":"平安","score":50,"direction":"中性","reason":"低分"}]}`
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-04-03",
		Session:    "close",
		Status:     "done",
		HitTotal:   2,
		ResultJSON: raw,
	}).Error)

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Total)
	require.Equal(t, "sh600036", resp.Data[0].StockCode)
	require.InDelta(t, 82.3, resp.Data[0].SignalScore, 0.01)
	require.Equal(t, "买入", resp.Data[0].Direction)
}

func TestCandidatePoolAPI_ResponseShapeKeys(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	for _, key := range []string{"code", "data", "total", "snapshot_time", "snapshot_id", "threshold"} {
		_, ok := raw[key]
		require.True(t, ok, "missing response key %q", key)
	}
	// 不得回退为顶层数组
	require.True(t, len(rec.Body.Bytes()) > 0 && rec.Body.Bytes()[0] == '{')
}

func TestCandidatePoolAPI_MinScoreQueryOverridesThreshold(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	require.NoError(t, db.Dao.Create(&data.Settings{CandidatePoolScoreThreshold: 60}).Error)
	data.ResetSettingCacheForTest()

	days0 := 0
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "600036.SH", SECURITY_CODE: "600036", SECURITY_NAME_ABBR: "招商银行", Tag: "强", DaysAgo: &days0, RSI: 28, StatusText: "强", NEW_PRICE: "40"},
			{SECUCODE: "300001.SZ", SECURITY_CODE: "300001", SECURITY_NAME_ABBR: "特锐德", Tag: "趋", DaysAgo: &days0, RSI: 32, StatusText: "趋", NEW_PRICE: "12"},
		},
		HitTotal: 2,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-04-04",
		Session:    "close",
		Status:     "done",
		HitTotal:   2,
		ResultJSON: string(raw),
	}).Error)

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list?min_score=95", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 95.0, resp.Threshold, "min_score 应覆盖配置阈值并回写到 threshold")
	require.Equal(t, 1, resp.Total)
	require.Equal(t, "sh600036", resp.Data[0].StockCode)
	require.Greater(t, resp.Data[0].SignalScore, 95.0)
}

func TestCandidatePoolAPI_TagFallbackWhenScoreMissing(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	// results[] 无 score，仅有 tag → 走既有 CalcResearchSignalScore，不新增算法
	raw := `{"results":[{"code":"600036","name":"招商银行","tag":"强","rsi":28,"reason":"强化买点"},{"code":"000001","name":"平安","tag":"冰","rsi":40,"reason":"冰点"}]}`
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-04-05",
		Session:    "close",
		Status:     "done",
		HitTotal:   2,
		ResultJSON: raw,
	}).Error)

	expectedStrong := data.CalcResearchSignalScore("强", nil, 28)
	require.Greater(t, expectedStrong, 60.0)

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Total)
	require.Equal(t, "sh600036", resp.Data[0].StockCode)
	require.InDelta(t, expectedStrong, resp.Data[0].SignalScore, 0.01)
	require.Equal(t, "买入", resp.Data[0].Direction)
}

func TestCandidatePoolAPI_TopLevelArrayAndMapFormats(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	days0 := 0
	arrPayload, err := json.Marshal([]models.SignalScanHit{
		{SECUCODE: "600036.SH", SECURITY_CODE: "600036", SECURITY_NAME_ABBR: "招商银行", Tag: "强", DaysAgo: &days0, RSI: 28, StatusText: "array", NEW_PRICE: "40"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-04-06",
		Session:    "close",
		Status:     "done",
		HitTotal:   1,
		ResultJSON: string(arrPayload),
	}).Error)

	mux := http.NewServeMux()
	api.RegisterCandidatePoolRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Total)
	require.Equal(t, "sh600036", resp.Data[0].StockCode)

	// 覆盖为 map 格式（更新为更新的 done 快照）
	mapRaw := `{"600519":{"name":"贵州茅台","score":88.5,"direction":"买入","reason":"map格式"}}`
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now().Add(time.Second),
		TradeDate:  "2099-04-07",
		Session:    "close",
		Status:     "done",
		HitTotal:   1,
		ResultJSON: mapRaw,
	}).Error)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var resp2 api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	require.Equal(t, 1, resp2.Total)
	require.Equal(t, "sh600519", resp2.Data[0].StockCode)
	require.InDelta(t, 88.5, resp2.Data[0].SignalScore, 0.01)
}

func TestCandidatePoolAssetMiddleware(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	h := api.ChainAssetMiddleware(api.CandidatePoolAssetMiddleware, api.RealOrdersAssetMiddleware)(next)

	req := httptest.NewRequest(http.MethodGet, "/api/candidate_pool/list", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, nextCalled)

	var resp api.CandidatePoolListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Empty(t, resp.Data)
}
