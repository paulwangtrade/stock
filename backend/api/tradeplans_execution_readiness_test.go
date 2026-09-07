package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"go-stock/backend/api"
	tpreadiness "go-stock/backend/tradingplan/readiness"

	"github.com/stretchr/testify/require"
)

func TestTradePlanExecutionReadinessAPI(t *testing.T) {
	planID := uint(37)
	mock := func(id uint) (tpreadiness.ExecutionReadiness, error) {
		require.Equal(t, planID, id)
		return tpreadiness.ExecutionReadiness{
			PlanID:        int(id),
			Status:        tpreadiness.StatusReady,
			CashEnough:    true,
			AvailableCash: 545_329,
			RequiredCash:  200_000,
			TotalEquity:   2_043_101,
			Concentration: tpreadiness.ConcentrationNormal,
			ExistingPositions: []tpreadiness.PositionConflict{},
			Issues:            []tpreadiness.ReadinessIssue{},
			AfterExecution:    []tpreadiness.PositionWeightProjection{},
		}, nil
	}

	h := api.NewTradePlansHandler().WithExecutionReadinessEval(mock)

	mux := http.NewServeMux()
	mux.Handle("/api/tradeplans/", h)

	rec := httptest.NewRecorder()
	url := "/api/tradeplans/" + strconv.FormatUint(uint64(planID), 10) + "/execution-readiness"
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	require.Equal(t, float64(0), body["code"])
	require.Equal(t, "READY", body["status"])
	require.Equal(t, true, body["cash_enough"])
	require.Equal(t, float64(545329), body["available_cash"])
	require.Equal(t, float64(200000), body["required_cash"])
	require.Equal(t, "NORMAL", body["concentration"])
}

func TestTradePlanExecutionReadinessAPI_NotFound(t *testing.T) {
	mock := func(id uint) (tpreadiness.ExecutionReadiness, error) {
		return tpreadiness.ExecutionReadiness{}, fmt.Errorf("plan not found")
	}
	h := api.NewTradePlansHandler().WithExecutionReadinessEval(mock)

	mux := http.NewServeMux()
	mux.Handle("/api/tradeplans/", h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/999/execution-readiness", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}
