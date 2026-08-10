package readiness

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/qualitygate"

	"github.com/stretchr/testify/require"
)

func baseDraftPlan(items []models.TradePlanItem) *models.TradePlan {
	return &models.TradePlan{
		ID:                   42,
		TradeDate:            "2026-07-29",
		Status:               models.TradePlanStatusDraft,
		Side:                 "buy",
		AmountPerStock:       100_000,
		PlanVersion:          1,
		PricingPolicyVersion: 1,
		PricingStage:         "after_close_intent",
		Items:                items,
	}
}

func TestEvaluate_DraftNoMaterialize(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{{
		StockCode: "sz000001", Side: "buy", TargetAmount: 100_000,
		IntentStatus: IntentSelected, LimitPrice: 0, TargetVolume: 0,
	}})
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{SkipGapEval: true},
	})
	require.Equal(t, StageIntentDraft, res.LifecycleStage)
	require.False(t, res.Materialized)
	require.False(t, res.Ready)
	require.True(t, hasCode(res.Blockers, CodeIntentNotMaterialized))
	require.True(t, hasCode(res.Blockers, CodeLimitMissing))
	require.True(t, hasCode(res.Blockers, CodeVolumeBelowLot))
	require.True(t, hasCode(res.Blockers, CodeNoTradeable))
}

func TestEvaluate_PricedNoVolume(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{{
		StockCode: "sz000001", Side: "buy", TargetAmount: 100_000,
		IntentStatus: IntentPriced, LimitPrice: 10.3, TargetVolume: 0,
	}})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{SkipGapEval: true},
	})
	require.Equal(t, StageIntentMaterialized, res.LifecycleStage)
	require.False(t, res.Materialized)
	require.False(t, res.Ready)
	require.True(t, hasCode(res.Blockers, CodePricedNoVolume))
	require.True(t, hasCode(res.Blockers, CodeVolumeBelowLot))
	require.False(t, hasCode(res.Blockers, CodeLimitMissing))
}

func TestEvaluate_FullMaterialized(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{{
		StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		TargetAmount: 100_000, IntentStatus: IntentPriced,
		LimitPrice: 10.0, TargetVolume: 10_000,
	}})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval: true,
			IndustryByCode: map[string]string{"sz000001": "银行"},
			NameByCode:     map[string]string{"sz000001": "平安银行"},
			AnchorPriceByCode: map[string]float64{"sz000001": 10},
		},
	})
	require.Equal(t, StageIntentMaterialized, res.LifecycleStage)
	require.True(t, res.Materialized)
	require.True(t, res.Ready)
	require.Equal(t, 1, res.TradeableCount)
	require.Empty(t, res.Blockers)
}

func TestEvaluate_GapSkipDoesNotTriggerE1(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{
		{
			StockCode: "sz000001", Side: "buy", TargetAmount: 100_000,
			IntentStatus: IntentGapSkip, LimitPrice: 0, TargetVolume: 0,
		},
		{
			StockCode: "sz000002", StockName: "万科A", Side: "buy", TargetAmount: 100_000,
			IntentStatus: IntentPriced, LimitPrice: 10, TargetVolume: 1000,
		},
	})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval:    true,
			IndustryByCode: map[string]string{"sz000002": "地产"},
			NameByCode:     map[string]string{"sz000002": "万科A"},
			AnchorPriceByCode: map[string]float64{"sz000002": 10},
		},
	})
	require.True(t, res.Materialized)
	require.True(t, res.Ready, "gap_skip limit=0 must not block via E1: %+v", res.Blockers)
	require.Equal(t, 1, res.SkipCount)
	require.False(t, hasRule(res.Blockers, qualitygate.RuleE1))
	require.False(t, hasCode(res.Blockers, qualitygate.CodeEntryPriceMissing))
}

func TestEvaluate_SizeSkipDoesNotTriggerE1(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{
		{
			StockCode: "sz000001", Side: "buy", TargetAmount: 100_000,
			IntentStatus: IntentSizeSkip, LimitPrice: 10.5, TargetVolume: 0,
		},
		{
			StockCode: "sz000002", StockName: "万科A", Side: "buy", TargetAmount: 100_000,
			IntentStatus: IntentPriced, LimitPrice: 10, TargetVolume: 1000,
		},
	})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval:    true,
			IndustryByCode: map[string]string{"sz000002": "地产"},
			NameByCode:     map[string]string{"sz000002": "万科A"},
			AnchorPriceByCode: map[string]float64{"sz000002": 10},
		},
	})
	require.True(t, res.Ready, "size_skip must not block: %+v", res.Blockers)
	require.False(t, hasRule(res.Blockers, qualitygate.RuleE1))
}

