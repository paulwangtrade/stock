// Package sellgate holds read-only sell-side classification and PositionState availability checks.
// It is a shared leaf package so readiness and strategy do not import each other.
package sellgate

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
)

// SellPositionGateResult is the read-only sell availability check for one symbol.
type SellPositionGateResult struct {
	Symbol       string
	StockName    string
	AvailableQty int64
	CanSell      bool
	AvgCost      float64
}

// ErrSellNoPosition is returned when the symbol has no paper_sim holding.
var ErrSellNoPosition = fmt.Errorf("no position")

// ErrSellInvalidStock is returned when stock_code is empty or not a holding.
var ErrSellInvalidStock = fmt.Errorf("invalid stock")

// ErrSellNotSellable is returned when PositionState.can_sell is false.
var ErrSellNotSellable = fmt.Errorf("not sellable")

// ErrSellInsufficientAvailable is returned when quantity exceeds available_volume.
var ErrSellInsufficientAvailable = fmt.Errorf("insufficient available")

// ErrSellInvalidQuantity is returned when quantity is zero or fails lot alignment.
var ErrSellInvalidQuantity = fmt.Errorf("invalid quantity")

// CheckSellPositionGate validates sell quantity against PositionState + paper_sim Snapshot.
// Does not read legacy paper_* tables.
func CheckSellPositionGate(stockCode string, quantity int64, tradeDate string, asOf time.Time) (*SellPositionGateResult, error) {
	code := strings.TrimSpace(stockCode)
	if code == "" {
		return nil, ErrSellInvalidStock
	}
	if quantity <= 0 {
		return nil, ErrSellInvalidQuantity
	}
	if !VolumeMeetsSellLot(code, quantity) {
		return nil, ErrSellInvalidQuantity
	}
	td := strings.TrimSpace(tradeDate)
	if td == "" {
		if asOf.IsZero() {
			asOf = time.Now()
		}
		td = asOf.Format("2006-01-02")
	}
	if asOf.IsZero() {
		asOf = time.Now()
	}

	psSvc := positionstate.NewService(nil)
	bundle := psSvc.Evaluate(positionstate.Query{TradeDate: td, AsOf: asOf})
	var view *positionstate.PositionStateView
	for i := range bundle.Positions {
		if strings.EqualFold(strings.TrimSpace(bundle.Positions[i].Symbol), code) {
			view = &bundle.Positions[i]
			break
		}
	}
	if view == nil || view.TotalQty <= 0 {
		return nil, ErrSellNoPosition
	}
	if !view.CanSell || view.AvailableQty <= 0 {
		return nil, ErrSellNotSellable
	}
	if quantity > view.AvailableQty {
		return nil, ErrSellInsufficientAvailable
	}

	snap, _ := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{AsOf: asOf})
	name := ""
	avgCost := 0.0
	if snap != nil && snap.Found {
		for _, p := range snap.Positions {
			if strings.EqualFold(strings.TrimSpace(p.StockCode), code) {
				name = strings.TrimSpace(p.StockName)
				avgCost = p.AvgCost
				break
			}
		}
	}

	return &SellPositionGateResult{
		Symbol:       code,
		StockName:    name,
		AvailableQty: view.AvailableQty,
		CanSell:      view.CanSell,
		AvgCost:      avgCost,
	}, nil
}

// VolumeMeetsSellLot validates sell lot alignment (MVP: 100-share lot; no QuantityPolicy sell rules).
func VolumeMeetsSellLot(_ string, volume int64) bool {
	const lot = int64(100)
	return volume >= lot && volume%lot == 0
}
