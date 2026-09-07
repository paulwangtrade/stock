package holdingdecision

import (
	"strings"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
)

// toRulesPolicy maps ActionPolicy → rules.Policy (suggest_only forced).
func toRulesPolicy(pol ActionPolicy) rules.Policy {
	rp := pol.RulePolicy
	out := rules.Policy{
		EngineEnabled:          rp.EngineEnabled || pol.UseRuleEngine,
		SuggestOnly:            true,
		ReduceEnabled:          pol.ReduceEnabled,
		ExitEnabled:            pol.ExitEnabled,
		EnablePnL:              rp.EnablePnL,
		EnableTenure:           rp.EnableTenure,
		EnableTrend:            rp.EnableTrend,
		EnableRisk:             rp.EnableRisk,
		EnablePortfolioTighten: rp.EnablePortfolioTighten,
		TakeProfitReturn:       rp.TakeProfitReturn,
		LargeProfitProtect:     rp.LargeProfitProtect,
		MaxLossReturn:          rp.MaxLossReturn,
		StaleHoldingDays:       rp.StaleHoldingDays,
		SoftHoldingDays:        rp.SoftHoldingDays,
		DefaultReduceFraction:  pol.DefaultReduceFraction,
	}
	if out.DefaultReduceFraction <= 0 || out.DefaultReduceFraction >= 1 {
		out.DefaultReduceFraction = 0.5
	}
	// When UseRuleEngine without explicit EngineEnabled, still run enabled families only if any Enable* set;
	// if none set, EngineEnabled alone with all families false → HOLD (safe).
	if pol.UseRuleEngine {
		out.EngineEnabled = true
	}
	return out
}

// buildFactsForSymbol is the Fact Builder step for Observe H1.2 path.
func buildFactsForSymbol(sym string, in ObservationInput) rules.HoldingFacts {
	hold := lookupHolding(in.Holdings, sym)
	pos := lookupPos(in.PositionStates, sym)
	ev := lookupEval(in.Evaluations, sym)

	days := ev.HoldingDays
	if days == 0 && pos.HoldingDays > 0 {
		days = pos.HoldingDays
	}
	qty := hold.TotalQty
	if qty <= 0 {
		qty = pos.TotalQty
	}
	avail := pos.AvailableQty

	var trend *rules.TrendFact
	if in.TrendFacts != nil {
		if tf, ok := in.TrendFacts[sym]; ok {
			trend = &rules.TrendFact{
				Available:    tf.Available,
				ThesisBroken: tf.ThesisBroken,
				BelowMA:      tf.BelowMA,
				Source:       tf.Source,
				Note:         tf.Note,
			}
		} else if tf, ok := in.TrendFacts[strings.ToUpper(sym)]; ok {
			trend = &rules.TrendFact{
				Available: tf.Available, ThesisBroken: tf.ThesisBroken, BelowMA: tf.BelowMA,
				Source: tf.Source, Note: tf.Note,
			}
		}
	}

	industry := ""
	if in.Industries != nil {
		industry = in.Industries[sym]
		if industry == "" {
			industry = in.Industries[strings.ToUpper(sym)]
		}
	}

	var risk *portfoliorisk.PortfolioRiskSnapshot
	if in.PortfolioRisk != nil {
		risk = in.PortfolioRisk
	} else if in.RiskSnapshot != nil && in.RiskSnapshot.Found {
		// Lightweight projection for concentration-only rules when full snap absent.
		risk = &portfoliorisk.PortfolioRiskSnapshot{Found: true}
		if in.RiskSnapshot.MaxSingleNamePct != nil {
			cap := *in.RiskSnapshot.MaxSingleNamePct
			risk.Concentration.Available = cap > 0
			if cap > 0 {
				risk.Concentration.CapSingle = &cap
			}
		}
		if in.RiskSnapshot.GrossExposure != nil {
			risk.Exposure.Available = true
			g := *in.RiskSnapshot.GrossExposure
			risk.Exposure.GrossExposure = &g
		}
	}

	facts := rules.BuildFacts(
		sym, hold.Weight, qty, hold.MarketValue,
		nil, ev.CurrentPrice, ev.ReturnRate,
		days, pos.CanSell, avail,
		trend, risk, industry, nil,
	)
	facts.ProfitState = ev.ProfitState
	facts.RiskState = ev.RiskState
	facts.PeriodState = ev.PeriodState
	return facts
}

// decideActionWithRules runs Fact Builder → Rule Evaluation → Merger → ActionDecision.
func decideActionWithRules(sym string, in ObservationInput, pol ActionPolicy) ActionDecision {
	facts := buildFactsForSymbol(sym, in)
	dec := rules.Decide(facts, toRulesPolicy(pol))
	d := ToActionDecision(dec)
	d.SuggestOnly = true
	d.ConflictResolution = append([]string{}, dec.ConflictResolution...)
	// Preserve D.8-ish state projection lightly.
	if d.State == "" {
		d.State = StateHoldNormal
		for _, h := range dec.RuleHits {
			if h.ReasonCode == rules.ReasonTenureLongReview || h.Severity == rules.SeverityWatch {
				d.State = StateHoldWatch
			}
			if h.ActionCandidate == rules.ActionExit || h.Severity == rules.SeverityCritical {
				if pol.ExitCandidateEnabled {
					d.State = StateExitCandidate
				} else {
					d.State = StateHoldReview
				}
			}
		}
	}
	if pol.DefaultReduceFraction > 0 && pol.DefaultReduceFraction < 1 && d.Reduce != nil && d.Reduce.Fraction == nil {
		frac := pol.DefaultReduceFraction
		d.Reduce.Fraction = &frac
	}
	return d
}
