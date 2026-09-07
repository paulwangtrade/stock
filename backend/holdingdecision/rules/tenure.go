package rules

import (
	"strings"

	"go-stock/backend/papertrading"
)

// Tenure: long holding with no meaningful change → observation only.
// Must not EXIT solely because days are large.

type tenureStaleRule struct{}

func (tenureStaleRule) ID() string     { return RuleTenureStale }
func (tenureStaleRule) Family() string { return FamilyTenure }

func (tenureStaleRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnableTenure || pol.StaleHoldingDays <= 0 {
		return nil
	}
	days := ctx.HoldingDays
	if days < pol.StaleHoldingDays && !strings.EqualFold(strings.TrimSpace(ctx.PeriodState), papertrading.HoldingPeriodLong) {
		return nil
	}
	// "无变化"：长期 +（亏损或收益接近 0）
	stale := false
	if days >= pol.StaleHoldingDays {
		stale = true
	}
	if strings.EqualFold(strings.TrimSpace(ctx.PeriodState), papertrading.HoldingPeriodLong) {
		stale = true
	}
	if !stale {
		return nil
	}
	flatOrLoss := false
	if ctx.ReturnRate != nil {
		r := *ctx.ReturnRate
		if r <= 0.02 { // no material positive change
			flatOrLoss = true
		}
	}
	if strings.EqualFold(strings.TrimSpace(ctx.ProfitState), papertrading.ProfitStateLoss) {
		flatOrLoss = true
	}
	if !flatOrLoss && ctx.ReturnRate == nil {
		// days only without PnL facts → HOLD observation (review), not REDUCE
		return []RuleHit{hit(RuleTenureStale, FamilyTenure, ActionHold, SeverityWatch, ReasonTenureStale, map[string]any{
			"holding_days":       days,
			"stale_holding_days": pol.StaleHoldingDays,
			"period_state":       ctx.PeriodState,
			"note":               "long tenure without PnL edge facts → HOLD review",
		})}
	}
	if !flatOrLoss {
		return nil
	}
	return []RuleHit{hit(RuleTenureStale, FamilyTenure, ActionReduce, SeverityWatch, ReasonTenureStale, map[string]any{
		"holding_days":       days,
		"stale_holding_days": pol.StaleHoldingDays,
		"return_rate":        ctx.ReturnRate,
		"profit_state":       ctx.ProfitState,
		"note":               "long tenure with flat/loss → observe REDUCE",
	})}
}

// tenureLongReviewRule: soft threshold → HOLD review observation (超期观察).
type tenureLongReviewRule struct{}

func (tenureLongReviewRule) ID() string     { return RuleTenureLongReview }
func (tenureLongReviewRule) Family() string { return FamilyTenure }

func (tenureLongReviewRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnableTenure || pol.SoftHoldingDays <= 0 {
		return nil
	}
	days := ctx.HoldingDays
	if days < pol.SoftHoldingDays && !strings.EqualFold(strings.TrimSpace(ctx.PeriodState), papertrading.HoldingPeriodLong) {
		return nil
	}
	// If stale rule already covers harder REDUCE, still emit review when soft < stale or soft only.
	if pol.StaleHoldingDays > 0 && days >= pol.StaleHoldingDays {
		// still emit long_review as HOLD annotation alongside; merger keeps reasons
	}
	return []RuleHit{hit(RuleTenureLongReview, FamilyTenure, ActionHold, SeverityWatch, ReasonTenureLongReview, map[string]any{
		"holding_days":      days,
		"soft_holding_days": pol.SoftHoldingDays,
		"period_state":      ctx.PeriodState,
		"note":              "long holding → HOLD review observation",
	})}
}
