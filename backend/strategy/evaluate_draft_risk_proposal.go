package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/risk"
)

// RiskProposalResult is the live plan-state risk proposal for a Draft TradePlan.
// It is not an Order, not Shadow TradePlanProposal, and does not authorize execution.
type RiskProposalResult struct {
	TradePlanID      uint                   `json:"tradePlanId"`
	TradePlanVersion int                    `json:"tradePlanVersion"`
	Passed           bool                   `json:"passed"`
	PlanFilterResult *risk.PlanFilterResult `json:"planFilterResult"`
	RiskReasons      []string               `json:"riskReasons"`
	CheckedAt        time.Time              `json:"checkedAt"`
}

// planFilterContextFn allows tests to inject PlanContext without touching Execution.
var planFilterContextFn = loadPlanFilterContext

// EvaluateDraftTradePlanRisk runs plan-state PlanFilter/PlanCheck against a Draft
// TradePlan and returns a RiskProposalResult. It does not mutate Status, does not
// Approve/Freeze, and does not call Execution or Trading Gate.
func EvaluateDraftTradePlanRisk(plan *models.TradePlan) (*RiskProposalResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("trade plan is nil")
	}
	if plan.ID == 0 {
		return nil, fmt.Errorf("trade plan id is required")
	}
	if !plan.IsDraft() {
		return nil, fmt.Errorf("trade plan %d status=%s, want draft", plan.ID, plan.Status)
	}
	if len(plan.Items) == 0 {
		return nil, fmt.Errorf("trade plan %d has no items", plan.ID)
	}

	amount := plan.AmountPerStock
	if amount <= 0 {
		amount = 100_000
	}
	maxNames := plan.MaxNames
	if maxNames <= 0 {
		maxNames = defaultMaxPlanNames
	}

	cands := tradePlanItemsToPlanCandidates(plan.Items, amount)
	ctx := planFilterContextFn(amount, maxNames)
	filtered := risk.PlanFilter(cands, ctx)
	if filtered == nil {
		return nil, fmt.Errorf("plan filter result is nil")
	}

	checkedAt := time.Now()
	result := &RiskProposalResult{
		TradePlanID:      plan.ID,
		TradePlanVersion: plan.PlanVersion,
		Passed:           riskProposalPassed(filtered),
		PlanFilterResult: filtered,
		RiskReasons:      collectRiskReasons(filtered),
		CheckedAt:        checkedAt,
	}

	logger.SugaredLogger.Infof(
		"EvaluateDraftTradePlanRisk planId=%d version=%d status=%s passed=%v riskStatus=%s reasons=%d",
		plan.ID, plan.PlanVersion, plan.Status, result.Passed, filtered.RiskStatus, len(result.RiskReasons),
	)
	return result, nil
}

func tradePlanItemsToPlanCandidates(items []models.TradePlanItem, defaultAmount float64) []risk.PlanCandidate {
	out := make([]risk.PlanCandidate, 0, len(items))
	for _, it := range items {
		amount := it.TargetAmount
		if amount <= 0 {
			amount = defaultAmount
		}
		out = append(out, risk.PlanCandidate{
			StockCode:       it.StockCode,
			StockName:       it.StockName,
			Rank:            it.Priority,
			Score:           it.Score,
			Reason:          it.Reason,
			StrategyName:    it.StrategyName,
			StrategyVersion: it.StrategyVersion,
			TargetAmount:    amount,
		})
	}
	return out
}

func riskProposalPassed(filtered *risk.PlanFilterResult) bool {
	if filtered == nil {
		return false
	}
	switch filtered.RiskStatus {
	case risk.PlanRiskStatusPassed, risk.PlanRiskStatusBypassed:
		return true
	default:
		return false
	}
}

func collectRiskReasons(filtered *risk.PlanFilterResult) []string {
	if filtered == nil {
		return nil
	}
	reasons := make([]string, 0)
	seen := map[string]bool{}
	for _, it := range filtered.Items {
		if it.Allowed {
			continue
		}
		code := strings.TrimSpace(string(it.RiskCode))
		msg := strings.TrimSpace(it.RiskMessage)
		line := code
		if msg != "" {
			if line != "" {
				line = code + ": " + msg
			} else {
				line = msg
			}
		}
		if line == "" {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		reasons = append(reasons, line)
	}
	if len(reasons) == 0 && filtered.RiskSummary != "" &&
		(filtered.RiskStatus == risk.PlanRiskStatusBlocked || filtered.RiskStatus == risk.PlanRiskStatusPartial) {
		reasons = append(reasons, filtered.RiskSummary)
	}
	return reasons
}
