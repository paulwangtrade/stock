package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func seedAPIAfterCloseIntentDraft(t *testing.T) *models.TradePlan {
	t.Helper()
	slip := 0.03
	plan := &models.TradePlan{
		TradeDate: "2026-08-05", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		PlanVersion: 1, PricingPolicyVersion: 1, PricingStage: "after_close_intent",
		SourceSession: models.TradePlanSourceAfterClose,
		DefaultEntryRule: "LIMIT_REF_PLUS_SLIP", DefaultMaxSlippage: &slip,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-08-05", StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, RefSource: "prev_close", RefAsOf: "2026-08-04",
		EntryRule: "LIMIT_REF_PLUS_SLIP", MaxSlippage: &slip,
		IntentStatus: readiness.IntentSelected, LimitPrice: 0, TargetVolume: 0,
	}}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func materializeMorningTestRunner(planID uint, _ *strategy.MorningIntentMaterializeOpts) (*strategy.MorningIntentMaterializeResult, error) {
	return strategy.RunMorningIntentMaterialize(planID, &strategy.MorningIntentMaterializeOpts{
		OpenPriceFn: func(string) (float64, bool) { return 10.2, true },
		VolumeOpts: &strategy.MorningPositionMaterializeOpts{
			Snapshot: &strategy.MorningAccountSnapshot{
				Cash: 1_000_000, Equity: 1_000_000,
				NameMarketValue: map[string]float64{},
				PositionVolumes: map[string]int64{},
			},
			Limits: &strategy.MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.50},
		},
		EvaluateReadiness: func(p *models.TradePlan) readiness.ExecutionIntentReadinessResult {
			return readiness.EvaluateExecutionIntentReadiness(p, &readiness.Options{SkipQualityGate: true})
		},
	})
}

func postMaterializeMorning(t *testing.T, mux *http.ServeMux, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tradeplans/materialize-morning", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

func TestTradePlansMaterializeMorningAPI_DraftHappyPath(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIAfterCloseIntentDraft(t)

	before := readiness.EvaluateExecutionIntentReadiness(plan, &readiness.Options{SkipQualityGate: true})
	require.False(t, before.Ready)
	beforeN := len(before.Blockers)

	mux := http.NewServeMux()
	h := api.NewTradePlansHandler().WithMorningMaterializeRunner(materializeMorningTestRunner)
	api.RegisterTradePlansHandler(mux, h)

	rec := postMaterializeMorning(t, mux, map[string]any{"plan_id": plan.ID})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanMaterializeMorningResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.Equal(t, plan.ID, resp.PlanID)
	require.Equal(t, 1, resp.MaterializedItems)
	require.True(t, resp.ReadinessReady)
	require.Empty(t, resp.Blockers)
	require.Less(t, len(resp.Blockers), beforeN)
	require.Equal(t, "morning_materialized", resp.PricingStage)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.InDelta(t, 10.0*(1+0.03), got.Items[0].LimitPrice, 1e-9)
	require.GreaterOrEqual(t, got.Items[0].TargetVolume, int64(100))
}

func TestTradePlansMaterializeMorningAPI_Idempotent(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIAfterCloseIntentDraft(t)

	mux := http.NewServeMux()
	h := api.NewTradePlansHandler().WithMorningMaterializeRunner(materializeMorningTestRunner)
	api.RegisterTradePlansHandler(mux, h)

	rec1 := postMaterializeMorning(t, mux, map[string]any{"plan_id": plan.ID})
	require.Equal(t, http.StatusOK, rec1.Code)
	var r1 api.TradePlanMaterializeMorningResponse
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &r1))
	require.True(t, r1.Success)

	got1, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	limit := got1.Items[0].LimitPrice
	vol := got1.Items[0].TargetVolume

	rec2 := postMaterializeMorning(t, mux, map[string]any{"plan_id": plan.ID})
	require.Equal(t, http.StatusOK, rec2.Code)
	var r2 api.TradePlanMaterializeMorningResponse
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &r2))
	require.True(t, r2.Success)
	require.Equal(t, 1, r2.MaterializedItems)

	got2, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, limit, got2.Items[0].LimitPrice)
	require.Equal(t, vol, got2.Items[0].TargetVolume)
}

func TestTradePlansMaterializeMorningAPI_MissingPlanID(t *testing.T) {
	setupAPITestDB(t)
	mux := http.NewServeMux()
	api.RegisterTradePlansHandler(mux, api.NewTradePlansHandler())
	rec := postMaterializeMorning(t, mux, map[string]any{})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp api.TradePlanMaterializeMorningResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Equal(t, api.TradePlanCodeInvalidPlanID, resp.Code)
}

func TestTradePlansMaterializeMorningAPI_MethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterTradePlansHandler(mux, api.NewTradePlansHandler())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/materialize-morning", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
