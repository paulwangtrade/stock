package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/obsfeedback"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func getPerformance(t *testing.T, query string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	path := "/api/papertrading/observation/performance"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestObservationPerformanceAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/performance", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestObservationPerformanceAPI_EmptyOK(t *testing.T) {
	restore := obsfeedback.SetStoreForTest(obsfeedback.NewMemoryStore())
	t.Cleanup(restore)
	code, body := getPerformance(t, "horizon=5&benchmark=csi300")
	require.Equal(t, http.StatusOK, code)
	require.True(t, body["ok"].(bool))
	require.Contains(t, body["disclaimer"].(string), "不代表未来收益")
	p := body["performance"].(map[string]any)
	require.Equal(t, float64(5), p["horizon"])
	require.Equal(t, "csi300", p["benchmark"])
	require.Equal(t, "none", p["action"])
	require.Equal(t, float64(0), p["samples"])
	require.Equal(t, float64(0), p["evaluated"])
	require.Nil(t, p["win_rate"])
	by := p["by_state"].(map[string]any)
	require.Contains(t, by, "HOLD_NORMAL")
	require.Contains(t, by, "HOLD_WATCH")
	require.Contains(t, by, "HOLD_REVIEW")
}

func TestObservationPerformanceAPI_EvaluatesAndNoLedgerMutation(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-03", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	px := 10.0
	store := obsfeedback.NewMemoryStore()
	require.NoError(t, store.Save([]obsfeedback.Record{{
		ObservationID:   "obs-2026-08-03-sz000001",
		Symbol:          "sz000001",
		ObservationDate: "2026-08-03",
		DecisionState:   obsfeedback.StateHoldNormal,
		MarketPrice:     &px,
		Source:          obsfeedback.SourcePortfolioObservation,
	}}))
	restore := obsfeedback.SetStoreForTest(store)
	t.Cleanup(restore)

	api.SetPerformanceSeriesProviderForTest(obsfeedback.MapSeriesProvider{Bars: map[string][]obsfeedback.Bar{
		"sz000001": {
			{Date: "2026-08-04", Close: 10.2, High: 10.3, Low: 10.1},
			{Date: "2026-08-05", Close: 10.4, High: 10.5, Low: 10.2},
			{Date: "2026-08-06", Close: 10.5, High: 10.6, Low: 10.3},
			{Date: "2026-08-07", Close: 10.6, High: 10.7, Low: 10.4},
			{Date: "2026-08-10", Close: 10.8, High: 11.0, Low: 10.4},
		},
		"000300.SH": {
			{Date: "2026-08-03", Close: 100},
			{Date: "2026-08-04", Close: 101},
			{Date: "2026-08-05", Close: 102},
			{Date: "2026-08-06", Close: 102},
			{Date: "2026-08-07", Close: 103},
			{Date: "2026-08-10", Close: 103},
		},
	}})
	t.Cleanup(func() { api.SetPerformanceSeriesProviderForTest(nil) })

	code, body := getPerformance(t, "horizon=5&benchmark=csi300")
	require.Equal(t, http.StatusOK, code)
	p := body["performance"].(map[string]any)
	require.Equal(t, float64(1), p["samples"])
	require.Equal(t, float64(1), p["evaluated"])
	require.InDelta(t, 1.0, p["win_rate"].(float64), 1e-9)
	require.InDelta(t, 0.08, p["avg_return"].(float64), 1e-9)
	require.InDelta(t, 0.05, p["avg_alpha"].(float64), 1e-9)
	normal := p["by_state"].(map[string]any)["HOLD_NORMAL"].(map[string]any)
	require.Equal(t, float64(1), normal["evaluated"])
	require.InDelta(t, 1.0, normal["win_rate"].(float64), 1e-9)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.00, pos.MarkPrice, 1e-9)
	require.Equal(t, int64(1000), pos.TotalVolume)
	var acc2 papertrading.PaperSimAccount
	require.NoError(t, db.Dao.First(&acc2, acc.ID).Error)
	require.InDelta(t, acc.Cash, acc2.Cash, 1e-6)
	require.InDelta(t, acc.Equity, acc2.Equity, 1e-6)
}

func TestObservationPerformanceAPI_MissingBenchmarkStillOK(t *testing.T) {
	px := 10.0
	store := obsfeedback.NewMemoryStore()
	require.NoError(t, store.Save([]obsfeedback.Record{{
		ObservationID:   "obs-2026-08-03-sz000001",
		Symbol:          "sz000001",
		ObservationDate: "2026-08-03",
		DecisionState:   obsfeedback.StateHoldWatch,
		MarketPrice:     &px,
	}}))
	restore := obsfeedback.SetStoreForTest(store)
	t.Cleanup(restore)
	api.SetPerformanceSeriesProviderForTest(obsfeedback.MapSeriesProvider{Bars: map[string][]obsfeedback.Bar{
		"sz000001": {
			{Date: "2026-08-04", Close: 9.7},
			{Date: "2026-08-05", Close: 9.6},
			{Date: "2026-08-06", Close: 9.5},
			{Date: "2026-08-07", Close: 9.4},
			{Date: "2026-08-10", Close: 9.3},
		},
	}})
	t.Cleanup(func() { api.SetPerformanceSeriesProviderForTest(nil) })

	code, body := getPerformance(t, "horizon=5&benchmark=csi300")
	require.Equal(t, http.StatusOK, code)
	p := body["performance"].(map[string]any)
	require.Equal(t, float64(1), p["evaluated"])
	require.InDelta(t, -0.07, p["avg_return"].(float64), 1e-9)
	require.Nil(t, p["avg_alpha"])
	watch := p["by_state"].(map[string]any)["HOLD_WATCH"].(map[string]any)
	require.InDelta(t, 1.0, watch["risk_capture_rate"].(float64), 1e-9)
}

func TestObservationPerformanceAPI_MissingFutureNotWrongEval(t *testing.T) {
	px := 10.0
	store := obsfeedback.NewMemoryStore()
	require.NoError(t, store.Save([]obsfeedback.Record{{
		ObservationID:   "obs-2026-08-03-sz000001",
		Symbol:          "sz000001",
		ObservationDate: "2026-08-03",
		DecisionState:   obsfeedback.StateHoldNormal,
		MarketPrice:     &px,
	}}))
	restore := obsfeedback.SetStoreForTest(store)
	t.Cleanup(restore)
	api.SetPerformanceSeriesProviderForTest(obsfeedback.MapSeriesProvider{Bars: map[string][]obsfeedback.Bar{
		"sz000001": {{Date: "2026-08-04", Close: 11}},
	}})
	t.Cleanup(func() { api.SetPerformanceSeriesProviderForTest(nil) })

	code, body := getPerformance(t, "horizon=5")
	require.Equal(t, http.StatusOK, code)
	p := body["performance"].(map[string]any)
	require.Equal(t, float64(1), p["samples"])
	require.Equal(t, float64(0), p["evaluated"])
	require.Nil(t, p["win_rate"])
	normal := p["by_state"].(map[string]any)["HOLD_NORMAL"].(map[string]any)
	require.Greater(t, normal["pending"].(float64)+normal["insufficient"].(float64), float64(0))
}
