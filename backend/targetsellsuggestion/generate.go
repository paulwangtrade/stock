package targetsellsuggestion

import (
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/portfoliotarget"
	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
)

// Generate computes Target-aware SELL suggestions (suggest_only).
// Default Options.Enabled=false → skipped. Never mutates Holdings / TradePlan / Execution.
func Generate(in Input) *Report {
	opt := in.Options
	opt.AllocOptions = normalizeAlloc(opt.AllocOptions)
	opt.RebalanceOptions.SuggestOnly = true
	if opt.MinDeltaWeight > 0 {
		opt.RebalanceOptions.MinDeltaWeight = opt.MinDeltaWeight
	}

	rep := emptyReport(in, opt)
	if !opt.Enabled {
		rep.Skipped = true
		rep.SkipReason = "target_sell_suggestion_disabled"
		rep.Notes = append(rep.Notes, "DefaultEnabled=false; pass Options.Enabled=true to generate")
		return rep
	}

	cur, equity := toCurrentPortfolio(in)
	if !cur.Found || equity <= 0 {
		rep.Notes = append(rep.Notes, ReasonDataMissing)
		return rep
	}

	tgt := resolveTarget(in, equity)
	if len(tgt.Positions) == 0 && len(in.Holdings) == 0 {
		rep.Notes = append(rep.Notes, "empty_target_and_holdings")
		return rep
	}

	reb := rebalance.Calculate(rebalance.RebalanceInput{
		AsOf:      firstTime(in.AsOf),
		TradeDate: strings.TrimSpace(in.TradeDate),
		Current:   cur,
		Target:    tgt,
		Options:   opt.RebalanceOptions,
	})
	if reb != nil {
		rep.RebalanceFingerprint = reb.InputsFingerprint
	}

	sellTgt := sellTargetFromRebalance(&tgt)
	posBy := indexHoldings(in.Holdings)

	// Process EXIT then REDUCE from H4 (SELL delta only; ignore BUY).
	seen := map[string]struct{}{}
	if reb != nil {
		for _, e := range reb.Exits {
			sym := normSym(e.Symbol)
			if sym == "" {
				continue
			}
			seen[sym] = struct{}{}
			sug := buildSuggestion(sym, e.CurrentWeight, 0, ActionExit, e.Reason, e.ReasonCodes, posBy[sym], equity, sellTgt, opt)
			if sug.SuggestQty > 0 || opt.IncludeAtTarget || sug.WeightGap > 0 {
				appendSuggestion(rep, sug)
			}
		}
		for _, r := range reb.Reduces {
			sym := normSym(r.Symbol)
			if sym == "" {
				continue
			}
			seen[sym] = struct{}{}
			sug := buildSuggestion(sym, r.CurrentWeight, r.TargetWeight, ActionReduce, r.Reason, r.ReasonCodes, posBy[sym], equity, sellTgt, opt)
			if sug.SuggestQty > 0 || opt.IncludeAtTarget || sug.WeightGap > minGap(opt) {
				appendSuggestion(rep, sug)
			}
		}
		if opt.IncludeAtTarget {
			for _, sk := range reb.Skipped {
				sym := normSym(sk.Symbol)
				if sym == "" {
					continue
				}
				if _, ok := seen[sym]; ok {
					continue
				}
				pos := posBy[sym]
				tw := lookupTargetWeight(&tgt, sym)
				cw := pos.Weight
				if cw <= 0 && equity > 0 && pos.MarketValue > 0 {
					cw = pos.MarketValue / equity
				}
				gap := cw - tw
				if math.Abs(gap) >= minGap(opt) {
					continue
				}
				sug := TargetSellSuggestion{
					Symbol: sym, CurrentWeight: cw, TargetWeight: tw, WeightGap: gap,
					SuggestQty: 0, Reason: ReasonAtTarget, Action: "",
					ReasonCodes: append([]string{}, sk.ReasonCodes...),
					AvailableQty: pos.AvailableQty,
					RecordOnly: true, SuggestOnly: true, NotAnOrder: true,
					NotSellTradePlan: true, NotExecution: true, NotHoldingsMutate: true,
				}
				appendSuggestion(rep, sug)
			}
		}
	}

	sort.Slice(rep.Suggestions, func(i, j int) bool {
		if rep.Suggestions[i].WeightGap != rep.Suggestions[j].WeightGap {
			return rep.Suggestions[i].WeightGap > rep.Suggestions[j].WeightGap
		}
		return rep.Suggestions[i].Symbol < rep.Suggestions[j].Symbol
	})
	finalizeSummary(rep)
	return rep
}

