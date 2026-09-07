package sellsuggestion

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
)

// Generate produces after-close SellSuggestion rows (suggest_only).
// Default Options.Enabled=false → skipped report with empty suggestions.
// Never creates SellTradePlan, never calls Execution or Broker.
func Generate(in Input) *Report {
	opt := in.Options
	opt.DecisionPolicy.PersistSellPlans = false
	opt.AllocOptions = normalizeAlloc(opt.AllocOptions)
	opt.RebalanceOptions.SuggestOnly = true

	rep := emptyReport(in, opt)
	if !opt.Enabled {
		rep.Skipped = true
		rep.SkipReason = "sell_suggestion_disabled"
		rep.Notes = append(rep.Notes, "DefaultEnabled=false; pass Options.Enabled=true to generate suggestions")
		return rep
	}

	decisions := in.PrecomputedDecisions
	if decisions == nil {
		obs := in.Observation
		obs.AsOf = firstTime(obs.AsOf, in.AsOf)
		if strings.TrimSpace(obs.TradeDate) == "" {
			obs.TradeDate = in.TradeDate
		}
		obs.Policy = opt.DecisionPolicy
		obs.Policy.PersistSellPlans = false
		if in.PortfolioRisk != nil && obs.PortfolioRisk == nil {
			obs.PortfolioRisk = in.PortfolioRisk
		}
		decisions = holdingdecision.Observe(obs)
	}
	if decisions == nil {
		rep.Notes = append(rep.Notes, "holding_decision_empty")
		return rep
	}
	rep.DecisionFingerprint = decisions.InputsFingerprint

	rebIndex := map[string]rebAnnot{}
	if opt.AttachRebalance {
		reb := runRebalanceContrast(in, opt)
		if reb != nil {
			rep.RebalanceAttached = true
			rep.RebalanceFingerprint = reb.InputsFingerprint
			rebIndex = indexRebalance(reb)
		} else {
			rep.Notes = append(rep.Notes, "rebalance_contrast_skipped")
		}
	}

	equity := in.Equity
	positions := in.Positions
	if positions == nil {
		positions = map[string]sellallocation.CurrentPosition{}
	}

	for _, d := range decisions.Decisions {
		sym := normSym(d.Symbol)
		if sym == "" {
			continue
		}
		action := strings.ToUpper(strings.TrimSpace(d.Action))
		pos := lookupPos(positions, sym, d.Weight)

		if action != ActionReduce && action != ActionExit {
			if !opt.IncludeHold {
				continue
			}
			appendSuggestion(rep, holdSuggestion(d, pos))
			continue
		}

		alloc := allocateFromAction(d, pos, in.PortfolioRisk, in.SellTarget, equity, opt.AllocOptions)
		appendSuggestion(rep, fromAllocation(d, pos, alloc, rebIndex[sym]))
	}

	sort.Slice(rep.Suggestions, func(i, j int) bool {
		return rep.Suggestions[i].Symbol < rep.Suggestions[j].Symbol
	})
	finalizeSummary(rep)
	return rep
}

func emptyReport(in Input, opt Options) *Report {
	asOf := in.AsOf
	if asOf.IsZero() && !in.Observation.AsOf.IsZero() {
		asOf = in.Observation.AsOf
	}
	return &Report{
		SchemaVersion:     SchemaVersion,
		AsOf:              asOf,
		TradeDate:         strings.TrimSpace(in.TradeDate),
		AccountID:         strings.TrimSpace(in.AccountID),
		Enabled:           opt.Enabled,
		SuggestOnly:       true,
		RecordOnly:        true,
		NotASellTradePlan: true,
		NotExecution:      true,
		NotBroker:         true,
		NotBuyChain:       true,
		NotAutoTrade:      true,
		PersistSellPlans:  false,
		Suggestions:       []SellSuggestion{},
		ByAction: map[string]int{
			ActionHold:   0,
			ActionReduce: 0,
			ActionExit:   0,
		},
		DataSourceNote: dataSourceNote,
	}
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
		opt.PreferWeightVsRatio = d.PreferWeightVsRatio
	}
	return opt
}

func lookupPos(positions map[string]sellallocation.CurrentPosition, sym string, weight float64) sellallocation.CurrentPosition {
	if pos, ok := positions[sym]; ok {
		pos.Symbol = sym
		return pos
	}
	for k, v := range positions {
		if normSym(k) == sym {
			v.Symbol = sym
			return v
		}
	}
	return sellallocation.CurrentPosition{
		Symbol:  sym,
		Weight:  weight,
		CanSell: false,
	}
}

func allocateFromAction(
	d holdingdecision.ActionDecision,
	pos sellallocation.CurrentPosition,
	risk *portfoliorisk.PortfolioRiskSnapshot,
	target *sellallocation.TargetPortfolio,
	equity float64,
	opt sellallocation.Options,
) *sellallocation.SellAllocation {
	intent, ok := sellallocation.ProjectIntentFromAction(d, pos.Weight, opt)
	if !ok {
		// Fallback: map ActionDecision into rules.HoldingDecision for AllocateFromDecision.
		rd := rules.HoldingDecision{
			Symbol:      d.Symbol,
			Action:      d.Action,
			FinalAction: d.Action,
			Explanation: d.Summary,
			ReasonCodes: d.ReasonCodes,
		}
		return sellallocation.AllocateFromDecision(rd, pos, risk, target, equity, opt)
	}
	return sellallocation.Allocate(sellallocation.Input{
		Intent:        intent,
		Position:      pos,
		PortfolioRisk: risk,
		Target:        target,
		Equity:        equity,
		Options:       opt,
	})
}

