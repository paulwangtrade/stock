package rules

import (
	"strings"

	"go-stock/backend/papertrading"
)

// PnL observation rules. Missing cost/price/return → family does not fire.

type pnlTakeProfitRule struct{}

func (pnlTakeProfitRule) ID() string     { return RulePnLTakeProfit }
func (pnlTakeProfitRule) Family() string { return FamilyPnL }

func (pnlTakeProfitRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnablePnL || pol.TakeProfitReturn <= 0 || !hasPnLFacts(ctx) || ctx.ReturnRate == nil {
		return nil
	}
	if *ctx.ReturnRate < pol.TakeProfitReturn {
		return nil
	}
	return []RuleHit{hit(RulePnLTakeProfit, FamilyPnL, ActionReduce, SeverityWatch, ReasonPnLTakeProfit, map[string]any{
		"return_rate":        *ctx.ReturnRate,
		"take_profit_return": pol.TakeProfitReturn,
		"note":               "profit_ratio reached target → observe REDUCE",
	})}
}

type pnlLargeProtectRule struct{}

func (pnlLargeProtectRule) ID() string     { return RulePnLLargeProtect }
func (pnlLargeProtectRule) Family() string { return FamilyPnL }

func (pnlLargeProtectRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnablePnL || pol.LargeProfitProtect <= 0 || !hasPnLFacts(ctx) || ctx.ReturnRate == nil {
		return nil
	}
	if *ctx.ReturnRate < pol.LargeProfitProtect {
		return nil
	}
	return []RuleHit{hit(RulePnLLargeProtect, FamilyPnL, ActionReduce, SeverityHigh, ReasonPnLLargeProtect, map[string]any{
		"return_rate":          *ctx.ReturnRate,
		"large_profit_protect": pol.LargeProfitProtect,
		"note":                 "large profit protection → observe REDUCE",
	})}
}

type pnlMaxLossRule struct{}

func (pnlMaxLossRule) ID() string     { return RulePnLMaxLoss }
func (pnlMaxLossRule) Family() string { return FamilyPnL }

func (pnlMaxLossRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	// MaxLossReturn is negative (e.g. -0.10). ≥0 means disabled.
	if !pol.EnablePnL || pol.MaxLossReturn >= 0 || !hasPnLFacts(ctx) || ctx.ReturnRate == nil {
		return nil
	}
	if *ctx.ReturnRate > pol.MaxLossReturn {
		return nil
	}
	action := ActionReduce
	sev := SeverityHigh
	if pol.ExitEnabled && strings.EqualFold(strings.TrimSpace(ctx.PeriodState), papertrading.HoldingPeriodLong) {
		action = ActionExit
		sev = SeverityCritical
	}
	return []RuleHit{hit(RulePnLMaxLoss, FamilyPnL, action, sev, ReasonPnLMaxLoss, map[string]any{
		"return_rate":     *ctx.ReturnRate,
		"max_loss_return": pol.MaxLossReturn,
		"period_state":    ctx.PeriodState,
		"note":            "loss beyond max tolerance → observe",
	})}
}

// pnlRiskDeteriorationRule: risk_state DANGER (or WATCH with loss) → REDUCE observation.
type pnlRiskDeteriorationRule struct{}

func (pnlRiskDeteriorationRule) ID() string     { return RulePnLRiskWorse }
func (pnlRiskDeteriorationRule) Family() string { return FamilyPnL }

func (pnlRiskDeteriorationRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnablePnL {
		return nil
	}
	risk := strings.ToUpper(strings.TrimSpace(ctx.RiskState))
	if risk != papertrading.RiskStateDanger && risk != papertrading.RiskStateWatch {
		return nil
	}
	// Prefer explicit risk_state; if only return-derived empty, skip (enrichDerived may fill).
	action := ActionHold
	sev := SeverityWatch
	reasonNote := "risk watch → HOLD observation"
	if risk == papertrading.RiskStateDanger {
		action = ActionReduce
		sev = SeverityHigh
		reasonNote = "risk deterioration DANGER → observe REDUCE"
	} else if strings.EqualFold(strings.TrimSpace(ctx.ProfitState), papertrading.ProfitStateLoss) {
		action = ActionReduce
		sev = SeverityWatch
		reasonNote = "risk WATCH with loss → observe REDUCE"
	}
	return []RuleHit{hit(RulePnLRiskWorse, FamilyPnL, action, sev, ReasonPnLRiskWorse, map[string]any{
		"risk_state":   ctx.RiskState,
		"profit_state": ctx.ProfitState,
		"return_rate":  ctx.ReturnRate,
		"note":         reasonNote,
	})}
}
