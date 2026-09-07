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
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestTradePlansSameDayCandidatesAPI_MultiOrigin(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	day := "2026-09-07"

	a := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		Side: "buy", SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus: risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(a, []models.TradePlanItem{{
		TradeDate: day, StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending,
	}}))
	b := &models.TradePlan{
		TradeDate: day, GeneratedAt: now,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		Side: "buy", SourceSession: models.TradePlanSourceWatchlist,
	}
	require.NoError(t, repo.CreatePlanWithItems(b, []models.TradePlanItem{{
		TradeDate: day, StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	mux := registerTradePlansTestMux(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tradeplans/same-day-candidates?trade_date="+day, nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp api.SameDayCandidatesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.Equal(t, day, resp.TradeDate)
	require.Equal(t, 2, resp.Count)
	require.Len(t, resp.Candidates, 2)

	sources := map[string]bool{}
	for _, c := range resp.Candidates {
		sources[c.Source] = true
		require.NotEmpty(t, c.SourceSession)
		require.NotZero(t, c.ID)
	}
	require.True(t, sources["strategy"])
	require.True(t, sources["watchlist"])

	// Upcoming still returns a single plan (highest draft version).
	upRec := httptest.NewRecorder()
	upReq := httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+day, nil)
	mux.ServeHTTP(upRec, upReq)
	require.Equal(t, http.StatusOK, upRec.Code)
	var up api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &up))
	require.True(t, up.OK)
	require.NotNil(t, up.Plan)
	require.Equal(t, b.ID, up.Plan.ID)
}
