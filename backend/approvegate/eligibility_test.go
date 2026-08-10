package approvegate

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func materializedReadyPlan() *models.TradePlan {
	return &models.TradePlan{
		ID:                   101,
		TradeDate:            "2026-07-29",
		Status:               models.TradePlanStatusDraft,
		AmountPerStock:       100_000,
		PlanVersion:          1,
		PricingPolicyVersion: 1,
		PricingStage:         "morning_materialized",
		Items: []models.TradePlanItem{{
			StockCode: "sz000001", StockName: "平安银行", Side: "buy",
			TargetAmount: 100_000, IntentStatus: readiness.IntentPriced,
			LimitPrice: 10.0, TargetVolume: 10_000,
		}},
	}
}

func unmaterializedPlan() *models.TradePlan {
	return &models.TradePlan{
		ID:                   102,
		TradeDate:            "2026-07-29",
		Status:               models.TradePlanStatusDraft,
		AmountPerStock:       100_000,
		PlanVersion:          1,
		PricingPolicyVersion: 1,
		PricingStage:         "after_close_intent",
		Items: []models.TradePlanItem{{
			StockCode: "sz000001", Side: "buy", TargetAmount: 100_000,
			IntentStatus: readiness.IntentSelected, LimitPrice: 0, TargetVolume: 0,
		}},
	}
}

func passRiskEvaluator(plan *models.TradePlan) (*strategy.RiskProposalResult, error) {
	return &strategy.RiskProposalResult{
		TradePlanID:      plan.ID,
		TradePlanVersion: plan.PlanVersion,
		Passed:           true,
		CheckedAt:        time.Now(),
	}, nil
}

func failRiskEvaluator(plan *models.TradePlan) (*strategy.RiskProposalResult, error) {
	return &strategy.RiskProposalResult{
		TradePlanID:      plan.ID,
		TradePlanVersion: plan.PlanVersion,
		Passed:           false,
		RiskReasons:      []string{"market blocked"},
		CheckedAt:        time.Now(),
	}, nil
}

func readinessOptsSkipGap() *readiness.Options {
	return &readiness.Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval: true,
			IndustryByCode: map[string]string{"sz000001": "银行"},
			NameByCode:     map[string]string{"sz000001": "平安银行"},
			AnchorPriceByCode: map[string]float64{"sz000001": 10},
		},
	}
}

func TestCheckApproveEligibility_ReadyAndRiskPass(t *testing.T) {
	plan := materializedReadyPlan()
	res, err := CheckApproveEligibility(plan.ID, &EligibilityOptions{
		LoadPlan: func(uint) (*models.TradePlan, error) { return plan, nil },
		EvaluateRisk: passRiskEvaluator,
		ReadinessOpts: readinessOptsSkipGap(),
	})
	require.NoError(t, err)
	require.True(t, res.Eligible)
	require.False(t, res.AlreadyApproved)
	require.NotNil(t, res.RiskResult)
	require.True(t, res.RiskResult.Passed)
	require.NotNil(t, res.ReadinessResult)
	require.True(t, res.ReadinessResult.Ready)
	require.Empty(t, res.Blockers)
}

func TestCheckApproveEligibility_RiskFail(t *testing.T) {
	plan := materializedReadyPlan()
	res, err := CheckApproveEligibility(plan.ID, &EligibilityOptions{
		LoadPlan: func(uint) (*models.TradePlan, error) { return plan, nil },
		EvaluateRisk: failRiskEvaluator,
		ReadinessOpts: readinessOptsSkipGap(),
	})
	require.NoError(t, err)
	require.False(t, res.Eligible)
	require.NotNil(t, res.RiskResult)
	require.False(t, res.RiskResult.Passed)
	require.True(t, hasCode(res.Blockers, CodeRiskNotPassed))
}

func TestCheckApproveEligibility_ReadinessBlocker(t *testing.T) {
	plan := unmaterializedPlan()
	res, err := CheckApproveEligibility(plan.ID, &EligibilityOptions{
		LoadPlan: func(uint) (*models.TradePlan, error) { return plan, nil },
		EvaluateRisk: passRiskEvaluator,
		ReadinessOpts: readinessOptsSkipGap(),
	})
	require.NoError(t, err)
	require.False(t, res.Eligible)
	require.NotNil(t, res.ReadinessResult)
	require.False(t, res.ReadinessResult.Ready)
	require.True(t, hasCode(res.Blockers, readiness.CodeLimitMissing))
	require.True(t, res.RiskResult.Passed)
}

func TestCheckApproveEligibility_AlreadyApproved(t *testing.T) {
	plan := materializedReadyPlan()
	now := time.Now()
	plan.ApprovedAt = &now
	plan.ApprovedBy = "alice"
	plan.ApprovedSource = "system"

	res, err := CheckApproveEligibility(plan.ID, &EligibilityOptions{
		LoadPlan: func(uint) (*models.TradePlan, error) { return plan, nil },
		EvaluateRisk: passRiskEvaluator,
		ReadinessOpts: readinessOptsSkipGap(),
	})
	require.NoError(t, err)
	require.True(t, res.AlreadyApproved)
	require.True(t, res.Eligible, "repeat approve re-runs gate; eligible when Risk and Readiness pass")
	require.True(t, hasCode(res.Warnings, CodeAlreadyApproved))
	require.Empty(t, res.Blockers)
}

func TestCheckApproveEligibility_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("eligibility.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"ApproveDraft(",
		"ApproveTradePlan(",
		"PromoteDraftToFrozen(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunDailyCandidateAndPlan(",
		"TradePlanStatusReady",
	} {
		require.NotContains(t, src, token)
	}
	require.Contains(t, src, "EvaluateDraftTradePlanRisk")
	require.Contains(t, src, "EvaluateExecutionIntentReadiness")
}

func hasCode(fs []readiness.Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}
