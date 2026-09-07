package strategy

import (
	"math"

	"go-stock/backend/logger"
	"go-stock/backend/tradingrule"
)

// QuantityShadowObservation is a read-only compare of legacy /100 vs QuantityPolicy.
// Phase12-M2.2.5: never writes target_volume; never enables EnableQuantityPolicy.
type QuantityShadowObservation struct {
	Instrument     string `json:"instrument"`
	OldQuantity    int64  `json:"old_quantity"`
	PolicyQuantity int64  `json:"policy_quantity"`
	QuantityDiff   int64  `json:"quantity_diff"` // policy - old
	Reason         string `json:"reason"`         // policy normalize reason
	MatchStatus    string `json:"match_status,omitempty"`
	RuleKey        string `json:"rule_key,omitempty"`
	SecurityType   string `json:"security_type,omitempty"`
	MarketSegment  string `json:"market_segment,omitempty"`
	RawQuantity    int64  `json:"raw_quantity,omitempty"`
}

const (
	shadowMatch        = "MATCH"
	shadowDiffIncrease = "DIFF_INCREASE" // policy > old
	shadowDiffDecrease = "DIFF_DECREASE" // policy < old (watch for abnormal cut)
)

// ObserveQuantityPolicyShadow compares legacy lot math vs Policy for one sizing input.
// Pure observation: does not mutate plans, flags, or Broker.
func ObserveQuantityPolicyShadow(stockCode string, effectiveAmount, limitPrice float64) QuantityShadowObservation {
	meta := morningQuantityMeta(stockCode)
	oldQty := calcMorningLotVolume(effectiveAmount, limitPrice)

	var rawQty int64
	var policyRes tradingrule.BuyQuantityResult
	if effectiveAmount > 0 && limitPrice > 0 {
		rawQty = int64(math.Floor(effectiveAmount / limitPrice))
		policyRes = tradingrule.NormalizeBuyQuantity(meta, rawQty)
	} else {
		policyRes = tradingrule.NormalizeBuyQuantity(meta, 0)
	}

	diff := policyRes.NormalizedQty - oldQty
	return QuantityShadowObservation{
		Instrument:     stockCode,
		OldQuantity:    oldQty,
		PolicyQuantity: policyRes.NormalizedQty,
		QuantityDiff:   diff,
		Reason:         policyRes.Reason,
		RuleKey:        policyRes.RuleKey,
		SecurityType:   string(meta.SecurityType),
		MarketSegment:  meta.MarketSegment,
		RawQuantity:    rawQty,
		MatchStatus:    classifyQuantityShadow(oldQty, policyRes.NormalizedQty),
	}
}

func classifyQuantityShadow(oldQty, policyQty int64) string {
	if oldQty == policyQty {
		return shadowMatch
	}
	if policyQty > oldQty {
		return shadowDiffIncrease
	}
	return shadowDiffDecrease
}

// IsAbnormalQuantityDecrease reports policy cut below legacy when legacy had a positive lot.
func (o QuantityShadowObservation) IsAbnormalQuantityDecrease() bool {
	return o.MatchStatus == shadowDiffDecrease && o.OldQuantity > 0
}

func logQuantityPolicyShadow(obs QuantityShadowObservation) {
	logger.SugaredLogger.Infof(
		"QuantityPolicyShadow instrument=%s old=%d policy=%d diff=%d reason=%s match=%s rule=%s type=%s board=%s raw=%d",
		obs.Instrument, obs.OldQuantity, obs.PolicyQuantity, obs.QuantityDiff,
		obs.Reason, obs.MatchStatus, obs.RuleKey, obs.SecurityType, obs.MarketSegment, obs.RawQuantity,
	)
}
