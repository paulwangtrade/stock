package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/tradingevent"

	"github.com/stretchr/testify/require"
)

func TestDailyInvestmentSummaryAPI_OK(t *testing.T) {
	tradingevent.ResetBufferForTest()
	mux := http.NewServeMux()
	api.RegisterDailyInvestmentSummaryRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/daily-summary?trade_date=2099-01-02", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	sum := body["summary"].(map[string]any)
	require.Equal(t, "2099-01-02", sum["trade_date"])
	require.Contains(t, sum, "portfolio_summary")
	require.Contains(t, sum, "trading_summary")
	require.Contains(t, sum, "risk_summary")
	require.Contains(t, sum, "position_attention")
	require.Contains(t, sum, "tomorrow_focus")
}

func TestDailyInvestmentSummaryAPI_ExecutionFailedVisible(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2026-08-20"
	now := time.Now()
	tradingevent.EmitExecution(tradingevent.EventExecutionFailed, td, "A", tradingevent.StatusFail, "fail_x", 3, now, "")

	mux := http.NewServeMux()
	api.RegisterDailyInvestmentSummaryRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/daily-summary?trade_date="+td, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	ts := body["summary"].(map[string]any)["trading_summary"].(map[string]any)
	require.Equal(t, "FAIL", ts["execution_status"])
	require.Contains(t, ts["narrative"].(string), "fail_x")
}

func TestDailyInvestmentSummaryAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterDailyInvestmentSummaryRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/investment/daily-summary", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
