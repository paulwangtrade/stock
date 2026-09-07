package papertrading_test

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestExitPolicy_DefaultBehaviourMatchesLegacy(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sh600105", HoldingDays: 25, UnrealizedReturn: ptrFloat(0.10),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 1, HoldingDays: 25, ReturnRate: ptrFloat(0.10),
			}},
		}},
	}
	legacy := papertrading.ProjectExitEvaluation(holding, papertrading.DefaultExitEvaluationPolicy)
	viaPolicy := papertrading.ProjectExitEvaluationWithExitPolicy(holding, papertrading.DefaultExitPolicy, nil)
	require.Equal(t, legacy.Holdings[0].Evaluation.State, viaPolicy.Holdings[0].Evaluation.State)
	require.Equal(t, legacy.Holdings[0].Evaluation.ReasonCodes, viaPolicy.Holdings[0].Evaluation.ReasonCodes)
	require.Equal(t, "default_v1", viaPolicy.Policy.PolicyID)
	require.Equal(t, 1, viaPolicy.Policy.Version)
	require.Equal(t, 20, viaPolicy.Policy.MaxHoldingDays)
}

func TestExitPolicy_CustomMaxHoldingDays(t *testing.T) {
	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "sz000001", HoldingDays: 6, UnrealizedReturn: ptrFloat(0.01),
			Lots: []papertrading.HoldingEvalLotRow{{
				FillID: 2, HoldingDays: 6, ReturnRate: ptrFloat(0.01),
			}},
		}},
	}
	// default: 6 <= 20 → NORMAL
	def := papertrading.ProjectExitEvaluationWithExitPolicy(holding, papertrading.DefaultExitPolicy, nil)
	require.Equal(t, papertrading.ExitEvalStateNormal, def.Holdings[0].Evaluation.State)
	require.NotContains(t, def.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonTimeReview)

	custom := papertrading.ExitPolicy{
		PolicyID: "short_v1", Version: 1, MaxHoldingDays: 5,
		LossWatchThreshold: -0.05, LossReviewThreshold: -0.10,
	}
	view := papertrading.ProjectExitEvaluationWithExitPolicy(holding, custom, nil)
	require.Equal(t, papertrading.ExitEvalStateWatch, view.Holdings[0].Evaluation.State)
	require.Contains(t, view.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonTimeReview)
	require.Equal(t, "short_v1", view.Policy.PolicyID)
	require.Equal(t, 5, view.Policy.MaxHoldingDays)
}

func TestExitPolicy_LossThresholds(t *testing.T) {
	asOf := exitAsOf(t)
	base := func(ret float64) *papertrading.HoldingEvalObservationView {
		return &papertrading.HoldingEvalObservationView{
			AsOf: asOf,
			Holdings: []papertrading.HoldingEvalStockRow{{
				StockCode: "sz000002", HoldingDays: 3, UnrealizedReturn: ptrFloat(ret),
				Lots: []papertrading.HoldingEvalLotRow{{
					FillID: 3, HoldingDays: 3, ReturnRate: ptrFloat(ret),
				}},
			}},
		}
	}
	pol := papertrading.DefaultExitPolicy

	watch := papertrading.ProjectExitEvaluationWithExitPolicy(base(-0.05), pol, nil)
	require.Equal(t, papertrading.ExitEvalStateWatch, watch.Holdings[0].Evaluation.State)
	require.Contains(t, watch.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)

	review := papertrading.ProjectExitEvaluationWithExitPolicy(base(-0.10), pol, nil)
	require.Equal(t, papertrading.ExitEvalStateReviewRequired, review.Holdings[0].Evaluation.State)
	require.Contains(t, review.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)

	ok := papertrading.ProjectExitEvaluationWithExitPolicy(base(-0.04), pol, nil)
	require.Equal(t, papertrading.ExitEvalStateNormal, ok.Holdings[0].Evaluation.State)
	require.NotContains(t, ok.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonLossReview)
}

func TestExitPolicy_EmptyAndAbnormalFallback(t *testing.T) {
	empty := papertrading.ExitPolicy{}.Normalized()
	require.Equal(t, papertrading.DefaultExitPolicy.PolicyID, empty.PolicyID)
	require.Equal(t, 20, empty.MaxHoldingDays)
	require.Equal(t, -0.05, empty.LossWatchThreshold)
	require.Equal(t, -0.10, empty.LossReviewThreshold)

	bad := papertrading.ExitPolicy{
		PolicyID: "x", Version: -1, MaxHoldingDays: 0,
		LossWatchThreshold: 0, LossReviewThreshold: 0,
	}.Normalized()
	require.Equal(t, 20, bad.MaxHoldingDays)
	require.Equal(t, -0.05, bad.LossWatchThreshold)

	// inverted thresholds → review clamped to watch
	inv := papertrading.ExitPolicy{
		PolicyID: "inv", Version: 1, MaxHoldingDays: 10,
		LossWatchThreshold: -0.10, LossReviewThreshold: -0.02,
	}.Normalized()
	require.Equal(t, -0.10, inv.LossWatchThreshold)
	require.Equal(t, -0.10, inv.LossReviewThreshold)

	require.NotPanics(t, func() {
		_ = papertrading.BuiltinExitPolicyProvider{}.Resolve(papertrading.ExitPolicyResolveContext{})
		_ = papertrading.ResolveExitPolicy(papertrading.ExitEvaluationBuildOptions{}, papertrading.ExitPolicyResolveContext{})
		cat, err := papertrading.ParseExitPolicyCatalogJSON(nil)
		require.NoError(t, err)
		_ = cat.Resolve(papertrading.ExitPolicyResolveContext{})
	})
}

