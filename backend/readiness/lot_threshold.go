package readiness

import "go-stock/backend/tradingrule"

// LotSize is the legacy A-share board lot (100).
// Retained for Flag OFF behavior and JSON/evidence compatibility ("lot_size").
// Flag ON: use VolumeMeetsBuyLot / EffectiveBuyLotThreshold (QuantityPolicy).
const LotSize = int64(100)

// VolumeMeetsBuyLot reports whether target_volume satisfies the buy-lot gate.
//
//	Flag OFF → volume >= LotSize (100) — identical to pre-M2.5-A
//	Flag ON  → ValidateBuyQuantity (Metadata + QuantityPolicy; Validate-only)
func VolumeMeetsBuyLot(stockCode string, volume int64) bool {
	if !tradingrule.EnableQuantityPolicy() {
		return volume >= LotSize
	}
	meta := tradingrule.MetaFromStockCode(stockCode)
	return tradingrule.ValidateBuyQuantity(meta, volume).Accepted
}

// EffectiveBuyLotThreshold is the minimum buy qty used for messaging / evidence.
//
//	Flag OFF → LotSize (100)
//	Flag ON  → Policy MinBuyQty for the instrument
func EffectiveBuyLotThreshold(stockCode string) int64 {
	if !tradingrule.EnableQuantityPolicy() {
		return LotSize
	}
	return tradingrule.PolicyFromInstrumentMeta(tradingrule.MetaFromStockCode(stockCode)).MinBuyQty()
}
