package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestPaperTradingDashboard_GETOnly(t *testing.T) {
	setupPaperAPITestDB(t)

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: now, Status: models.TradePlanStatusReady,
		FreezeAt: &now, FreezeBy: "test", PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", StockName: "平安银行", Side: "buy", Status: models.TradePlanItemPending, TargetVolume: 1000, LimitPrice: 10},
	}))
	papertrading.SetQuoteFetcherForTest(func(codes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{Code: codes[0], Open: "10.05", Price: "10.10"}}, nil
	})
	t.Cleanup(func() { papertrading.SetQuoteFetcherForTest(nil) })

	_, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerManual, Actor: "dev",
		SkipWeekdayCheck: true,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	api.RegisterPaperTradingRoutes(mux)

	// POST on dashboard must be rejected
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/dashboard/today", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/dashboard/today?trade_date=2026-07-30", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var todayBody map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &todayBody))
	require.Equal(t, true, todayBody["ok"])
	today := todayBody["today"].(map[string]any)
	require.Equal(t, true, today["enabled"])
	require.EqualValues(t, 1, today["filledCount"])

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/dashboard/positions", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var posBody map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &posBody))
	require.Equal(t, true, posBody["ok"])

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/dashboard/runs?trade_date=2026-07-30", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var runsBody map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &runsBody))
	require.Equal(t, true, runsBody["ok"])
}
