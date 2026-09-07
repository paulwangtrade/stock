package holdingdecision

import (
	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
)

// DecideHolding runs the H1.2 chain (suggest_only):
//
//	Holding → Facts → Rule Evaluation → RuleHit → Merger → HoldingDecision
//
// Default policy / no hits → HOLD. Never creates SellTradePlan or calls Execution.
func DecideHolding(facts rules.HoldingFacts, pol rules.Policy) rules.HoldingDecision {
	pol.SuggestOnly = true
	return rules.Decide(facts, pol)
}

// BuildHoldingFacts is a thin alias for rules.BuildFacts at the holdingdecision package boundary.
func BuildHoldingFacts(
	symbol string,
	weight float64,
	qty int64,
	marketValue float64,
	cost, price, returnRate *float64,
	holdingDays int,
	canSell bool,
	availableQty int64,
	trend *rules.TrendFact,
	risk *portfoliorisk.PortfolioRiskSnapshot,
	industry string,
	tightened *rules.TightenedConstraints,
) rules.HoldingFacts {
	return rules.BuildFacts(symbol, weight, qty, marketValue, cost, price, returnRate, holdingDays, canSell, availableQty, trend, risk, industry, tightened)
}

// EvaluateWithRules runs the H1.2 Rule Engine for one symbol (RuleContext path).
// Default rules.Policy → HOLD. Never creates SellTradePlan or calls Execution.
func EvaluateWithRules(ctx rules.RuleContext) rules.HoldingDecisionResult {
	ctx.Policy.SuggestOnly = true
	return rules.Evaluate(ctx)
}

// EvaluateAllWithRules evaluates each context independently (observation batch).
func EvaluateAllWithRules(rows []rules.RuleContext) []rules.HoldingDecisionResult {
	out := make([]rules.HoldingDecisionResult, 0, len(rows))
	for _, row := range rows {
		row.Policy.SuggestOnly = true
		out = append(out, rules.Evaluate(row))
	}
	return out
}

// RiskSnapshotFromPortfolio projects a full PortfolioRiskSnapshot for rule contexts.
// Optional helper; does not Build risk internally.
func RiskSnapshotFromPortfolio(snap *portfoliorisk.PortfolioRiskSnapshot) *portfoliorisk.PortfolioRiskSnapshot {
	return snap
}

// ToActionDecision maps an H1.2 HoldingDecision onto the H1.1 ActionDecision shape (still observation only).
func ToActionDecision(d rules.HoldingDecision) ActionDecision {
	ev := Evidence{}
	if v, ok := d.Evidence["current_price"].(float64); ok {
		ev.CurrentPrice = &v
	}
	if v, ok := d.Evidence["return_rate"].(float64); ok {
		ev.ReturnRate = &v
	}
	if v, ok := d.Evidence["holding_days"].(int); ok {
		ev.HoldingDays = v
	}
	if v, ok := d.Evidence["risk_state"].(string); ok {
		ev.RiskState = v
	}
	if v, ok := d.Evidence["profit_state"].(string); ok {
		ev.ProfitState = v
	}
	if v, ok := d.Evidence["period_state"].(string); ok {
		ev.PeriodState = v
	}
	out := ActionDecision{
		Symbol:             d.Symbol,
		Action:             d.FinalAction,
		Reason:             firstReason(d.ReasonCodes),
		ReasonCodes:        append([]string{}, d.ReasonCodes...),
		Summary:            d.Explanation,
		Evidence:           ev,
		ExecutableHint:     d.ExecutableHint,
		RecordOnly:         true,
		NotAnOrder:         true,
		PersistSellPlans:   false,
		SuggestOnly:        true,
		FiredRules:         toFiredRules(d.RuleHits),
		ConflictResolution: append([]string{}, d.ConflictResolution...),
	}
	if w, ok := d.Evidence["weight"].(float64); ok {
		out.Weight = w
	}
	switch d.FinalAction {
	case rules.ActionReduce:
		frac := 0.5
		out.Reduce = &ReduceHint{Fraction: &frac}
	case rules.ActionExit:
		out.Exit = &ExitHint{Intent: ExitIntentFlattenSellable}
	}
	return out
}

// FiredRule is a compact fired-rule projection on ActionDecision.
type FiredRule struct {
	RuleID     string `json:"rule_id"`
	Family     string `json:"family,omitempty"`
	ReasonCode string `json:"reason_code,omitempty"`
}

func toFiredRules(hits []rules.RuleHit) []FiredRule {
	if len(hits) == 0 {
		return nil
	}
	out := make([]FiredRule, 0, len(hits))
	for _, h := range hits {
		out = append(out, FiredRule{RuleID: h.RuleID, Family: h.Family, ReasonCode: h.ReasonCode})
	}
	return out
}

func firstReason(codes []string) string {
	if len(codes) == 0 {
		return ReasonNone
	}
	return codes[0]
}
