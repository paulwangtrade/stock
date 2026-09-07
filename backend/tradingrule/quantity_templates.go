package tradingrule

import "go-stock/backend/instrument"

// Static buy templates (Phase12-M2.1). Sell / close-out fields deferred to M2.3.

// TemplateMAIN: A-share main board (and default equity) — 100 share multiples.
func TemplateMAIN() OrderQuantityRules {
	return OrderQuantityRules{
		Key:          "CN:EQUITY:MAIN",
		QtyUnit:      instrument.QtyUnitShare,
		BuyMinQty:    100,
		BuyStepQty:   100,
		StepAfterMin: false,
	}
}

// TemplateSTAR: STAR board — min 200, then 1-share increments.
func TemplateSTAR() OrderQuantityRules {
	return OrderQuantityRules{
		Key:          "CN:EQUITY:STAR",
		QtyUnit:      instrument.QtyUnitShare,
		BuyMinQty:    200,
		BuyStepQty:   1,
		StepAfterMin: true,
	}
}

// TemplateETF: default listed fund — 100 share multiples (override later via M1 table).
func TemplateETF() OrderQuantityRules {
	return OrderQuantityRules{
		Key:          "CN:ETF:DEFAULT",
		QtyUnit:      instrument.QtyUnitETFShare,
		BuyMinQty:    100,
		BuyStepQty:   100,
		StepAfterMin: false,
	}
}

// TemplateConvertibleBond: bond units — 10-lot multiples (indicative).
func TemplateConvertibleBond() OrderQuantityRules {
	return OrderQuantityRules{
		Key:          "CN:CONVERTIBLE_BOND",
		QtyUnit:      instrument.QtyUnitBondUnit,
		BuyMinQty:    10,
		BuyStepQty:   10,
		StepAfterMin: false,
	}
}
