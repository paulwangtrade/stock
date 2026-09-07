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
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestTradePlansAPI_PlanByID_ExactDraft(t *testing.T) {
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
		TradeDate: "2026-07-22", GeneratedAt: now, PoolID: 2,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		FreezeAt: &freezeAt, FreezeBy: "ops", FreezeReason: "eod",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:   risk.PlanRiskStatusPassed,
		ApprovedAt:   &now, ApprovedBy: "approver",
	}
	require.NoError(t, repo.CreatePlanWithItems(frozen, []models.TradePlanItem{
		{StockCode: "sh600519", Side: "buy", Priority: 1, TargetAmount: 20000, Status: models.TradePlanItemPending},
	}))
	newDraft := &models.TradePlan{
		TradeDate: "2026-07-22", GeneratedAt: now, PoolID: 3,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(newDraft, []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Priority: 1, TargetAmount: 10000, Status: models.TradePlanItemPending},
	}))

	mux := registerTradePlansTestMux(t)
	// Upcoming prefers frozen on same trade_date.
	upRec := httptest.NewRecorder()
	mux.ServeHTTP(upRec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, upRec.Code)
	var upResp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(upRec.Body.Bytes(), &upResp))
	require.Equal(t, frozen.ID, upResp.Plan.ID)

	planRec := httptest.NewRecorder()
	mux.ServeHTTP(planRec, httptest.NewRequest(
		http.MethodGet,
		"/api/tradeplans/plan?plan_id="+strconv.FormatUint(uint64(newDraft.ID), 10),
		nil,
	))
	require.Equal(t, http.StatusOK, planRec.Code)
	var planResp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(planRec.Body.Bytes(), &planResp))
	require.Equal(t, api.TradePlanCodeOK, planResp.Code)
	require.True(t, planResp.OK)
	require.Equal(t, newDraft.ID, planResp.PlanID)
	require.NotNil(t, planResp.Plan)
	require.Equal(t, newDraft.ID, planResp.Plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, planResp.Plan.Status)
	require.False(t, planResp.Plan.Freeze.IsFrozen)
	require.Equal(t, "sz000001", planResp.Plan.Items[0].StockCode)
}

func TestTradePlansAPI_PlanByID_InvalidAndMissing(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	mux := registerTradePlansTestMux(t)

	badRec := httptest.NewRecorder()
	mux.ServeHTTP(badRec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/plan?plan_id=0", nil))
	require.Equal(t, http.StatusBadRequest, badRec.Code)
	var badResp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(badRec.Body.Bytes(), &badResp))
	require.Equal(t, api.TradePlanCodeInvalidPlanID, badResp.Code)

	missRec := httptest.NewRecorder()
	mux.ServeHTTP(missRec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/plan?plan_id=999999", nil))
	require.Equal(t, http.StatusOK, missRec.Code)
	var missResp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(missRec.Body.Bytes(), &missResp))
	require.Equal(t, api.TradePlanCodeNoUpcoming, missResp.Code)
	require.False(t, missResp.OK)
}

func TestTradePlansAPI_PlanByID_SellItemReason(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	now := time.Now()
	sellReason := "exit_review:REVIEW_REQUIRED;LOSS_REVIEW;亏损关注"

	plan := &models.TradePlan{
		TradeDate: "2026-08-29", GeneratedAt: now, PoolID: 1,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		Side: "sell", SourceSession: models.TradePlanSourceExitReview,
		RiskStatus: risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{
			StockCode: "sz000001", StockName: "深科技", Side: "sell",
			Priority: 1, TargetVolume: 100, Status: models.TradePlanItemPending,
			Reason: sellReason,
		},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/api/tradeplans/plan?plan_id="+strconv.FormatUint(uint64(plan.ID), 10),
		nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.Len(t, resp.Plan.Items, 1)
	require.Equal(t, sellReason, resp.Plan.Items[0].Reason)
}
