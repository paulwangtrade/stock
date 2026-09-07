package sellallocation

import (
	"math"
	"strings"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
)

// Allocate computes suggest-only SellAllocation from Intent + position + optional risk/target.
// Never creates SellTradePlan or calls Execution.
func Allocate(in Input) *SellAllocation {
	opt := normalizeOptions(in.Options)
	out := emptyAlloc(in.Position.Symbol)
	pos := in.Position
	out.CurrentQty = pos.TotalQty
	out.AvailableQty = pos.AvailableQty
	out.CurrentWeight = pos.Weight

	intent := in.Intent
	action := strings.ToUpper(strings.TrimSpace(intent.Action))
	if action != ActionReduce && action != ActionExit {
		out.Binding = BindingHoldSkip
		out.SkippedReason = ReasonActionHold
		out.Reason = ReasonActionHold
		out.ReasonCodes = []string{ReasonActionHold}
		out.TargetWeight = intent.TargetPositionWeight
		return out
	}
	out.Action = action
	out.Reason = strings.TrimSpace(intent.Reason)
	out.ReasonCodes = append([]string{}, intent.ReasonCodes...)
	if out.Reason == "" {
		out.Reason = ReasonFromDecision
	}

	if pos.TotalQty <= 0 && pos.Weight <= 0 && pos.MarketValue <= 0 {
		out.Binding = BindingDataMissing
		out.SkippedReason = ReasonDataMissing
		out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonDataMissing)
		return out
	}

	// Optional: TargetPortfolio refines REDUCE target weight.
	tw := intent.TargetPositionWeight
	if action == ActionReduce {
		if twTgt, ok := LookupTargetWeight(in.Target, pos.Symbol); ok {
			if tw <= 0 && twTgt >= 0 {
				tw = twTgt
			} else if twTgt >= 0 && twTgt < tw {
				tw = twTgt
			}
		}
	}
	if action == ActionExit {
		tw = 0
	}
	out.TargetWeight = tw

	wc := pos.Weight
	eps := opt.EpsilonWeight
	if wc <= tw+eps && action == ActionReduce {
		out.Binding = BindingAtTarget
		out.SkippedReason = ReasonAlreadyAtTarget
		out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonAlreadyAtTarget)
		v := tw
		out.TargetPositionWeightAfter = &v
		return out
	}

	ratio := intent.TargetReduceRatio
	if action == ActionExit {
		ratio = 1
	}
	if ratio <= 0 {
		ratio = opt.DefaultReduceRatio
	}
	if ratio > 1 {
		ratio = 1
	}

	// Risk boost (tighten sell) when enabled.
	over := in.Overshoot
	if over == nil {
		over = DeriveOvershoot(in.PortfolioRisk, wc, "")
	}
	riskBoostQty := int64(0)
	if opt.RiskBoostEnabled && action == ActionReduce {
		boosted, note := applyRiskBoost(ratio, over)
		if boosted > ratio {
			ratio = boosted
			out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonRiskBoost)
			if note != "" {
				out.ReasonCodes = appendUnique(out.ReasonCodes, note)
			}
		}
	}

	equity := in.Equity
	if equity <= 0 && pos.Weight > 0 && pos.MarketValue > 0 {
		equity = pos.MarketValue / pos.Weight
	}

	intentQty := intentQtyRaw(action, pos, tw, ratio, equity, opt.PreferWeightVsRatio)
	out.Caps.IntentQtyCap = intentQty
	if opt.RiskBoostEnabled && action == ActionReduce && over != nil && over.Severity == SeverityHigh {
		// ensure at least intent after boost tracked
		riskBoostQty = intentQty
		out.Caps.RiskBoostQty = riskBoostQty
	}

	avail := pos.AvailableQty
	if avail < 0 {
		avail = 0
	}
	out.Caps.AvailableQtyCap = avail

	qty := intentQty
	binding := BindingOK
	if !pos.CanSell {
		qty = 0
		binding = BindingT1
		out.SkippedReason = ReasonT1Locked
		out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonT1Locked)
	} else {
		if qty > avail {
			qty = avail
			binding = BindingT1
			out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonT1Clip)
		}
	}

	lot := opt.LotSize
	if lot <= 0 {
		lot = DefaultLotSize
	}
	rounded := floorLot(qty, lot)
	out.Caps.LotRounded = rounded
	if rounded < qty && qty > 0 {
		out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonLotRound)
	}
	qty = rounded

	minQty := opt.MinSellQty
	if minQty <= 0 {
		minQty = lot
	}
	if qty > 0 && qty < minQty {
		qty = 0
		binding = BindingBelowMin
		out.SkippedReason = ReasonBelowMin
		out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonBelowMin)
	}

	if opt.MinSellNotional > 0 && qty > 0 {
		mark := pos.MarkPrice
		if mark <= 0 && pos.TotalQty > 0 {
			mark = pos.MarketValue / float64(pos.TotalQty)
		}
		if mark > 0 && float64(qty)*mark < opt.MinSellNotional {
			qty = 0
			binding = BindingBelowMin
			out.SkippedReason = ReasonBelowMin
			out.ReasonCodes = appendUnique(out.ReasonCodes, ReasonBelowMin)
		}
	}

	if qty == 0 && binding == BindingOK {
		binding = BindingIntent
	}

	out.SuggestedSellQty = qty
	out.Binding = binding
	out.SuggestedReduceRatioEffective = 0
	if pos.TotalQty > 0 {
		out.SuggestedReduceRatioEffective = float64(qty) / float64(pos.TotalQty)
	}
	mark := pos.MarkPrice
	if mark <= 0 && pos.TotalQty > 0 && pos.MarketValue > 0 {
		mark = pos.MarketValue / float64(pos.TotalQty)
	}
	if mark > 0 && qty > 0 {
		out.SuggestedSellNotional = float64(qty) * mark
	}
	if pos.TotalQty > 0 && equity > 0 {
		remain := pos.TotalQty - qty
		if remain < 0 {
			remain = 0
		}
		// approximate post weight
		postW := wc
		if pos.TotalQty > 0 {
			postW = wc * (float64(remain) / float64(pos.TotalQty))
		}
		out.TargetPositionWeightAfter = &postW
	} else {
		v := tw
		out.TargetPositionWeightAfter = &v
	}
	out.ExecutableHint = pos.CanSell && avail > 0 && qty > 0
	return out
}

