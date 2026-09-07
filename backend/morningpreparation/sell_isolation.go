package morningpreparation

import (
	"strings"

	"go-stock/backend/models"
)

const tSellDraftPricingStage = "t_sell_draft"

// isMorningBuySide is the 09:26 adapter rule: only consume buy (legacy empty = buy).
func isMorningBuySide(plan *models.TradePlan) bool {
	if plan == nil {
		return false
	}
	side := strings.ToLower(strings.TrimSpace(plan.Side))
	return side == "" || side == "buy"
}

// isSellIsolatedFromBuyMaterialize is a second gate so a leaked sell plan is
// never Attempted. Primary picker is isMorningBuySide + buy-draft fallback.
func isSellIsolatedFromBuyMaterialize(plan *models.TradePlan) bool {
	if plan == nil {
		return false
	}
	if !isMorningBuySide(plan) {
		return true
	}
	src := strings.TrimSpace(plan.SourceSession)
	if strings.EqualFold(src, models.TradePlanSourceTSell) ||
		strings.EqualFold(src, models.TradePlanSourceExitReview) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(plan.PricingStage), tSellDraftPricingStage) {
		return true
	}
	return false
}
