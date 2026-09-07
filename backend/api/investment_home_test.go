package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"

	"github.com/stretchr/testify/require"
)

func TestInvestmentHomeAPI_OK(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterInvestmentHomeRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/home?trade_date=2099-01-02", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	home := body["home"].(map[string]any)
	require.Equal(t, "2099-01-02", home["trade_date"])
	require.Contains(t, home, "portfolio_summary")
	require.Contains(t, home, "decision_summary")
	require.Contains(t, home, "daily_summary")
	require.Contains(t, home, "trading_status")
	require.Contains(t, home, "attention_items")
}

func TestInvestmentHomeAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterInvestmentHomeRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/investment/home", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestInvestmentMiddleware_RoutesHomeAndDaily(t *testing.T) {
	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.DailyInvestmentSummaryAssetMiddleware(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/home", nil))
	require.False(t, nextHit)
	require.Equal(t, http.StatusOK, rec.Code)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/investment/daily-summary", nil))
	require.Equal(t, http.StatusOK, rec2.Code)

	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/api/investment/daily-attention", nil))
	require.Equal(t, http.StatusOK, rec3.Code)
}