// AllocateFromDecision is a convenience: Decision + Position (+ optional risk/target) → SellAllocation.
func AllocateFromDecision(
	decision rules.HoldingDecision,
	pos CurrentPosition,
	risk *portfoliorisk.PortfolioRiskSnapshot,
	target *TargetPortfolio,
	equity float64,
	opt Options,
) *SellAllocation {
	opt = normalizeOptions(opt)
	intent, ok := ProjectIntentFromRules(decision, pos.Weight, opt)
	if !ok {
		out := emptyAlloc(pos.Symbol)
		out.CurrentQty = pos.TotalQty
		out.AvailableQty = pos.AvailableQty
		out.CurrentWeight = pos.Weight
		out.Binding = BindingHoldSkip
		out.SkippedReason = ReasonActionHold
		out.Reason = ReasonActionHold
		out.ReasonCodes = []string{ReasonActionHold}
		return out
	}
	return Allocate(Input{
		Intent:        intent,
		Position:      pos,
		PortfolioRisk: risk,
		Target:        target,
		Equity:        equity,
		Options:       opt,
	})
}

func emptyAlloc(symbol string) *SellAllocation {
	return &SellAllocation{
		SchemaVersion:    SchemaVersion,
		Phase:            PhaseSuggestOnly,
		SuggestOnly:      true,
		Symbol:           strings.TrimSpace(symbol),
		RecordOnly:       true,
		NotAnOrder:       true,
		NotExecution:     true,
		NotTradePlan:     true,
		NotBuyChain:      true,
		PersistSellPlans: false,
		Binding:          BindingOK,
	}
}

func normalizeOptions(opt Options) Options {
	d := DefaultOptions()
	opt.Phase = PhaseSuggestOnly
	if opt.LotSize <= 0 {
		opt.LotSize = d.LotSize
	}
	if opt.EpsilonWeight <= 0 {
		opt.EpsilonWeight = d.EpsilonWeight
	}
	if opt.DefaultReduceRatio <= 0 || opt.DefaultReduceRatio >= 1 {
		opt.DefaultReduceRatio = d.DefaultReduceRatio
	}
	if strings.TrimSpace(opt.PreferWeightVsRatio) == "" {
		opt.PreferWeightVsRatio = d.PreferWeightVsRatio
	}
	return opt
}

