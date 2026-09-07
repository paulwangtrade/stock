package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"

	"github.com/stretchr/testify/require"
)

func TestDecisionSummaryAPI_OK(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterDecisionSummaryRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/decision-summary?trade_date=2099-01-02", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	sum := body["summary"].(map[string]any)
	require.Contains(t, sum, "portfolio_health")
	require.Contains(t, sum, "opportunity_attention")
	require.Contains(t, sum, "capital_efficiency_attention")
	require.Contains(t, sum, "risk_attention")
	require.Contains(t, sum, "explanation")
	require.Contains(t, sum, "overall_attention")
	attn := sum["overall_attention"].(string)
	require.Contains(t, []string{"HOLD", "REVIEW", "WATCH"}, attn)
}

func TestDecisionSummaryAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterDecisionSummaryRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/portfolio/decision-summary", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPortfolioMiddleware_RoutesDecisionSummary(t *testing.T) {
	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.PortfolioDashboardAssetMiddleware(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/decision-summary", nil))
	require.False(t, nextHit)
	require.Equal(t, http.StatusOK, rec.Code)
}
