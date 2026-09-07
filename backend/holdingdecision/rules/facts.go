package rules

import "go-stock/backend/portfoliorisk"

// HoldingFacts is the Fact layer between Holding rows and Rule Evaluation (H1.2).
// Built only from injected observation inputs — never from TradePlan / Execution.
type HoldingFacts struct {
	Symbol string

	Weight       float64
	TotalQty     int64
	MarketValue  float64
	Cost         *float64
	CurrentPrice *float64
	ReturnRate   *float64
	HoldingDays  int
	ProfitState  string
	RiskState    string
	PeriodState  string

	CanSell      bool
	AvailableQty int64

	Trend     *TrendFact
	Industry  string
	RiskSnap  *portfoliorisk.PortfolioRiskSnapshot
	Tightened *TightenedConstraints
}

// BuildFacts assembles Facts from holding / position / optional enrichments.
// Missing PnL or Trend fields stay nil/empty — rules skip rather than invent.
func BuildFacts(
	symbol string,
	weight float64,
	qty int64,
	marketValue float64,
	cost, price, returnRate *float64,
	holdingDays int,
	canSell bool,
	availableQty int64,
	trend *TrendFact,
	risk *portfoliorisk.PortfolioRiskSnapshot,
	industry string,
	tightened *TightenedConstraints,
) HoldingFacts {
	return HoldingFacts{
		Symbol:       symbol,
		Weight:       weight,
		TotalQty:     qty,
		MarketValue:  marketValue,
		Cost:         cost,
		CurrentPrice: price,
		ReturnRate:   returnRate,
		HoldingDays:  holdingDays,
		CanSell:      canSell,
		AvailableQty: availableQty,
		Trend:        trend,
		Industry:     industry,
		RiskSnap:     risk,
		Tightened:    tightened,
	}
}

// ToContext maps Facts + Policy → RuleContext for evaluation.
func (f HoldingFacts) ToContext(pol Policy) RuleContext {
	return RuleContext{
		Symbol:        f.Symbol,
		Weight:        f.Weight,
		TotalQty:      f.TotalQty,
		MarketValue:   f.MarketValue,
		Cost:          f.Cost,
		CurrentPrice:  f.CurrentPrice,
		ReturnRate:    f.ReturnRate,
		HoldingDays:   f.HoldingDays,
		ProfitState:   f.ProfitState,
		RiskState:     f.RiskState,
		PeriodState:   f.PeriodState,
		CanSell:       f.CanSell,
		AvailableQty:  f.AvailableQty,
		Trend:         f.Trend,
		PortfolioRisk: f.RiskSnap,
		Industry:      f.Industry,
		Tightened:     f.Tightened,
		Policy:        pol,
	}
}

// FactEvidence returns a stable evidence map for the HoldingDecision payload.
func (f HoldingFacts) FactEvidence() map[string]any {
	ev := map[string]any{
		"symbol":        f.Symbol,
		"weight":        f.Weight,
		"total_qty":     f.TotalQty,
		"holding_days":  f.HoldingDays,
		"can_sell":      f.CanSell,
		"available_qty": f.AvailableQty,
	}
	if f.Cost != nil {
		ev["cost"] = *f.Cost
	}
	if f.CurrentPrice != nil {
		ev["current_price"] = *f.CurrentPrice
	}
	if f.ReturnRate != nil {
		ev["return_rate"] = *f.ReturnRate
	}
	if f.ProfitState != "" {
		ev["profit_state"] = f.ProfitState
	}
	if f.RiskState != "" {
		ev["risk_state"] = f.RiskState
	}
	if f.PeriodState != "" {
		ev["period_state"] = f.PeriodState
	}
	if f.Industry != "" {
		ev["industry"] = f.Industry
	}
	if f.Trend != nil {
		ev["trend_available"] = f.Trend.Available
		ev["trend_thesis_broken"] = f.Trend.ThesisBroken
		ev["trend_below_ma"] = f.Trend.BelowMA
	} else {
		ev["trend_available"] = false
	}
	if f.RiskSnap != nil {
		ev["portfolio_risk_found"] = f.RiskSnap.Found
		ev["concentration_available"] = f.RiskSnap.Concentration.Available
		ev["exposure_available"] = f.RiskSnap.Exposure.Available
		ev["sector_available"] = f.RiskSnap.Sector.Available
	}
	return ev
}

// Decide runs the full H1.2 chain:
//
//	HoldingFacts → Rule Evaluation → RuleHit → Decision Merger → HoldingDecision
//
// suggest_only is always true. No hits → HOLD.
func Decide(facts HoldingFacts, pol Policy) HoldingDecision {
	pol.SuggestOnly = true
	ctx := facts.ToContext(pol)
	res := Evaluate(ctx)
	res.SuggestOnly = true
	res.Action = res.FinalAction
	res.Evidence = mergeEvidence(facts.FactEvidence(), res.RuleHits)
	if res.FinalAction == "" {
		res.FinalAction = ActionHold
		res.Action = ActionHold
	}
	return res
}

// EvaluateRules exposes Rule Evaluation only (hits before merge semantics still go through Evaluate).
func EvaluateRules(facts HoldingFacts, pol Policy) []RuleHit {
	pol.SuggestOnly = true
	ctx := facts.ToContext(pol)
	ctx = enrichDerived(ctx)
	if !pol.EngineEnabled {
		return nil
	}
	var hits []RuleHit
	for _, r := range defaultRules() {
		hits = append(hits, r.Eval(ctx)...)
	}
	return hits
}

func mergeEvidence(base map[string]any, hits []RuleHit) map[string]any {
	out := cloneEvidence(base)
	if out == nil {
		out = map[string]any{}
	}
	if len(hits) == 0 {
		out["rule_hit_count"] = 0
		return out
	}
	out["rule_hit_count"] = len(hits)
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.RuleID)
		for k, v := range h.Evidence {
			key := h.RuleID + "." + k
			out[key] = v
		}
	}
	out["fired_rule_ids"] = ids
	return out
}
