package anchor

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
)

// FollowedStockAnchorProvider resolves ref_price from followed_stock only.
// Behavior mirrors legacy strategy.defaultAfterCloseAnchor (Phase10-B.0):
//   FollowPrice > 0 preferred, else Price > 0; otherwise ok=false.
// It never reads realtime open quotes, Kline, stock_info, or CandidatePool prices.
type FollowedStockAnchorProvider struct{}

// Resolve implements Provider.
func (FollowedStockAnchorProvider) Resolve(ctx Context) (Result, bool) {
	code := strings.TrimSpace(ctx.StockCode)
	if code == "" || db.Dao == nil {
		return Result{}, false
	}

	var follow data.FollowedStock
	if err := db.Dao.Where("stock_code = ?", code).First(&follow).Error; err != nil {
		return Result{}, false
	}

	px := follow.FollowPrice
	if px <= 0 {
		px = follow.Price
	}
	if px <= 0 {
		return Result{}, false
	}

	asOf := strings.TrimSpace(ctx.TradeDate)
	if !follow.Time.IsZero() {
		asOf = follow.Time.Format("2006-01-02")
	}

	src := RefSourcePrevClose
	if strings.TrimSpace(ctx.PoolSource) == PoolSourceStrategyRun {
		src = RefSourceStrategySnapshot
	}

	return Result{
		RefPrice:   px,
		RefSource:  src,
		RefAsOf:    asOf,
		Confidence: 0.5,
	}, true
}
