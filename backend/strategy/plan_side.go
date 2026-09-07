package strategy

import (
	"go-stock/backend/models"
	"go-stock/backend/sellgate"
)

// IsSellSide reports side=sell (case-insensitive).
func IsSellSide(side string) bool {
	return sellgate.IsSellSide(side)
}

// IsPureSellPlan reports a plan whose Side and every item Side are sell.
func IsPureSellPlan(plan *models.TradePlan) bool {
	return sellgate.IsPureSellPlan(plan)
}
