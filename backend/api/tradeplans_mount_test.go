package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestTradePlansRoute_RegisteredInMain(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	mainPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "main.go"))
	b, err := os.ReadFile(mainPath)
	require.NoError(t, err, mainPath)
	body := string(b)
	require.Contains(t, body, "TradePlansAssetMiddleware",
		"main.go ChainAssetMiddleware 必须注册 TradePlansAssetMiddleware")
	require.Contains(t, body, "ChainAssetMiddleware",
		"main.go 必须使用 ChainAssetMiddleware 挂载 AssetServer API")
}

func TestTradePlansRoute_ChainGET_Frozen(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	freezeAt := now.Add(-time.Hour)
	today := "2026-07-22"

	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: today, GeneratedAt: now, PoolID: 1,
		Status: models.TradePlanStatusDraft, PlanVersion: 9,
		SourceSession: models.TradePlanSourceAfterClose,
	}, nil))
	frozen := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 2,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freezeAt, FreezeBy: "ops", FreezeReason: "eod",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:   risk.PlanRiskStatusPassed,
		ApprovedAt:   &now, ApprovedBy: "approver",
	}
	require.NoError(t, repo.CreatePlanWithItems(frozen, []models.TradePlanItem{
		{StockCode: "sh600519", Side: "buy", Priority: 1, TargetAmount: 20000, Status: models.TradePlanItemPending},
	}))

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	h := api.ChainAssetMiddleware(
		api.CandidatePoolAssetMiddleware,
		api.RealOrdersAssetMiddleware,
		api.TradePlansAssetMiddleware,
	)(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, nextCalled)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.NotNil(t, resp.Plan)
	require.Equal(t, frozen.ID, resp.Plan.ID)
	require.True(t, resp.Plan.Freeze.IsFrozen)
}

func TestTradePlansRoute_ChainGET_DraftFallback(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	today := "2026-07-22"

	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: today, GeneratedAt: now, PoolID: 3,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		SourceSession: models.TradePlanSourceMorningRebuild,
	}, nil))
	draft := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 4,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPartial,
	}
	require.NoError(t, repo.CreatePlanWithItems(draft, []models.TradePlanItem{
		{StockCode: "sz001309", Side: "buy", Priority: 1, TargetAmount: 10000, Status: models.TradePlanItemPending},
	}))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := api.ChainAssetMiddleware(
		api.RealOrdersAssetMiddleware,
		api.TradePlansAssetMiddleware,
	)(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.NotNil(t, resp.Plan)
	require.Equal(t, draft.ID, resp.Plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, resp.Plan.Status)
	require.False(t, resp.Plan.Freeze.IsFrozen)
}

func TestTradePlansRoute_ChainGET_NoPlan(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	h := api.ChainAssetMiddleware(
		api.CandidatePoolAssetMiddleware,
		api.TradePlansAssetMiddleware,
	)(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date=2099-01-01", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, nextCalled)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNoUpcoming, resp.Code)
	require.False(t, resp.OK)
	require.Nil(t, resp.Plan)
}
