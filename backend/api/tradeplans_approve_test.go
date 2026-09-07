package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/approvegate"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func registerApproveTestMux(t *testing.T, opts *approvegate.ApproveOptions) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	h := api.NewTradePlansHandler().WithApproveOptions(opts)
	api.RegisterTradePlansHandler(mux, h)
	return mux
}

func passRiskForAPI(plan *models.TradePlan) (*strategy.RiskProposalResult, error) {
	return &strategy.RiskProposalResult{
		TradePlanID: plan.ID, TradePlanVersion: plan.PlanVersion,
		Passed: true, CheckedAt: time.Now(),
	}, nil
}

func failRiskForAPI(plan *models.TradePlan) (*strategy.RiskProposalResult, error) {
	return &strategy.RiskProposalResult{
		TradePlanID: plan.ID, TradePlanVersion: plan.PlanVersion,
		Passed: false, RiskReasons: []string{"market blocked"}, CheckedAt: time.Now(),
	}, nil
}

func approveEligOpts(risk func(*models.TradePlan) (*strategy.RiskProposalResult, error)) *approvegate.ApproveOptions {
	return &approvegate.ApproveOptions{
		Eligibility: &approvegate.EligibilityOptions{
			EvaluateRisk: risk,
			ReadinessOpts: &readiness.Options{
				MarketData: qualitygate.MarketDataSnapshot{
					SkipGapEval: true,
					IndustryByCode: map[string]string{"sz000001": "银行"},
					NameByCode:     map[string]string{"sz000001": "平安银行"},
					AnchorPriceByCode: map[string]float64{"sz000001": 10},
				},
			},
		},
	}
}

func seedAPIMaterializedDraft(t *testing.T) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: "2026-07-31", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "morning_materialized",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-31", StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		IntentStatus: readiness.IntentPriced, LimitPrice: 10, TargetVolume: 10000,
	}}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func seedAPIUnmaterializedDraft(t *testing.T) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: "2026-07-31", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "after_close_intent",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-31", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		IntentStatus: readiness.IntentSelected, LimitPrice: 0, TargetVolume: 0,
	}}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func postApprove(t *testing.T, mux *http.ServeMux, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tradeplans/approve", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

func TestTradePlansApproveAPI_Success(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	rec := postApprove(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "source": "http_api",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.Equal(t, approvegate.CodeApproveWriteOK, resp.ResultCode)
	require.Equal(t, plan.ID, resp.PlanID)
	require.Equal(t, models.TradePlanStatusDraft, resp.Status)
	require.NotEmpty(t, resp.ApprovedAt)
	require.Equal(t, "alice", resp.ApprovedBy)
	require.Equal(t, "http_api", resp.ApprovedSource)
	require.Empty(t, resp.Blockers)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, "alice", got.ApprovedBy)
	require.Equal(t, "http_api", got.ApprovedSource)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
}

func TestTradePlansApproveAPI_RiskDeny(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	mux := registerApproveTestMux(t, approveEligOpts(failRiskForAPI))

	rec := postApprove(t, mux, map[string]any{"plan_id": plan.ID, "actor": "alice", "source": "http_api"})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeRiskDeny, resp.Code)
	require.False(t, resp.OK)
	require.Equal(t, approvegate.CodeDenied, resp.ResultCode)
	require.NotEmpty(t, resp.Blockers)
	require.Equal(t, approvegate.CodeRiskNotPassed, resp.Blockers[0].Code)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)
}

func TestTradePlansApproveAPI_ReadinessDeny(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIUnmaterializedDraft(t)
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	rec := postApprove(t, mux, map[string]any{"plan_id": plan.ID, "actor": "alice", "source": "http_api"})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeReadinessDeny, resp.Code)
	require.False(t, resp.OK)
	require.NotEmpty(t, resp.Blockers)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)
}

func TestTradePlansApproveAPI_DuplicateApprove(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	first := postApprove(t, mux, map[string]any{"plan_id": plan.ID, "actor": "alice", "source": "manual"})
	require.Equal(t, http.StatusOK, first.Code)
	var firstResp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstResp))
	require.True(t, firstResp.OK)
	firstAt := firstResp.ApprovedAt

	second := postApprove(t, mux, map[string]any{"plan_id": plan.ID, "actor": "bob", "source": "http_api"})
	require.Equal(t, http.StatusOK, second.Code)
	var resp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeAlreadyApproved, resp.Code)
	require.False(t, resp.OK)
	require.Equal(t, approvegate.CodeAlreadyApproved, resp.ResultCode)
	require.True(t, resp.AlreadyApproved)
	require.Equal(t, "alice", resp.ApprovedBy)
	require.Equal(t, "manual", resp.ApprovedSource)
	require.Equal(t, firstAt, resp.ApprovedAt)
}

func TestTradePlansApproveAPI_InvalidPlan(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	rec := postApprove(t, mux, map[string]any{"plan_id": 0, "actor": "alice"})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeInvalidPlanID, resp.Code)
	require.False(t, resp.OK)
	require.Equal(t, approvegate.CodeInvalidPlanID, resp.ResultCode)

	rec2 := postApprove(t, mux, map[string]any{"plan_id": 999999, "actor": "alice", "source": "http_api"})
	require.Equal(t, http.StatusOK, rec2.Code)
	var missing api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &missing))
	require.Equal(t, api.TradePlanCodeNoPlan, missing.Code)
	require.False(t, missing.OK)
	require.Equal(t, "PLAN_NOT_FOUND", missing.ResultCode)
}

func TestTradePlansApproveAPI_MethodNotAllowed(t *testing.T) {
	setupAPITestDB(t)
	mux := registerApproveTestMux(t, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/approve", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestTradePlansApproveAPI_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("tradeplans_approve.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"PromoteDraftToFrozen(",
		"FreezeTradePlan(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunDailyCandidateAndPlan(",
	} {
		require.NotContains(t, src, token)
	}
	require.Contains(t, src, "ApproveTradePlanByID")
}
