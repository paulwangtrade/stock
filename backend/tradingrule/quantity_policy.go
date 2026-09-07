package tradingrule

import "go-stock/backend/instrument"

// Buy quantity normalize reasons (Phase12-M2.1).
const (
	BuyQtyReasonOK            = "OK"
	BuyQtyReasonAdjusted      = "ADJUSTED"
	BuyQtyReasonRejectBelowMin = "REJECT_BELOW_MIN"
	BuyQtyReasonRejectZero    = "REJECT_ZERO"
)

// QuantityPolicy is the Phase12-M2 buy-lot domain surface (pure; no I/O).
// M2.2 wires materialize behind EnableQuantityPolicy; Broker remains unwired (M2.3).
type QuantityPolicy interface {
	RuleKey() string
	MinBuyQty() int64
	NormalizeBuy(rawQty int64) BuyQuantityResult
}

// BuyQuantityResult is normalized buy qty + reason.
type BuyQuantityResult struct {
	NormalizedQty int64  `json:"normalized_qty"`
	Reason        string `json:"reason"`
	RawQty        int64  `json:"raw_qty"`
	RuleKey       string `json:"rule_key,omitempty"`
}

// OrderQuantityRules is a static buy quantity template (M2.1 buy-only fields).
type OrderQuantityRules struct {
	Key string
	QtyUnit string
	// BuyMinQty is the minimum acceptable buy quantity (e.g. MAIN 100, STAR 200).
	BuyMinQty int64
	// BuyStepQty is the alignment step when StepAfterMin is false (e.g. 100 or 10).
	BuyStepQty int64
	// StepAfterMin: when true (STAR), any qty >= BuyMinQty is legal at 1-share increments.
	StepAfterMin bool
}

// RuleKey implements QuantityPolicy.
func (r OrderQuantityRules) RuleKey() string {
	if r.Key == "" {
		return "CN:UNKNOWN"
	}
	return r.Key
}

// MinBuyQty implements QuantityPolicy.
func (r OrderQuantityRules) MinBuyQty() int64 {
	if r.BuyMinQty <= 0 {
		return 100
	}
	return r.BuyMinQty
}

// NormalizeBuy implements QuantityPolicy (pure buy adjust).
func (r OrderQuantityRules) NormalizeBuy(rawQty int64) BuyQuantityResult {
	return adjustBuyQty(r, rawQty)
}

// Ensure interface conformance.
var _ QuantityPolicy = OrderQuantityRules{}

// PolicyFromInstrumentMeta maps M1 QuantityMeta → static QuantityPolicy template.
// Does not read DB or trading paths.
func PolicyFromInstrumentMeta(meta instrument.QuantityMeta) QuantityPolicy {
	switch meta.SecurityType {
	case instrument.SecurityConvertibleBond:
		return TemplateConvertibleBond()
	case instrument.SecurityETF:
		return TemplateETF()
	case instrument.SecurityEquity:
		if meta.MarketSegment == instrument.BoardSTAR {
			return TemplateSTAR()
		}
		return TemplateMAIN()
	default:
		return TemplateMAIN() // fail-closed toward 100-lot
	}
}

// NormalizeBuyQuantity is the M2.1 entry: instrument metadata + raw qty → normalized + reason.
func NormalizeBuyQuantity(meta instrument.QuantityMeta, rawQty int64) BuyQuantityResult {
	return PolicyFromInstrumentMeta(meta).NormalizeBuy(rawQty)
}
