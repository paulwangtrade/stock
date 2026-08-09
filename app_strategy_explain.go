package main

import (
	"context"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/strategyexplain"
	"go-stock/backend/usagemetrics"
)

// StrategyExplanationDTO is the Wails-facing Explanation for TradePlan / Observation UI.
type StrategyExplanationDTO = strategyexplain.Explanation

// ExplainStrategySnapshot renders a read-only strategy explanation from a StrategySnapshot id.
// Gated by Feature AdvancedObservation. Does not touch Broker / Gateway / Fill.
func (a *App) ExplainStrategySnapshot(tier string, snapshotID string, includeExit bool) *StrategyExplanationDTO {
	user := shellUserForTier(tier)
	_ = entitlement.Default().EnsureTierDefaults(user)
	_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedObservation, usagemetrics.EventOpened, map[string]string{
		"scene":  "strategy_explanation",
		"source": "wails",
	})
	out, err := strategyexplain.Default().Explain(context.Background(), strategyexplain.ExplainRequest{
		User:        user,
		SnapshotID:  snapshotID,
		IncludeExit: includeExit,
	})
	if err != nil || out == nil {
		return &strategyexplain.Explanation{
			Status:      strategyexplain.StatusFailed,
			Headline:    "解释生成失败",
			Disclaimers: []string{"本说明为只读策略解释，不构成投资建议或买卖指令。"},
		}
	}
	return out
}

// ExplainStrategyPlanItem resolves PlanReference → item snapshot, then explains.
func (a *App) ExplainStrategyPlanItem(tier string, planID uint, planItemID uint, includeExit bool) *StrategyExplanationDTO {
	user := shellUserForTier(tier)
	_ = entitlement.Default().EnsureTierDefaults(user)
	_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedObservation, usagemetrics.EventOpened, map[string]string{
		"scene":  "strategy_explanation",
		"source": "wails_plan_item",
	})
	out, err := strategyexplain.Default().Explain(context.Background(), strategyexplain.ExplainRequest{
		User:        user,
		PlanID:      planID,
		PlanItemID:  planItemID,
		IncludeExit: includeExit,
	})
	if err != nil || out == nil {
		return &strategyexplain.Explanation{
			Status:      strategyexplain.StatusFailed,
			Headline:    "解释生成失败",
			Disclaimers: []string{"本说明为只读策略解释，不构成投资建议或买卖指令。"},
		}
	}
	return out
}
