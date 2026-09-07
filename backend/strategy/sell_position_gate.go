package strategy

import (
	"time"

	"go-stock/backend/sellgate"
)

// SellPositionGateResult is the read-only sell availability check for one symbol.
type SellPositionGateResult = sellgate.SellPositionGateResult

// ErrSellNoPosition is returned when the symbol has no paper_sim holding.
var ErrSellNoPosition = sellgate.ErrSellNoPosition

// ErrSellInvalidStock is returned when stock_code is empty or not a holding.
var ErrSellInvalidStock = sellgate.ErrSellInvalidStock

// ErrSellNotSellable is returned when PositionState.can_sell is false.
var ErrSellNotSellable = sellgate.ErrSellNotSellable

// ErrSellInsufficientAvailable is returned when quantity exceeds available_volume.
var ErrSellInsufficientAvailable = sellgate.ErrSellInsufficientAvailable

// ErrSellInvalidQuantity is returned when quantity is zero or fails lot alignment.
var ErrSellInvalidQuantity = sellgate.ErrSellInvalidQuantity

// CheckSellPositionGate validates sell quantity against PositionState + paper_sim Snapshot.
// Does not read legacy paper_* tables.
func CheckSellPositionGate(stockCode string, quantity int64, tradeDate string, asOf time.Time) (*SellPositionGateResult, error) {
	return sellgate.CheckSellPositionGate(stockCode, quantity, tradeDate, asOf)
}

// VolumeMeetsSellLot validates sell lot alignment (MVP: 100-share lot; no QuantityPolicy sell rules).
func VolumeMeetsSellLot(stockCode string, volume int64) bool {
	return sellgate.VolumeMeetsSellLot(stockCode, volume)
}
