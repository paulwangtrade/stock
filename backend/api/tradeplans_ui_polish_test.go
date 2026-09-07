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
	"go-stock/backend/tradingcalendar"

	"github.com/stretchr/testify/require"
)

func TestTradePlansUpcoming_EnrichsEmptyStockNames(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	today := "2026-07-26"
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-27", GeneratedAt: time.Now(), PoolID: 1,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
	}, []models.TradePlanItem{
		{StockCode: "sz001309", StockName: "", Side: "buy", Priority: 1, TargetAmount: 10000, Status: models.TradePlanItemPending},
		{StockCode: "sh600519", StockName: "已有名", Side: "buy", Priority: 2, TargetAmount: 10000, Status: models.TradePlanItemPending},
	}))

	h := api.NewTradePlansHandler().WithStockNameLookup(func(codes []string) map[string]string {
		return map[string]string{
			"sz001309": "德明利",
			"sh600519": "不应覆盖",
		}
	})
	mux := http.NewServeMux()
	api.RegisterTradePlansHandler(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.NotNil(t, resp.Plan)
	require.Len(t, resp.Plan.Items, 2)
	require.Equal(t, "德明利", resp.Plan.Items[0].StockName)
	require.Equal(t, "已有名", resp.Plan.Items[1].StockName)
}

func TestTradePlansUpcoming_NameLookupEmptyDoesNotFail(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	today := "2026-07-26"
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-27", GeneratedAt: time.Now(), PoolID: 2,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
	}, []models.TradePlanItem{
		{StockCode: "sz001309", StockName: "", Side: "buy", Priority: 1, TargetAmount: 10000, Status: models.TradePlanItemPending},
	}))

	h := api.NewTradePlansHandler().WithStockNameLookup(func([]string) map[string]string {
		return map[string]string{}
	})
	mux := http.NewServeMux()
	api.RegisterTradePlansHandler(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.NotNil(t, resp.Plan)
	require.Equal(t, "", resp.Plan.Items[0].StockName)
}

func TestTradePlansUpcoming_NextTradingDayWeekend(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	want, err := tradingcalendar.NextTradingDayString("2026-07-26")
	require.NoError(t, err)
	require.Equal(t, "2026-07-27", want)

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date=2026-07-26", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNoUpcoming, resp.Code)
	require.Equal(t, "2026-07-27", resp.NextTradingDay)
}

func TestTradePlansUpcoming_NextTradingDayOnOK(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	today := "2026-07-26"
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-27", GeneratedAt: time.Now(), PoolID: 3,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
	}, nil))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.Equal(t, "2026-07-27", resp.NextTradingDay)
}
