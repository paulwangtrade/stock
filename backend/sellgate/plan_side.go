package sellgate

import (
	"strings"

	"go-stock/backend/models"
)

// IsSellSide reports side=sell (case-insensitive).
func IsSellSide(side string) bool {
	return strings.EqualFold(strings.TrimSpace(side), "sell")
}

// IsPureSellPlan reports a plan whose Side and every item Side are sell.
func IsPureSellPlan(plan *models.TradePlan) bool {
	if plan == nil || len(plan.Items) == 0 {
		return false
	}
	if !IsSellSide(plan.Side) {
		return false
	}
	for _, it := range plan.Items {
		if !IsSellSide(it.Side) {
			return false
		}
	}
	return true
}
