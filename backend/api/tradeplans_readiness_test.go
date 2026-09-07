package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"

	"github.com/stretchr/testify/require"
)

func TestTradePlansReadinessAPI_NoPlan(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/readiness?trade_date=2099-01-01", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNoPlan, resp.Code)
	require.Equal(t, 40401, resp.Code)
	require.False(t, resp.OK)
	require.Nil(t, resp.Readiness)
}

func TestTradePlansReadinessAPI_Draft(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "after_close_intent",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-30", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		IntentStatus: readiness.IntentSelected, LimitPrice: 0, TargetVolume: 0,
	}}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/tradeplans/readiness?plan_id="+strconv.FormatUint(uint64(plan.ID), 10), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.NotNil(t, resp.Readiness)
	require.Equal(t, plan.ID, resp.Readiness.PlanID)
	require.Equal(t, "2026-07-30", resp.Readiness.TradeDate)
	require.Equal(t, readiness.StageIntentDraft, resp.Readiness.LifecycleStage)
	require.False(t, resp.Readiness.Ready)
	require.Equal(t, len(resp.Readiness.Blockers) == 0, resp.Readiness.Ready)
	require.NotEmpty(t, resp.Readiness.Blockers)
}

func TestTradePlansReadinessAPI_MaterializedReady(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "morning_materialized",
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-30", StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		IntentStatus: readiness.IntentPriced, LimitPrice: 10, TargetVolume: 10000,
	}}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/tradeplans/readiness?trade_date=2026-07-30", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.NotNil(t, resp.Readiness)
	require.Equal(t, readiness.StageIntentMaterialized, resp.Readiness.LifecycleStage)
	require.True(t, resp.Readiness.Ready)
	require.Empty(t, resp.Readiness.Blockers)
	require.Equal(t, len(resp.Readiness.Blockers) == 0, resp.Readiness.Ready)
}

func TestTradePlansReadinessAPI_E1Blocker(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "after_close_intent",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-30", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		IntentStatus: readiness.IntentSelected, LimitPrice: 0, TargetVolume: 0,
	}}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/tradeplans/readiness?plan_id="+strconv.FormatUint(uint64(plan.ID), 10), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.Readiness.Ready)
	require.True(t, readinessFindingHasRule(resp.Readiness.Blockers, qualitygate.RuleE1))
	require.True(t, readinessFindingHasCode(resp.Readiness.Blockers, qualitygate.CodeEntryPriceMissing))
}

func TestTradePlansReadinessAPI_SkipAware(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "morning_materialized",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{
			TradeDate: "2026-07-30", StockCode: "sz000001", Side: "buy",
			Status: models.TradePlanItemPending, TargetAmount: 100_000,
			IntentStatus: readiness.IntentGapSkip, LimitPrice: 0, TargetVolume: 0,
		},
		{
			TradeDate: "2026-07-30", StockCode: "sz000002", StockName: "万科A", Side: "buy",
			Status: models.TradePlanItemPending, TargetAmount: 100_000,
			IntentStatus: readiness.IntentPriced, LimitPrice: 10, TargetVolume: 1000,
		},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/tradeplans/readiness?plan_id="+strconv.FormatUint(uint64(plan.ID), 10), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Readiness.Ready, "gap_skip must not trip E1: %+v", resp.Readiness.Blockers)
	require.False(t, readinessFindingHasRule(resp.Readiness.Blockers, qualitygate.RuleE1))
	require.False(t, readinessFindingHasCode(resp.Readiness.Blockers, qualitygate.CodeEntryPriceMissing))
}

