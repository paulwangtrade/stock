package tradingrule

// adjustBuyQty floors raw buy qty to a legal lot under the rules (pure).
//
//	MAIN/ETF/CB: floor to BuyStepQty; reject if below BuyMinQty
//	STAR (StepAfterMin): accept any qty >= BuyMinQty (1-share step)
func adjustBuyQty(r OrderQuantityRules, rawQty int64) BuyQuantityResult {
	out := BuyQuantityResult{RawQty: rawQty, RuleKey: r.RuleKey()}
	if rawQty <= 0 {
		out.NormalizedQty = 0
		out.Reason = BuyQtyReasonRejectZero
		return out
	}
	minQty := r.BuyMinQty
	if minQty <= 0 {
		minQty = 1
	}
	if rawQty < minQty {
		out.NormalizedQty = 0
		out.Reason = BuyQtyReasonRejectBelowMin
		return out
	}

	var adjusted int64
	if r.StepAfterMin {
		// STAR: >= min, unit step 1 — keep raw.
		adjusted = rawQty
	} else {
		step := r.BuyStepQty
		if step <= 0 {
			step = minQty
		}
		adjusted = (rawQty / step) * step
		if adjusted < minQty {
			out.NormalizedQty = 0
			out.Reason = BuyQtyReasonRejectBelowMin
			return out
		}
	}

	out.NormalizedQty = adjusted
	if adjusted == rawQty {
		out.Reason = BuyQtyReasonOK
	} else {
		out.Reason = BuyQtyReasonAdjusted
	}
	return out
}
