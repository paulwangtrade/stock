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

func TestPaperTradingReportsDaily_GET(t *testing.T) {
	setupPaperAPITestDB(t)

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: now, Status: models.TradePlanStatusReady,
		FreezeAt: &now, FreezeBy: "test", ApprovedAt: &now, ApprovedBy: "test", PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
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
		Now:              time.Date(2026, 8, 5, 10, 0, 0, 0, time.Local),
	})
	require.NoError(t, err)
	_, err = papertrading.SettlementJob(plan.TradeDate, papertrading.DefaultOpenPriceProvider(), false)
	require.NoError(t, err)

	mux := http.NewServeMux()
	api.RegisterPaperTradingRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/reports/daily", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/reports/daily?from=2026-07-01&to=2026-07-31", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])
	require.EqualValues(t, 1, body["total"])
	reports := body["reports"].([]any)
	require.Len(t, reports, 1)
}