func TestEvaluate_GapSkipOnly_NoTradeable(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{{
		StockCode: "sz000001", Side: "buy",
		IntentStatus: IntentGapSkip, LimitPrice: 0, TargetVolume: 0,
	}})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{SkipGapEval: true},
	})
	require.True(t, res.Materialized)
	require.False(t, res.Ready)
	require.True(t, hasCode(res.Blockers, CodeNoTradeable))
	require.False(t, hasRule(res.Blockers, qualitygate.RuleE1))
}

func TestEvaluate_QualityGateBlockerI1(t *testing.T) {
	plan := baseDraftPlan([]models.TradePlanItem{{
		StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		TargetAmount: 100_000, IntentStatus: IntentPriced,
		LimitPrice: 10, TargetVolume: 1000,
	}})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		Positions: []qualitygate.AccountPosition{{StockCode: "sz000001", Volume: 500}},
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval:       true,
			IndustryByCode:    map[string]string{"sz000001": "银行"},
			NameByCode:        map[string]string{"sz000001": "平安银行"},
			AnchorPriceByCode: map[string]float64{"sz000001": 10},
		},
	})
	require.False(t, res.Ready)
	require.True(t, hasRule(res.Blockers, qualitygate.RuleI1))
	require.True(t, hasCode(res.Blockers, qualitygate.CodePositionConflict))
}

func TestEvaluate_WarningOnlyReady(t *testing.T) {
	// Three names same sector → G1 WARN; Spec complete → Ready.
	aps := 100_000.0
	plan := baseDraftPlan([]models.TradePlanItem{
		{StockCode: "a", StockName: "A", Side: "buy", TargetAmount: aps, IntentStatus: IntentPriced, LimitPrice: 10, TargetVolume: 1000},
		{StockCode: "b", StockName: "B", Side: "buy", TargetAmount: aps, IntentStatus: IntentPriced, LimitPrice: 10, TargetVolume: 1000},
		{StockCode: "c", StockName: "C", Side: "buy", TargetAmount: aps, IntentStatus: IntentPriced, LimitPrice: 10, TargetVolume: 1000},
	})
	plan.PricingStage = "morning_materialized"
	res := EvaluateExecutionIntentReadiness(plan, &Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval: true,
			IndustryByCode: map[string]string{
				"a": "通信设备", "b": "通信设备", "c": "通信设备",
			},
			NameByCode: map[string]string{"a": "A", "b": "B", "c": "C"},
			AnchorPriceByCode: map[string]float64{"a": 10, "b": 10, "c": 10},
		},
	})
	require.True(t, res.Materialized)
	require.True(t, res.Ready, "WARN must not clear Ready: blockers=%+v", res.Blockers)
	require.Empty(t, res.Blockers)
	require.True(t, hasRule(res.Warnings, qualitygate.RuleG1))
}

func TestEvaluate_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("readiness.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"ApproveTradePlan(",
		"ApproveDraft(",
		"PromoteDraftToFrozen(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"EvaluateDraftTradePlanRisk(",
		"PlanFilter(",
	} {
		require.NotContains(t, src, token)
	}
}

func TestInferLifecycleStage(t *testing.T) {
	p := baseDraftPlan(nil)
	require.Equal(t, StageIntentDraft, inferLifecycleStage(p))
	p.PricingStage = "morning_materialized"
	require.Equal(t, StageIntentMaterialized, inferLifecycleStage(p))
	now := time.Now()
	p.ApprovedAt = &now
	require.Equal(t, StageIntentLocked, inferLifecycleStage(p))
	p.Status = models.TradePlanStatusReady
	p.FreezeAt = &now
	require.Equal(t, StageFrozenSnapshot, inferLifecycleStage(p))
}

func hasCode(fs []Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

func hasRule(fs []Finding, rule string) bool {
	for _, f := range fs {
		if f.RuleCode == rule {
			return true
		}
	}
	return false
}