func TestTradePlansReadinessAPI_WarningOnly(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	aps := 100_000.0
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: aps,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "morning_materialized",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-07-30", StockCode: "a", StockName: "A", Side: "buy", TargetAmount: aps,
			Status: models.TradePlanItemPending, IntentStatus: readiness.IntentPriced, LimitPrice: 10, TargetVolume: 1000},
		{TradeDate: "2026-07-30", StockCode: "b", StockName: "B", Side: "buy", TargetAmount: aps,
			Status: models.TradePlanItemPending, IntentStatus: readiness.IntentPriced, LimitPrice: 10, TargetVolume: 1000},
		{TradeDate: "2026-07-30", StockCode: "c", StockName: "C", Side: "buy", TargetAmount: aps,
			Status: models.TradePlanItemPending, IntentStatus: readiness.IntentPriced, LimitPrice: 10, TargetVolume: 1000},
	}))

	// Library-level WARN-only contract (G1) used by mapper semantics.
	lib := readiness.EvaluateExecutionIntentReadiness(plan, &readiness.Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval: true,
			IndustryByCode: map[string]string{
				"a": "通信设备", "b": "通信设备", "c": "通信设备",
			},
			NameByCode:        map[string]string{"a": "A", "b": "B", "c": "C"},
			AnchorPriceByCode: map[string]float64{"a": 10, "b": 10, "c": 10},
		},
	})
	require.True(t, lib.Ready)
	require.Empty(t, lib.Blockers)
	require.True(t, hasLibWarningRule(lib.Warnings, qualitygate.RuleG1))
	mapped := mapReadyViewForTest(plan, lib)
	require.True(t, mapped.Ready)
	require.Empty(t, mapped.Blockers)
	require.True(t, readinessFindingHasRule(mapped.Warnings, qualitygate.RuleG1))

	// HTTP default Options may emit P1 WARN without industry enrich; Ready iff blockers empty.
	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/tradeplans/readiness?plan_id="+strconv.FormatUint(uint64(plan.ID), 10), nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, len(resp.Readiness.Blockers) == 0, resp.Readiness.Ready)
	require.True(t, resp.Readiness.Ready)
}

func TestTradePlansReadinessAPI_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("tradeplans_readiness.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"ApproveTradePlan(",
		"ApproveDraft(",
		"PromoteDraftToFrozen(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"EvaluateDraftTradePlanRisk(",
		"GetUpcomingTradePlan(",
		"TradePlanVisibilityView",
	} {
		require.NotContains(t, src, token, "must not contain %s", token)
	}
	require.Contains(t, src, "len(blockers) == 0")
	require.Contains(t, src, "GetByID")
	require.Contains(t, src, "GetLatestByTradeDate")
	require.Contains(t, src, "do NOT use QualityGate Passed")
}

func TestTradePlansReadinessAPI_ChainMount(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	h := api.ChainAssetMiddleware(api.TradePlansAssetMiddleware)(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/readiness?trade_date=2099-01-01", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, nextCalled)

	var resp api.TradePlanReadinessResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNoPlan, resp.Code)
}

func readinessFindingHasRule(fs []api.ReadinessFindingView, rule string) bool {
	for _, f := range fs {
		if f.RuleCode == rule {
			return true
		}
	}
	return false
}

func readinessFindingHasCode(fs []api.ReadinessFindingView, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

func hasLibWarningRule(fs []readiness.Finding, rule string) bool {
	for _, f := range fs {
		if f.RuleCode == rule {
			return true
		}
	}
	return false
}

// mapReadyViewForTest mirrors api mapper ready:=len(blockers)==0 without importing unexported helpers.
func mapReadyViewForTest(plan *models.TradePlan, res readiness.ExecutionIntentReadinessResult) api.ExecutionIntentReadinessView {
	blockers := make([]api.ReadinessFindingView, 0, len(res.Blockers))
	for _, f := range res.Blockers {
		blockers = append(blockers, api.ReadinessFindingView{
			RuleCode: f.RuleCode, Code: f.Code, Severity: "block", Message: f.Message, Evidence: f.Evidence,
		})
	}
	warnings := make([]api.ReadinessFindingView, 0, len(res.Warnings))
	for _, f := range res.Warnings {
		warnings = append(warnings, api.ReadinessFindingView{
			RuleCode: f.RuleCode, Code: f.Code, Severity: "warn", Message: f.Message, Evidence: f.Evidence,
		})
	}
	return api.ExecutionIntentReadinessView{
		PlanID: plan.ID, TradeDate: plan.TradeDate, LifecycleStage: res.LifecycleStage,
		Ready: len(blockers) == 0, Blockers: blockers, Warnings: warnings,
	}
}
