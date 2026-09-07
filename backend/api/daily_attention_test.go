package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"

	"github.com/stretchr/testify/require"
)

func TestDailyAttentionAPI_OK(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterDailyAttentionRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/daily-attention?trade_date=2099-01-02", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, float64(0), body["code"])
	require.Equal(t, true, body["ok"])
	attn, ok := body["attention"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, attn, "overall_action")
	require.Contains(t, attn, "quality")
	require.Contains(t, attn, "items")
	action := attn["overall_action"].(string)
	require.Contains(t, []string{"HOLD", "WATCH", "REVIEW"}, action)
}

func TestDailyAttentionAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterDailyAttentionRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/investment/daily-attention", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestDailyAttentionAPI_ViaInvestmentMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.DailyInvestmentSummaryAssetMiddleware(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/daily-attention?trade_date=2099-01-02", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])
}
