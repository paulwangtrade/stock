package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func registerTradePlansTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterTradePlansRoutes(mux)
	return mux
}

func TestTradePlansAPI_FrozenReturned(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	freezeAt := now.Add(-time.Hour)
	today := "2026-07-22"

	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-22", GeneratedAt: now, PoolID: 1,
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
		{StockCode: "sh600519", StockName: "茅台", Side: "buy", Priority: 1, TargetAmount: 20000, Status: models.TradePlanItemPending},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.Equal(t, today, resp.TradeDate)
	require.NotNil(t, resp.Plan)
	require.Equal(t, frozen.ID, resp.Plan.ID)
	require.Equal(t, "2026-07-23", resp.Plan.TradeDate)
	require.Equal(t, models.TradePlanStatusReady, resp.Plan.Status)
	require.True(t, resp.Plan.Freeze.IsFrozen)
	require.Equal(t, "ops", resp.Plan.Freeze.FreezeBy)
	require.True(t, resp.Plan.Risk.Passed)
	require.Len(t, resp.Plan.Items, 1)
	require.Equal(t, "sh600519", resp.Plan.Items[0].StockCode)
}

func TestTradePlansAPI_DraftFallback(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	today := "2026-07-22"

	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-22", GeneratedAt: now, PoolID: 3,
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
		{StockCode: "sz001309", Side: "buy", Priority: 1, TargetAmount: 10000,
			Status: models.TradePlanItemSkipped, RiskCode: "LIMIT_UP", RiskMessage: "涨停"},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.NotNil(t, resp.Plan)
	require.Equal(t, draft.ID, resp.Plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, resp.Plan.Status)
	require.False(t, resp.Plan.Freeze.IsFrozen)
	require.False(t, resp.Plan.Risk.Passed)
	require.Contains(t, resp.Plan.Risk.Reasons, "LIMIT_UP: 涨停")
}

func TestTradePlansAPI_NoPlanBusinessCode(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-07-20", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
	}, nil))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date=2026-07-22", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNoUpcoming, resp.Code)
	require.False(t, resp.OK)
	require.Nil(t, resp.Plan)
	require.Equal(t, "2026-07-22", resp.TradeDate)
	require.Equal(t, "no upcoming trade plan", resp.Message)
}

func TestTradePlansAPI_JSONSchemaSnakeCase(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	freezeAt := now
	plan := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 7,
		Status: models.TradePlanStatusReady, PlanVersion: 2,
		FreezeAt: &freezeAt, FreezeBy: "ops",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:   risk.PlanRiskStatusPassed,
		ApprovedAt:   &now, ApprovedBy: "alice",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sh600000", StockName: "浦发", Side: "buy", Priority: 1, TargetAmount: 8000, Status: models.TradePlanItemPending, StrategyName: "s1"},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date=2026-07-22", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.Bytes()
	raw := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(body, &raw))
	require.Contains(t, raw, "code")
	require.Contains(t, raw, "ok")
	require.Contains(t, raw, "trade_date")
	require.Contains(t, raw, "plan")
	require.NotContains(t, string(body), `"tradeDate"`)
	require.NotContains(t, string(body), `"planVersion"`)
	require.NotContains(t, string(body), `"sourceSession"`)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotNil(t, resp.Plan)
	require.NotEmpty(t, resp.Plan.TradeDate)
	require.GreaterOrEqual(t, resp.Plan.PlanVersion, 0)
	require.NotNil(t, resp.Plan.Risk.Reasons)
	require.NotEmpty(t, resp.Plan.Freeze.FreezeAt)
	require.NotEmpty(t, resp.Plan.Freeze.ApprovedAt)
	require.Len(t, resp.Plan.Items, 1)
	require.NotEmpty(t, resp.Plan.Items[0].StockCode)
}

func TestTradePlansAPI_ReadOnlyDoesNotMutateDB(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	freezeAt := now
	plan := &models.TradePlan{
		TradeDate: "2026-07-23", GeneratedAt: now, PoolID: 99,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freezeAt, FreezeBy: "ops", FreezeReason: "freeze",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:   risk.PlanRiskStatusPassed,
		Message:      "immutable",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Priority: 1, TargetAmount: 5000, Status: models.TradePlanItemPending},
	}))

	var before models.TradePlan
	require.NoError(t, db.Dao.First(&before, plan.ID).Error)
	var beforePlanCount, beforeItemCount int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&beforePlanCount).Error)
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Count(&beforeItemCount).Error)

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date=2026-07-22", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var after models.TradePlan
	require.NoError(t, db.Dao.First(&after, plan.ID).Error)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.Message, after.Message)
	require.Equal(t, before.UpdatedAt.UnixNano(), after.UpdatedAt.UnixNano())

	var afterPlanCount, afterItemCount int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&afterPlanCount).Error)
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Count(&afterItemCount).Error)
	require.Equal(t, beforePlanCount, afterPlanCount)
	require.Equal(t, beforeItemCount, afterItemCount)

	body := strings.TrimSpace(rec.Body.String())
	require.Contains(t, body, `"trade_date"`)
}

func TestTradePlansAPI_MethodNotAllowed(t *testing.T) {
	setupAPITestDB(t)
	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tradeplans/upcoming", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestTradePlansAssetMiddleware(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	h := api.ChainAssetMiddleware(api.TradePlansAssetMiddleware)(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date=2099-01-01", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, nextCalled)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNoUpcoming, resp.Code)
}
