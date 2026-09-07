package tradingrule

import "go-stock/backend/instrument"

// BuyQuantityValidation is the Phase12-M2.3+ Broker validation result (Validate-only).
type BuyQuantityValidation struct {
	Accepted bool   `json:"accepted"`
	Qty      int64  `json:"qty"` // on accept: original qty unchanged; on reject: 0 or hint from Normalize
	Reason   string `json:"reason"`
	RuleKey  string `json:"rule_key,omitempty"`
	// WouldNormalize is diagnostic only when reject because qty is adjustable but not legal as-is
	// (e.g. CB 11 → would be 10). Broker must NOT apply this qty (M2.3.2 Validate-only).
	WouldNormalize bool  `json:"would_normalize,omitempty"`
	SuggestedQty   int64 `json:"suggested_qty,omitempty"`
}

// ValidateBuyQuantity reports whether qty is already a legal buy lot (strict, no adjust accept).
// OK iff NormalizeBuy(qty).NormalizedQty == qty && qty > 0.
func ValidateBuyQuantity(meta instrument.QuantityMeta, qty int64) BuyQuantityValidation {
	res := NormalizeBuyQuantity(meta, qty)
	if res.NormalizedQty > 0 && res.NormalizedQty == qty {
		return BuyQuantityValidation{
			Accepted: true, Qty: qty, Reason: res.Reason, RuleKey: res.RuleKey,
		}
	}
	out := BuyQuantityValidation{
		Accepted: false, Qty: 0, Reason: res.Reason, RuleKey: res.RuleKey,
	}
	if res.NormalizedQty > 0 && res.NormalizedQty != qty {
		out.WouldNormalize = true
		out.SuggestedQty = res.NormalizedQty
		if out.Reason == "" || out.Reason == BuyQtyReasonAdjusted {
			out.Reason = BuyQtyReasonAdjusted // not aligned; Broker rejects (Validate-only)
		}
	}
	return out
}

// ApplyBrokerBuyQuantityGate is the Broker final quantity gate (Phase12-M2.3.2 Validate-only).
//
//	Flag OFF → pass-through (Accepted iff qty>0); does not alter qty
//	Flag ON  → ValidateBuyQuantity only; illegal → reject; never rewrites qty
//	          (CB 11 → reject; Materialize/Policy owns normalize/generation)
//
// Does not replace resolveBuyQuantity /100; callers apply this after resolve.
func ApplyBrokerBuyQuantityGate(stockCode string, qty int64) BuyQuantityValidation {
	if !EnableQuantityPolicy() {
		if qty <= 0 {
			return BuyQuantityValidation{Accepted: false, Qty: 0, Reason: BuyQtyReasonRejectZero}
		}
		return BuyQuantityValidation{Accepted: true, Qty: qty, Reason: "FLAG_OFF"}
	}
	meta := MetaFromStockCode(stockCode)
	v := ValidateBuyQuantity(meta, qty)
	if v.Accepted {
		v.Qty = qty // never rewrite
	}
	return v
}
