package portfolioinsight

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/rebalance"
)

// Build maps read-only sources into PortfolioInsight. Pure function; no I/O.
func Build(in Input) *PortfolioInsight {
	asOf := in.AsOf
	if asOf.IsZero() {
		if in.Risk != nil && !in.Risk.AsOf.IsZero() {
			asOf = in.Risk.AsOf
		} else {
			asOf = time.Now().UTC()
		}
	}
	tradeDate := in.TradeDate
	if tradeDate == "" && in.Risk != nil {
		tradeDate = in.Risk.TradeDate
	}

	gaps := []string{}
	holdings := in.Holdings
	if holdings == nil {
		holdings = []rules.HoldingDecision{}
	}

	summary := buildSummary(in.Risk, holdings, &gaps)
	whyReduce := buildWhyReduce(holdings)
	whyBuy := buildWhyBuyLimited(in.Risk, in.AllocationShadow, in.Rebalance, &gaps)

	if in.AllocationShadow == nil || !in.AllocationShadow.Present {
		// already gap'd inside why_buy; ensure listed
		gaps = appendUniqueGap(gaps, "shadow_disabled_or_missing")
	}
	if in.Rebalance == nil {
		gaps = appendUniqueGap(gaps, "rebalance_suggestion_missing")
	}
	if len(holdings) == 0 {
		gaps = appendUniqueGap(gaps, "holding_decisions_missing")
	}

	out := &PortfolioInsight{
		SchemaVersion:      SchemaVersion,
		AsOf:               asOf,
		TradeDate:          tradeDate,
		AccountID:          in.AccountID,
		RecordOnly:         true,
		NotTradingAdvice:   true,
		NotAutoTrade:       true,
		NotATradePlan:      true,
		NotExecution:       true,
		NotBuyChainWrite:   true,
		NotStrategyWrite:   true,
		Disclaimer:         DisclaimerZH,
		DisclaimerKey:      DisclaimerKey,
		PortfolioSummary:   summary,
		WhyReduce:          whyReduce,
		WhyBuyLimited:      whyBuy,
		DataGaps:           sortedUnique(gaps),
		SourcesFingerprint: fingerprint(in.Risk, holdings, in.AllocationShadow, in.Rebalance),
	}
	return out
}

func appendUniqueGap(gaps []string, g string) []string {
	for _, x := range gaps {
		if x == g {
			return gaps
		}
	}
	return append(gaps, g)
}

func fingerprint(
	risk *portfoliorisk.PortfolioRiskSnapshot,
	holdings []rules.HoldingDecision,
	shadow *AllocationShadowView,
	reb *rebalance.RebalanceSuggestion,
) string {
	type fp struct {
		RiskFP   string
		Holdings []string
		ShadowFP string
		RebFP    string
	}
	h := make([]string, 0, len(holdings))
	for _, d := range holdings {
		h = append(h, d.Symbol+"|"+d.FinalAction+"|"+d.Action)
		for _, c := range d.ReasonCodes {
			h = append(h, c)
		}
	}
	payload := fp{Holdings: h}
	if risk != nil {
		payload.RiskFP = risk.InputsFingerprint
	}
	if shadow != nil {
		payload.ShadowFP = shadow.ShadowFingerprint
	}
	if reb != nil {
		payload.RebFP = reb.InputsFingerprint
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}