func buildSuggestion(
	sym string,
	currentW, targetW float64,
	action, reason string,
	codes []string,
	pos HoldingPosition,
	equity float64,
	sellTgt *sellallocation.TargetPortfolio,
	opt Options,
) TargetSellSuggestion {
	if targetW < 0 {
		targetW = 0
	}
	if action == ActionExit {
		targetW = 0
	}
	gap := currentW - targetW
	if gap < 0 {
		gap = 0
	}

	sug := TargetSellSuggestion{
		Symbol:            sym,
		CurrentWeight:     currentW,
		TargetWeight:      targetW,
		WeightGap:         gap,
		Reason:            strings.TrimSpace(reason),
		Action:            action,
		ReasonCodes:       append([]string{}, codes...),
		AvailableQty:      pos.AvailableQty,
		RecordOnly:        true,
		SuggestOnly:       true,
		NotAnOrder:        true,
		NotSellTradePlan:  true,
		NotExecution:      true,
		NotHoldingsMutate: true,
	}
	if sug.Reason == "" {
		if action == ActionExit {
			sug.Reason = ReasonTargetZero
		} else {
			sug.Reason = ReasonOverweight
		}
	}

	allocPos := sellallocation.CurrentPosition{
		Symbol:       sym,
		TotalQty:     pos.Qty,
		AvailableQty: pos.AvailableQty,
		LockedQty:    pos.LockedQty,
		MarketValue:  pos.MarketValue,
		Weight:       currentW,
		MarkPrice:    pos.MarkPrice,
		CanSell:      pos.CanSell,
	}
	if allocPos.TotalQty <= 0 && pos.Qty > 0 {
		allocPos.TotalQty = pos.Qty
	}
	ratio := 0.0
	if currentW > 1e-12 {
		ratio = gap / currentW
	}
	if ratio > 1 {
		ratio = 1
	}
	if action == ActionExit {
		ratio = 1
	}
	intent := sellallocation.SellIntent{
		Symbol:               sym,
		Action:               action,
		TargetPositionWeight: targetW,
		TargetReduceRatio:    ratio,
		Reason:               sug.Reason,
		ReasonCodes:          append([]string{}, codes...),
		RecordOnly:           true,
		NotAnOrder:           true,
	}
	alloc := sellallocation.Allocate(sellallocation.Input{
		Intent:   intent,
		Position: allocPos,
		Target:   sellTgt,
		Equity:   equity,
		Options:  opt.AllocOptions,
	})
	if alloc != nil {
		sug.SuggestQty = alloc.SuggestedSellQty
		sug.Binding = alloc.Binding
		if alloc.TargetWeight >= 0 {
			sug.TargetWeight = alloc.TargetWeight
			sug.WeightGap = currentW - alloc.TargetWeight
			if sug.WeightGap < 0 {
				sug.WeightGap = 0
			}
		}
		for _, c := range alloc.ReasonCodes {
			sug.ReasonCodes = appendUnique(sug.ReasonCodes, c)
		}
	}
	return sug
}

func emptyReport(in Input, opt Options) *Report {
	return &Report{
		SchemaVersion:     SchemaVersion,
		AsOf:              firstTime(in.AsOf),
		TradeDate:         strings.TrimSpace(in.TradeDate),
		AccountID:         strings.TrimSpace(in.AccountID),
		Enabled:           opt.Enabled,
		SuggestOnly:       true,
		RecordOnly:        true,
		NotASellTradePlan: true,
		NotExecution:      true,
		NotHoldingsMutate: true,
		NotAutoTrade:      true,
		Suggestions:       []TargetSellSuggestion{},
		DataSourceNote:    dataSourceNote,
	}
}

