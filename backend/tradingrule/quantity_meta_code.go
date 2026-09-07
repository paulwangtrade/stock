package tradingrule

import "go-stock/backend/instrument"

// MetaFromStockCode resolves M1 template QuantityMeta from a stock code (no DB).
func MetaFromStockCode(stockCode string) instrument.QuantityMeta {
	id, err := instrument.Classify(stockCode)
	if err != nil {
		return instrument.TemplateQuantityMeta(instrument.SecurityUnknown, instrument.BoardUNKNOWN)
	}
	meta := instrument.TemplateQuantityMeta(id.SecurityType, id.MarketSegment)
	meta.Symbol = id.Symbol
	meta.IdentitySource = id.Source
	return meta
}
