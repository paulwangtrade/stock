package tradingrule

import "go-stock/backend/data"

func init() {
	// Phase12-M2.5-C2.1/C2.3: wire Validate-only Submit+Fill gates without import cycle.
	data.RegisterQuantityPolicySubmitHooks(EnableQuantityPolicy, func(stockCode string, volume int64) bool {
		return ValidateBuyQuantity(MetaFromStockCode(stockCode), volume).Accepted
	})
}