func toCurrentPortfolio(in Input) (rebalance.CurrentPortfolio, float64) {
	equity := in.Equity
	if equity <= 0 {
		var mv float64
		for _, h := range in.Holdings {
			mv += h.MarketValue
		}
		equity = mv + in.Cash
	}
	out := rebalance.CurrentPortfolio{
		Found:     len(in.Holdings) > 0 || equity > 0,
		Equity:    equity,
		Cash:      in.Cash,
		Exposure:  in.Exposure,
		Positions: []rebalance.CurrentPortfolioPos{},
		Source:    rebalance.SourceSnapshot,
	}
	if out.Exposure <= 0 {
		var mv float64
		for _, h := range in.Holdings {
			mv += h.MarketValue
		}
		out.Exposure = mv
	}
	for _, h := range in.Holdings {
		sym := normSym(h.Symbol)
		if sym == "" {
			continue
		}
		w := h.Weight
		if w <= 0 && equity > 0 && h.MarketValue > 0 {
			w = h.MarketValue / equity
		}
		out.Positions = append(out.Positions, rebalance.CurrentPortfolioPos{
			Symbol: sym, Qty: h.Qty, MarketValue: h.MarketValue, Weight: w,
		})
	}
	if len(out.Positions) == 0 && equity <= 0 {
		out.Found = false
	}
	return out, equity
}

func resolveTarget(in Input, equity float64) rebalance.TargetPortfolio {
	if in.RebalanceTarget != nil {
		t := *in.RebalanceTarget
		if t.EquityRef <= 0 {
			t.EquityRef = equity
		}
		return t
	}
	if in.Target != nil {
		t := in.Target.ToRebalanceTarget()
		if t.EquityRef <= 0 {
			t.EquityRef = equity
		}
		return t
	}
	return rebalance.TargetPortfolio{EquityRef: equity, Positions: []rebalance.TargetPosition{}}
}

func sellTargetFromRebalance(tgt *rebalance.TargetPortfolio) *sellallocation.TargetPortfolio {
	if tgt == nil {
		return nil
	}
	out := &sellallocation.TargetPortfolio{
		EquityRef: tgt.EquityRef,
		Names:     make([]sellallocation.TargetName, 0, len(tgt.Positions)),
	}
	for _, p := range tgt.Positions {
		sym := normSym(p.Symbol)
		if sym == "" {
			continue
		}
		out.Names = append(out.Names, sellallocation.TargetName{
			Symbol: sym, TargetWeight: p.TargetWeight,
		})
	}
	return out
}

func indexHoldings(hs []HoldingPosition) map[string]HoldingPosition {
	out := map[string]HoldingPosition{}
	for _, h := range hs {
		sym := normSym(h.Symbol)
		if sym == "" {
			continue
		}
		h.Symbol = sym
		out[sym] = h
	}
	return out
}

func lookupTargetWeight(tgt *rebalance.TargetPortfolio, sym string) float64 {
	if tgt == nil {
		return 0
	}
	for _, p := range tgt.Positions {
		if normSym(p.Symbol) == sym {
			return p.TargetWeight
		}
	}
	return 0
}

func normalizeAlloc(opt sellallocation.Options) sellallocation.Options {
	d := sellallocation.DefaultOptions()
	opt.Phase = sellallocation.PhaseSuggestOnly
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
		opt.PreferWeightVsRatio = sellallocation.PreferWeightOnly
	}
	return opt
}

func minGap(opt Options) float64 {
	if opt.MinDeltaWeight > 0 {
		return opt.MinDeltaWeight
	}
	if opt.RebalanceOptions.MinDeltaWeight > 0 {
		return opt.RebalanceOptions.MinDeltaWeight
	}
	return rebalance.DefaultCalculateOptions().MinDeltaWeight
}

func appendSuggestion(rep *Report, sug TargetSellSuggestion) {
	rep.Suggestions = append(rep.Suggestions, sug)
}

func finalizeSummary(rep *Report) {
	s := Summary{SuggestionCount: len(rep.Suggestions)}
	for _, sug := range rep.Suggestions {
		s.TotalSuggestQty += sug.SuggestQty
		s.TotalWeightGap += sug.WeightGap
		switch sug.Action {
		case ActionReduce:
			s.ReduceCount++
		case ActionExit:
			s.ExitCount++
		}
	}
	rep.Summary = s
}

func normSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func firstTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

func appendUnique(codes []string, add string) []string {
	add = strings.TrimSpace(add)
	if add == "" {
		return codes
	}
	for _, c := range codes {
		if c == add {
			return codes
		}
	}
	return append(codes, add)
}

// FromTargetPortfolio is a thin alias helper for callers holding portfoliotarget.
func FromTargetPortfolio(t *portfoliotarget.TargetPortfolio) *rebalance.TargetPortfolio {
	if t == nil {
		return nil
	}
	rb := t.ToRebalanceTarget()
	return &rb
}