func TestExitPolicy_CatalogJSONAndProvider(t *testing.T) {
	raw := []byte(`{
		"default_policy_id": "default_v1",
		"policies": [
			{
				"policy_id": "default_v1",
				"version": 1,
				"max_holding_days": 20,
				"loss_watch_threshold": -0.05,
				"loss_review_threshold": -0.10
			},
			{
				"policy_id": "short_v1",
				"version": 1,
				"max_holding_days": 5,
				"loss_watch_threshold": -0.03,
				"loss_review_threshold": -0.06
			}
		],
		"strategy_map": { "momentum_v1": "short_v1" }
	}`)
	cat, err := papertrading.ParseExitPolicyCatalogJSON(raw)
	require.NoError(t, err)

	def := cat.Resolve(papertrading.ExitPolicyResolveContext{})
	require.Equal(t, "default_v1", def.PolicyID)
	require.Equal(t, 20, def.MaxHoldingDays)

	short := cat.Resolve(papertrading.ExitPolicyResolveContext{PolicyID: "short_v1"})
	require.Equal(t, 5, short.MaxHoldingDays)

	byStrat := cat.Resolve(papertrading.ExitPolicyResolveContext{StrategyName: "momentum_v1"})
	require.Equal(t, "short_v1", byStrat.PolicyID)

	t.Cleanup(papertrading.ResetExitPolicyProviderForTest)
	papertrading.SetExitPolicyProviderForTest(cat)

	asOf := exitAsOf(t)
	holding := &papertrading.HoldingEvalObservationView{
		AsOf: asOf,
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "x", HoldingDays: 6, UnrealizedReturn: ptrFloat(0),
			Lots: []papertrading.HoldingEvalLotRow{{FillID: 9, HoldingDays: 6, ReturnRate: ptrFloat(0)}},
		}},
	}
	// Build path with explicit ExitPolicy override (short)
	shortPol := short
	view := papertrading.ProjectExitEvaluationWithExitPolicy(holding, shortPol, nil)
	require.Contains(t, view.Holdings[0].Evaluation.ReasonCodes, papertrading.ExitReasonTimeReview)

	resolved := papertrading.ResolveExitPolicy(papertrading.ExitEvaluationBuildOptions{}, papertrading.ExitPolicyResolveContext{})
	require.Equal(t, "default_v1", resolved.PolicyID)
}

func TestExitPolicy_NoForbiddenActionCodes(t *testing.T) {
	holding := &papertrading.HoldingEvalObservationView{
		Holdings: []papertrading.HoldingEvalStockRow{{
			StockCode: "y", HoldingDays: 100, UnrealizedReturn: ptrFloat(-0.5),
			Lots: []papertrading.HoldingEvalLotRow{{FillID: 1, HoldingDays: 100, ReturnRate: ptrFloat(-0.5)}},
		}},
	}
	custom := papertrading.ExitPolicy{
		PolicyID: "harsh", Version: 1, MaxHoldingDays: 1,
		LossWatchThreshold: -0.01, LossReviewThreshold: -0.02,
	}
	view := papertrading.ProjectExitEvaluationWithExitPolicy(holding, custom, nil)
	joined := stringsJoin(view.Holdings[0].Evaluation.ReasonCodes)
	require.NotContains(t, joined, "SELL")
	require.NotContains(t, joined, "EXIT_NOW")
	require.NotContains(t, joined, "FORCE_CLOSE")
}

func TestExitPolicy_ProviderUsedByBuildOptions(t *testing.T) {
	t.Cleanup(papertrading.ResetExitPolicyProviderForTest)
	cat, err := papertrading.ParseExitPolicyCatalogJSON([]byte(`{
		"default_policy_id": "short_v1",
		"policies": [{
			"policy_id": "short_v1",
			"version": 2,
			"max_holding_days": 5,
			"loss_watch_threshold": -0.05,
			"loss_review_threshold": -0.10
		}]
	}`))
	require.NoError(t, err)
	pol := papertrading.ResolveExitPolicy(
		papertrading.ExitEvaluationBuildOptions{PolicyProvider: cat},
		papertrading.ExitPolicyResolveContext{},
	)
	require.Equal(t, "short_v1", pol.PolicyID)
	require.Equal(t, 5, pol.MaxHoldingDays)
	require.Equal(t, 2, pol.Version)
}
