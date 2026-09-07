package rules

import (
	"math"
	"strings"

	"go-stock/backend/papertrading"
)

// Evaluate runs enabled H1.2 rules for one symbol and merges to HoldingDecisionResult.
// Default Policy (engine off) → HOLD with no business hits. Always suggest_only.
func Evaluate(ctx RuleContext) HoldingDecisionResult {
	pol := ctx.Policy
	pol.SuggestOnly = true
	if pol.DefaultReduceFraction <= 0 || pol.DefaultReduceFraction >= 1 {
		pol.DefaultReduceFraction = 0.5
	}
	ctx.Policy = pol
	ctx = enrichDerived(ctx)

	hint := executableHint(ctx)

	if !pol.EngineEnabled {
		res := MergeHits(ctx.Symbol, nil, pol, hint)
		res.SuggestOnly = true
		res.Action = res.FinalAction
		return res
	}

	var hits []RuleHit
	for _, r := range defaultRules() {
		hits = append(hits, r.Eval(ctx)...)
	}
	res := MergeHits(ctx.Symbol, hits, pol, hint)
	res.SuggestOnly = true
	res.Action = res.FinalAction
	return res
}

func defaultRules() []HoldingRule {
	return []HoldingRule{
		pnlTakeProfitRule{},
		pnlLargeProtectRule{},
		pnlMaxLossRule{},
		pnlRiskDeteriorationRule{},
		tenureStaleRule{},
		tenureLongReviewRule{},
		trendBreakRule{},
		riskNameOverCapRule{},
		riskGrossHotRule{},
		riskSectorHotRule{},
		portfolioTightenRule{},
	}
}

func enrichDerived(ctx RuleContext) RuleContext {
	if ctx.ReturnRate == nil && ctx.Cost != nil && ctx.CurrentPrice != nil && *ctx.Cost > 0 {
		v := (*ctx.CurrentPrice - *ctx.Cost) / *ctx.Cost
		ctx.ReturnRate = &v
	}
	if ctx.ProfitState == "" {
		ctx.ProfitState = papertrading.ClassifyProfitState(ctx.ReturnRate)
	}
	if ctx.RiskState == "" && ctx.ReturnRate != nil {
		ctx.RiskState = papertrading.ClassifyRiskState(ctx.ReturnRate, papertrading.DefaultRiskStateThresholds)
	}
	if ctx.PeriodState == "" && ctx.HoldingDays > 0 {
		ctx.PeriodState = papertrading.ClassifyHoldingPeriodState(ctx.HoldingDays, papertrading.DefaultHoldingPeriodThresholds)
	}
	return ctx
}

func executableHint(ctx RuleContext) bool {
	if ctx.TotalQty <= 0 {
		return false
	}
	if !ctx.CanSell {
		return false
	}
	return ctx.AvailableQty > 0
}

func hasPnLFacts(ctx RuleContext) bool {
	if ctx.ReturnRate != nil {
		return true
	}
	return ctx.Cost != nil && ctx.CurrentPrice != nil && *ctx.Cost > 0
}

func weightAboveCap(weight, cap float64) bool {
	if cap <= 0 || math.IsNaN(weight) || weight <= 0 {
		return false
	}
	return weight > cap+1e-12
}

func cloneEvidence(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func hit(ruleID, family, action, severity, reason string, ev map[string]any) RuleHit {
	return RuleHit{
		RuleID:          ruleID,
		Family:          family,
		ActionCandidate: action,
		Severity:        severity,
		ReasonCode:      reason,
		Evidence:        cloneEvidence(ev),
	}
}

func normIndustry(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