func intentQtyRaw(action string, pos CurrentPosition, tw, ratio, equity float64, prefer string) int64 {
	if action == ActionExit {
		if pos.TotalQty > 0 {
			return pos.TotalQty
		}
		return 0
	}
	wc := pos.Weight
	dw := wc - tw
	if dw < 0 {
		dw = 0
	}

	var qtyWeight, qtyRatio int64
	mark := pos.MarkPrice
	if mark <= 0 && pos.TotalQty > 0 && pos.MarketValue > 0 {
		mark = pos.MarketValue / float64(pos.TotalQty)
	}
	if equity > 0 && mark > 0 && dw > 0 {
		qtyWeight = int64(math.Floor((dw * equity / mark) + 1e-9))
	} else if pos.TotalQty > 0 && wc > 1e-12 && dw > 0 {
		qtyWeight = int64(math.Floor(float64(pos.TotalQty)*(dw/wc) + 1e-9))
	}
	if pos.TotalQty > 0 && ratio > 0 {
		qtyRatio = int64(math.Floor(float64(pos.TotalQty)*ratio + 1e-9))
	}

	switch prefer {
	case PreferWeightOnly:
		return qtyWeight
	case PreferRatioOnly:
		return qtyRatio
	case PreferMax:
		if qtyWeight > qtyRatio {
			return qtyWeight
		}
		return qtyRatio
	default: // min — conservative
		if qtyWeight <= 0 {
			return qtyRatio
		}
		if qtyRatio <= 0 {
			return qtyWeight
		}
		if qtyWeight < qtyRatio {
			return qtyWeight
		}
		return qtyRatio
	}
}

func floorLot(qty, lot int64) int64 {
	if qty <= 0 || lot <= 0 {
		return 0
	}
	return (qty / lot) * lot
}

func applyRiskBoost(ratio float64, over *RiskOvershootFact) (float64, string) {
	if over == nil {
		return ratio, ""
	}
	sev := strings.ToLower(strings.TrimSpace(over.Severity))
	if sev == "" {
		sev = SeverityNone
		if over.NameOverCapRatio != nil && *over.NameOverCapRatio > 0.25 {
			sev = SeverityHigh
		} else if over.NameOverCapRatio != nil && *over.NameOverCapRatio > 0 {
			sev = SeverityMild
		} else if over.GrossHeadroom != nil && *over.GrossHeadroom <= 0 {
			sev = SeverityMild
		}
	}
	switch sev {
	case SeverityHigh:
		r := ratio + DefaultRiskBoostHighExtra
		if r > 1 {
			r = 1
		}
		return r, SeverityHigh
	case SeverityMild:
		r := ratio + DefaultRiskBoostMildExtra
		if r > 1 {
			r = 1
		}
		return r, SeverityMild
	default:
		return ratio, ""
	}
}

// DeriveOvershoot builds a RiskOvershootFact from PortfolioRiskSnapshot when concentration is available.
func DeriveOvershoot(risk *portfoliorisk.PortfolioRiskSnapshot, weight float64, _ string) *RiskOvershootFact {
	out := &RiskOvershootFact{Severity: SeverityNone}
	if risk == nil || !risk.Found {
		return out
	}
	if risk.Concentration.Available && risk.Concentration.CapSingle != nil && *risk.Concentration.CapSingle > 0 {
		cap := *risk.Concentration.CapSingle
		if weight > cap {
			r := weight/cap - 1
			out.NameOverCapRatio = &r
			if r > 0.25 {
				out.Severity = SeverityHigh
			} else {
				out.Severity = SeverityMild
			}
		}
	}
	if risk.Exposure.Available && risk.Exposure.HeadroomVsCap != nil {
		h := *risk.Exposure.HeadroomVsCap
		out.GrossHeadroom = &h
		if h <= 0 && out.Severity == SeverityNone {
			out.Severity = SeverityMild
		}
	}
	return out
}

func appendUnique(codes []string, add ...string) []string {
	seen := map[string]struct{}{}
	for _, c := range codes {
		seen[c] = struct{}{}
	}
	for _, a := range add {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		codes = append(codes, a)
		seen[a] = struct{}{}
	}
	return codes
}
