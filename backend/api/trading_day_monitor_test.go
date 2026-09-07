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

func TestTradingDayMonitorAPI_NormalDay(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2026-08-17"
	now := time.Date(2026, 8, 17, 15, 5, 0, 0, time.Local)
	tradingevent.EmitAutomationStep(td, now, "materialize", "MATERIALIZATION_AUTO_SUCCESS", "", 1, true)
	tradingevent.EmitAutomationStep(td, now, "approve", "AUTO_APPROVAL_SUCCESS", "", 1, true)
	tradingevent.EmitAutomationStep(td, now, "freeze", "AUTO_FREEZE_SUCCESS", "", 1, true)
	tradingevent.EmitExecution(tradingevent.EventExecutionCompleted, td, "A", tradingevent.StatusPass, "ok", 1, now, "e1")
	tradingevent.EmitSettlement(tradingevent.EventSettlementCompleted, td, tradingevent.StatusPass, "SETTLEMENT_OK", 1, now)

	mux := http.NewServeMux()
	api.RegisterTradingDayMonitorRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/trading/day-monitor?trade_date="+td, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	mon := body["monitor"].(map[string]any)
	require.Equal(t, td, mon["trade_date"])
	morning := mon["morning"].(map[string]any)
	require.Equal(t, "PASS", morning["materialize"].(map[string]any)["status"])
	require.Equal(t, "PASS", morning["approve"].(map[string]any)["status"])
	require.Equal(t, "PASS", morning["freeze"].(map[string]any)["status"])
	exec := mon["execution"].(map[string]any)
	require.Equal(t, "PASS", exec["status"])
	require.Equal(t, "A", exec["session"])
	settle := mon["settlement"].(map[string]any)
	require.Equal(t, "PASS", settle["status"])
}

func TestTradingDayMonitorAPI_NoPlan(t *testing.T) {
	tradingevent.ResetBufferForTest()
	mux := http.NewServeMux()
	api.RegisterTradingDayMonitorRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/trading/day-monitor?trade_date=2099-01-01", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	mon := body["monitor"].(map[string]any)
	morning := mon["morning"].(map[string]any)
	// No events; readiness aux may UNKNOWN or PENDING depending on DB
	mat := morning["materialize"].(map[string]any)
	require.Contains(t, []any{"UNKNOWN", "PENDING"}, mat["status"])
}

func TestTradingDayMonitorAPI_ExecutionFailed(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2026-08-18"
	now := time.Now()
	tradingevent.EmitExecution(tradingevent.EventExecutionFailed, td, "A", tradingevent.StatusFail, "fail_reason", 9, now, "")

	mux := http.NewServeMux()
	api.RegisterTradingDayMonitorRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/trading/day-monitor?trade_date="+td, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	exec := body["monitor"].(map[string]any)["execution"].(map[string]any)
	require.Equal(t, "FAIL", exec["status"])
	require.Equal(t, "fail_reason", exec["reason"])
	require.Equal(t, float64(9), exec["plan_id"])
}

func TestTradingDayMonitorAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterTradingDayMonitorRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/trading/day-monitor", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
