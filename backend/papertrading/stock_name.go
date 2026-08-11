package papertrading

import (
	"strings"

	"go-stock/backend/models"
	"go-stock/backend/stockname"
)

// resolvePaperSimStockName returns a non-empty display name for order/fill/position writes.
// Non-empty item.StockName is kept. Empty names go through StockNameResolver.
// Unresolved → "未知名称"; never returns "". Does not reject the fill.
func resolvePaperSimStockName(plan *models.TradePlan, item models.TradePlanItem) string {
	if n := strings.TrimSpace(item.StockName); n != "" {
		return n
	}
	hint := stockname.Hint{PlanItemID: item.ID, PlanID: item.PlanID}
	if plan != nil {
		if hint.PlanID == 0 {
			hint.PlanID = plan.ID
		}
		if hint.PoolID == 0 {
			hint.PoolID = plan.PoolID
		}
	}
	return stockname.ResolveWithHint(item.StockCode, hint).Name
}