func holdSuggestion(d holdingdecision.ActionDecision, pos sellallocation.CurrentPosition) SellSuggestion {
	reason := strings.TrimSpace(d.Summary)
	if reason == "" {
		reason = strings.TrimSpace(d.Reason)
	}
	if reason == "" && len(d.ReasonCodes) > 0 {
		reason = d.ReasonCodes[0]
	}
	if reason == "" {
		reason = ActionHold
	}
	return SellSuggestion{
		Symbol:          normSym(d.Symbol),
		Action:          ActionHold,
		Reason:          reason,
		CurrentPosition: toPosView(pos),
		SuggestSellQty:  0,
		TargetWeight:    pos.Weight,
		RiskReason:      joinRisk(d.ReasonCodes, nil, ""),
		ReasonCodes:     append([]string{}, d.ReasonCodes...),
		RecordOnly:      true,
		NotAnOrder:      true,
		NotTradePlan:    true,
		NotExecution:    true,
		NotBroker:       true,
		SuggestOnly:     true,
	}
}

func fromAllocation(
	d holdingdecision.ActionDecision,
	pos sellallocation.CurrentPosition,
	alloc *sellallocation.SellAllocation,
	reb rebAnnot,
) SellSuggestion {
	if alloc == nil {
		alloc = &sellallocation.SellAllocation{
			Action: strings.ToUpper(strings.TrimSpace(d.Action)),
			Reason: d.Reason,
		}
	}
	reason := strings.TrimSpace(alloc.Reason)
	if reason == "" {
		reason = strings.TrimSpace(d.Summary)
	}
	if reason == "" {
		reason = strings.TrimSpace(d.Reason)
	}
	codes := append([]string{}, alloc.ReasonCodes...)
	for _, c := range d.ReasonCodes {
		codes = appendUnique(codes, c)
	}
	return SellSuggestion{
		Symbol:            normSym(d.Symbol),
		Action:            firstNonEmpty(alloc.Action, strings.ToUpper(strings.TrimSpace(d.Action))),
		Reason:            reason,
		CurrentPosition:   toPosView(pos),
		SuggestSellQty:    alloc.SuggestedSellQty,
		TargetWeight:      alloc.TargetWeight,
		RiskReason:        joinRisk(codes, alloc, reb.risk),
		ReasonCodes:       codes,
		Binding:           alloc.Binding,
		ExecutableHint:    alloc.ExecutableHint,
		RebalanceContrast: reb.note,
		RecordOnly:        true,
		NotAnOrder:        true,
		NotTradePlan:      true,
		NotExecution:      true,
		NotBroker:         true,
		SuggestOnly:       true,
	}
}

func toPosView(pos sellallocation.CurrentPosition) CurrentPositionView {
	return CurrentPositionView{
		TotalQty:     pos.TotalQty,
		AvailableQty: pos.AvailableQty,
		LockedQty:    pos.LockedQty,
		MarketValue:  pos.MarketValue,
		Weight:       pos.Weight,
		CanSell:      pos.CanSell,
		HoldingDays:  pos.HoldingDays,
	}
}

func appendSuggestion(rep *Report, sug SellSuggestion) {
	rep.Suggestions = append(rep.Suggestions, sug)
	if _, ok := rep.ByAction[sug.Action]; ok {
		rep.ByAction[sug.Action]++
	}
}

func finalizeSummary(rep *Report) {
	s := Summary{SuggestionCount: len(rep.Suggestions)}
	for _, sug := range rep.Suggestions {
		s.TotalSuggestQty += sug.SuggestSellQty
		switch sug.Action {
		case ActionReduce:
			s.ReduceCount++
		case ActionExit:
			s.ExitCount++
		case ActionHold:
			s.HoldCount++
		}
	}
	rep.Summary = s
}

func joinRisk(codes []string, alloc *sellallocation.SellAllocation, rebRisk string) string {
	parts := make([]string, 0, 8)
	push := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		for _, p := range parts {
			if p == s {
				return
			}
		}
		parts = append(parts, s)
	}
	for _, c := range codes {
		if isRiskish(c) {
			push(c)
		}
	}
	if alloc != nil {
		switch alloc.Binding {
		case sellallocation.BindingT1, sellallocation.BindingRisk, sellallocation.BindingBelowMin:
			push(alloc.Binding)
		}
		if alloc.SkippedReason != "" && isRiskish(alloc.SkippedReason) {
			push(alloc.SkippedReason)
		}
	}
	push(rebRisk)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ";")
}

func isRiskish(code string) bool {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return false
	}
	switch c {
	case sellallocation.ReasonT1Locked, sellallocation.ReasonT1Clip,
		sellallocation.ReasonRiskBoost, sellallocation.ReasonBelowMin,
		sellallocation.ReasonLotRound, sellallocation.ReasonDataMissing,
		sellallocation.BindingT1, sellallocation.BindingRisk,
		rebalance.ReasonOverweight, rebalance.ReasonGrossBound,
		rebalance.ReasonSingleCap, rebalance.ReasonRiskTighten,
		rebalance.ReasonCashBound, rebalance.ReasonBlockNew,
		rebalance.ReasonTargetZero, rebalance.ReasonOrphan,
		holdingdecision.ReasonConcentration,
		holdingdecision.ReasonRiskIncrease, holdingdecision.ReasonRiskMaterial:
		return true
	}
	return strings.Contains(c, "RISK") ||
		strings.HasPrefix(c, "T1_") ||
		strings.Contains(c, "CAP") ||
		strings.Contains(c, "OVERWEIGHT") ||
		strings.Contains(c, "CONCENTRATION") ||
		strings.Contains(c, "GROSS") ||
		strings.Contains(c, "TIGHTEN")
}

func normSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func firstTime(a, b time.Time) time.Time {
	if !a.IsZero() {
		return a
	}
	return b
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
