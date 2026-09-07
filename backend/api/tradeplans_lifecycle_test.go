package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestTradePlanLifecycleAPI(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	now := time.Now()
	freeze := now.Add(-time.Hour)
	plan := &models.TradePlan{
		TradeDate: "2026-08-09", GeneratedAt: now, Status: models.TradePlanStatusReady,
		PlanVersion: 1, ApprovedAt: &now, FreezeAt: &freeze,
	}
	require.NoError(t, db.Dao.Create(plan).Error)

	mux := http.NewServeMux()
	api.RegisterTradePlansRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/"+strconv.FormatUint(uint64(plan.ID), 10)+"/lifecycle", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	lc := body["lifecycle"].(map[string]any)
	require.Equal(t, "FROZEN", lc["display_status"])
	require.Equal(t, float64(plan.ID), lc["plan_id"])
	require.NotNil(t, lc["approved_at"])
	require.NotNil(t, lc["freeze_at"])
}

func TestExecutionSummaryAndRiskAPI(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1e6})
	t.Cleanup(papertrading.ResetConfigCache)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, db.Dao.Create(acc).Error)

	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/execution-summary", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var sumBody map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &sumBody))
	require.True(t, sumBody["ok"].(bool))
	sum := sumBody["execution_summary"].(map[string]any)
	require.Nil(t, sum["avg_slippage"])

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/risk", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var riskBody map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &riskBody))
	require.True(t, riskBody["ok"].(bool))
	risk := riskBody["risk"].(map[string]any)
	require.Equal(t, "OK", risk["quality"])
}
