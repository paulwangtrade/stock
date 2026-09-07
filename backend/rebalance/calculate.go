package rebalance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfoliorisk"
)

// Calculate computes Current vs Target → RebalanceSuggestion (suggest_only).
// Never creates TradePlan, never calls Execution, never mutates Holdings.
func Calculate(in RebalanceInput) *RebalanceSuggestion {
	opt := in.Options
	opt.SuggestOnly = true // nail phase
	if opt.MinDeltaWeight <= 0 || math.IsNaN(opt.MinDeltaWeight) {
		opt.MinDeltaWeight = DefaultCalculateOptions().MinDeltaWeight
	}
	if opt.ReduceVsExitEpsilon < 0 || math.IsNaN(opt.ReduceVsExitEpsilon) {
		opt.ReduceVsExitEpsilon = DefaultCalculateOptions().ReduceVsExitEpsilon
	}
	if strings.TrimSpace(opt.OrphanPolicy) == "" {
		opt.OrphanPolicy = OrphanExit
	}

	out := emptySuggestion()
	if !in.AsOf.IsZero() {
		out.AsOf = in.AsOf.UTC().Format(time.RFC3339)
	}
	out.TradeDate = strings.TrimSpace(in.TradeDate)

	if !in.Current.Found || in.Current.Equity <= 0 || math.IsNaN(in.Current.Equity) {
		out.Reason = ReasonDataMissing
		out.Notes = append(out.Notes, "current portfolio missing or equity non-positive")
		out.InputsFingerprint = fingerprint(in, out)
		return out
	}

	equity := in.Current.Equity
	curW := map[string]float64{}
	curMV := map[string]float64{}
	for _, p := range in.Current.Positions {
		sym := normSym(p.Symbol)
		if sym == "" {
			continue
		}
		curW[sym] = p.Weight
		curMV[sym] = p.MarketValue
		if p.MarketValue <= 0 && p.Weight > 0 {
			curMV[sym] = p.Weight * equity
		}
	}

	tgtW := map[string]float64{}
	for _, p := range in.Target.Positions {
		sym := normSym(p.Symbol)
		if sym == "" {
			continue
		}
		w := p.TargetWeight
		if w <= 0 && p.TargetAmount > 0 && equity > 0 {
			w = p.TargetAmount / equity
		}
		if w < 0 || math.IsNaN(w) {
			w = 0
		}
		tgtW[sym] = w
	}

	universe := map[string]struct{}{}
	for s := range curW {
		universe[s] = struct{}{}
	}
	for s := range tgtW {
		universe[s] = struct{}{}
	}
	syms := make([]string, 0, len(universe))
	for s := range universe {
		syms = append(syms, s)
	}
	sort.Strings(syms)

	eps := opt.ReduceVsExitEpsilon
	minDW := opt.MinDeltaWeight

	var rawBuys []rawBuy
	buyBefore := 0.0

	for _, sym := range syms {
		cw := curW[sym]
		tw, inT := tgtW[sym]
		if !inT {
			if opt.OrphanPolicy == OrphanHold {
				out.Skipped = append(out.Skipped, SkippedDelta{Symbol: sym, ReasonCodes: []string{ReasonAlreadyAtTarget}})
				continue
			}
			tw = 0
		}
		dw := tw - cw
		if math.Abs(dw) < minDW {
			out.Skipped = append(out.Skipped, SkippedDelta{Symbol: sym, ReasonCodes: []string{ReasonBelowMinDelta, ReasonAlreadyAtTarget}})
			continue
		}
		notion := math.Abs(dw) * equity
		if dw > 0 {
			codes := []string{ReasonUnderweight}
			reason := ReasonUnderweight
			isNew := cw <= 0 && tw > 0
			if isNew {
				codes = []string{ReasonNewName}
				reason = ReasonNewName
			}
			rawBuys = append(rawBuys, rawBuy{sym: sym, dw: dw, notion: notion, cw: cw, tw: tw, codes: codes, reason: reason, isNew: isNew})
			buyBefore += notion
			continue
		}
		// dw < 0
		if tw <= eps {
			codes := []string{ReasonTargetZero}
			reason := ReasonTargetZero
			if !inT {
				codes = []string{ReasonOrphan, ReasonTargetZero}
				reason = ReasonOrphan
			}
			out.Exits = append(out.Exits, ExitDelta{
				Symbol:        sym,
				DeltaWeight:   math.Abs(dw),
				DeltaNotional: notionFromMV(curMV[sym], math.Abs(dw), cw, equity),
				CurrentWeight: cw,
				Intent:        "flatten_sellable",
				Reason:        reason,
				ReasonCodes:   codes,
			})
			continue
		}
		out.Reduces = append(out.Reduces, ReduceDelta{
			Symbol:        sym,
			DeltaWeight:   math.Abs(dw),
			DeltaNotional: notionFromMV(curMV[sym], math.Abs(dw), cw, equity),
			CurrentWeight: cw,
			TargetWeight:  tw,
			Reason:        ReasonOverweight,
			ReasonCodes:   []string{ReasonOverweight},
		})
	}

	impact := RiskImpact{
		Applied:              opt.RiskTightenBuys,
		BuyNotionalBeforeCut: buyBefore,
		BlockNewEntries:      in.Resolved.BlockNewEntries,
	}
	effCap := effectiveSingleCap(in.Resolved, in.PortfolioRisk)
	impact.EffectiveSingleCap = effCap

	buys := make([]BuyDelta, 0, len(rawBuys))
	if !opt.RiskTightenBuys {
		for _, r := range rawBuys {
			buys = append(buys, BuyDelta{
				Symbol: r.sym, DeltaWeight: r.dw, DeltaNotional: r.notion,
				CurrentWeight: r.cw, TargetWeight: r.tw, Reason: r.reason, ReasonCodes: append([]string{}, r.codes...),
			})
		}
	} else {
		buys, impact = applyBuyRiskCuts(rawBuys, equity, curW, in, opt, impact)
	}
	impact.BuyNotionalAfterCut = sumBuyNotional(buys)
	impact.BuyNotionalCut = impact.BuyNotionalBeforeCut - impact.BuyNotionalAfterCut
	if impact.BuyNotionalCut < 0 {
		impact.BuyNotionalCut = 0
	}

	out.Buys = buys
	out.RiskImpact = impact
	out.Summary = SuggestSummary{
		BuyNotional:    sumBuyNotional(buys),
		ReduceNotional: sumReduceNotional(out.Reduces),
		ExitNotional:   sumExitNotional(out.Exits),
		BuyCount:       len(buys),
		ReduceCount:    len(out.Reduces),
		ExitCount:      len(out.Exits),
	}
	out.Reason = summarizeReason(out)
	out.InputsFingerprint = fingerprint(in, out)
	return out
}

func emptySuggestion() *RebalanceSuggestion {
	return &RebalanceSuggestion{
		SchemaVersion:     SuggestSchemaVersion,
		Phase:             PhaseSuggestOnly,
		SuggestOnly:       true,
		RecordOnly:        true,
		NotTradePlan:      true,
		NotExecution:      true,
		NotBuyChain:       true,
		NotHoldingsMutate: true,
		Buys:              []BuyDelta{},
		Reduces:           []ReduceDelta{},
		Exits:             []ExitDelta{},
		DataSourceNote:    suggestDataNote,
	}
}

func notionFromMV(mv, absDW, cw, equity float64) float64 {
	if mv > 0 && cw > 1e-12 {
		return mv * (absDW / cw)
	}
	return absDW * equity
}

func effectiveSingleCap(resolved allocationengine.ResolvedConstraints, risk *portfoliorisk.PortfolioRiskSnapshot) float64 {
	cap := resolved.MaxSingleWeight
	if risk != nil && risk.Found && risk.Concentration.Available && risk.Concentration.CapSingle != nil && *risk.Concentration.CapSingle > 0 {
		if cap <= 0 || *risk.Concentration.CapSingle < cap {
			cap = *risk.Concentration.CapSingle
		}
	}
	if cap < 0 || math.IsNaN(cap) {
		return 0
	}
	return cap
}

func applyBuyRiskCuts(
	raw []rawBuy,
	equity float64,
	curW map[string]float64,
	in RebalanceInput,
	opt CalculateOptions,
	impact RiskImpact,
) ([]BuyDelta, RiskImpact) {
	resolved := in.Resolved
	effCap := impact.EffectiveSingleCap
	headroom := buyHeadroom(in, opt)
	if headroom != nil {
		impact.GrossHeadroomUsed = headroom
	}

	type adj struct {
		r       rawBuy
		notion  float64
		binding string
		codes   []string
	}
	work := make([]adj, 0, len(raw))
	for _, r := range raw {
		codes := append([]string{}, r.codes...)
		notion := r.notion
		binding := ""
		if resolved.BlockNewEntries && r.isNew {
			impact.NamesBuyBlocked++
			impact.Notes = append(impact.Notes, "block_new_entries:"+r.sym)
			outSkip := true
			_ = outSkip
			continue
		}
		// Single cap: post weight ≤ cap
		if effCap > 0 {
			maxBuyW := effCap - r.cw
			if maxBuyW <= 0 {
				impact.NamesBuyClipped++
				impact.Notes = append(impact.Notes, "single_cap_block:"+r.sym)
				continue
			}
			maxNotion := maxBuyW * equity
			if notion > maxNotion+1e-9 {
				notion = maxNotion
				binding = ReasonSingleCap
				codes = append(codes, ReasonSingleCap, ReasonRiskTighten)
				impact.NamesBuyClipped++
			}
			// Also clip target-implied: tw may exceed cap
			if r.tw > effCap+1e-12 {
				codes = appendUnique(codes, ReasonSingleCap)
			}
		}
		if resolved.MinOrderAmount > 0 && notion > 0 && notion < resolved.MinOrderAmount {
			impact.Notes = append(impact.Notes, "below_min_order:"+r.sym)
			continue
		}
		work = append(work, adj{r: r, notion: notion, binding: binding, codes: codes})
	}

	sum := 0.0
	for _, w := range work {
		sum += w.notion
	}
	scale := 1.0
	budget := math.Inf(1)
	if headroom != nil && *headroom >= 0 {
		budget = *headroom
	}
	if opt.MaxBuyNotional > 0 && opt.MaxBuyNotional < budget {
		budget = opt.MaxBuyNotional
	}
	if !math.IsInf(budget, 1) && sum > budget+1e-9 && sum > 0 {
		scale = budget / sum
		impact.Notes = append(impact.Notes, "gross_or_cash_scale")
		if headroom != nil {
			impact.Notes = append(impact.Notes, ReasonGrossBound)
		}
	}

	out := make([]BuyDelta, 0, len(work))
	for _, w := range work {
		notion := w.notion * scale
		if notion <= 0 {
			continue
		}
		dw := notion / equity
		binding := w.binding
		codes := w.codes
		if scale < 1-1e-12 {
			binding = firstNonEmpty(binding, ReasonGrossBound)
			codes = appendUnique(codes, ReasonGrossBound, ReasonRiskTighten)
			impact.NamesBuyClipped++
		}
		out = append(out, BuyDelta{
			Symbol:        w.r.sym,
			DeltaWeight:   dw,
			DeltaNotional: notion,
			CurrentWeight: w.r.cw,
			TargetWeight:  w.r.tw,
			Reason:        w.r.reason,
			ReasonCodes:   codes,
			RiskBinding:   binding,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out, impact
}

func buyHeadroom(in RebalanceInput, opt CalculateOptions) *float64 {
	cash := in.Current.Cash
	reserve := 0.0
	if in.Resolved.ReserveCashRatio > 0 && in.Current.Equity > 0 {
		reserve = in.Current.Equity * in.Resolved.ReserveCashRatio
	}
	availCash := cash - reserve
	if availCash < 0 {
		availCash = 0
	}
	grossBudget := math.Inf(1)
	if in.Resolved.MaxGrossExposurePct > 0 && in.Current.Equity > 0 {
		maxGross := in.Resolved.MaxGrossExposurePct * in.Current.Equity
		room := maxGross - in.Current.Exposure
		if room < 0 {
			room = 0
		}
		grossBudget = room
	}
	// PortfolioRisk exposure headroom when available
	if in.PortfolioRisk != nil && in.PortfolioRisk.Found && in.PortfolioRisk.Exposure.Available {
		if in.PortfolioRisk.Exposure.HeadroomVsCap != nil {
			h := *in.PortfolioRisk.Exposure.HeadroomVsCap
			if h < 0 {
				h = 0
			}
			// headroom is typically a ratio; if |h|<=1 treat as fraction of equity
			var room float64
			if h <= 1.0+1e-9 {
				room = h * in.Current.Equity
			} else {
				room = h
			}
			if room < grossBudget {
				grossBudget = room
			}
		}
	}
	out := math.Min(availCash, grossBudget)
	if math.IsInf(out, 1) {
		out = availCash
	}
	return &out
}

func sumBuyNotional(buys []BuyDelta) float64 {
	s := 0.0
	for _, b := range buys {
		s += b.DeltaNotional
	}
	return s
}

func sumReduceNotional(rows []ReduceDelta) float64 {
	s := 0.0
	for _, b := range rows {
		s += b.DeltaNotional
	}
	return s
}

func sumExitNotional(rows []ExitDelta) float64 {
	s := 0.0
	for _, b := range rows {
		s += b.DeltaNotional
	}
	return s
}

func summarizeReason(out *RebalanceSuggestion) string {
	switch {
	case out.Summary.BuyCount+out.Summary.ReduceCount+out.Summary.ExitCount == 0:
		return ReasonAlreadyAtTarget
	case out.RiskImpact.BuyNotionalCut > 1e-6:
		return ReasonRiskTighten
	case out.Summary.ExitCount > 0:
		return ReasonTargetZero
	case out.Summary.ReduceCount > 0:
		return ReasonOverweight
	case out.Summary.BuyCount > 0:
		return ReasonUnderweight
	default:
		return ReasonAlreadyAtTarget
	}
}

func appendUnique(codes []string, add ...string) []string {
	seen := map[string]struct{}{}
	for _, c := range codes {
		seen[c] = struct{}{}
	}
	for _, a := range add {
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

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func normSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func fingerprint(in RebalanceInput, out *RebalanceSuggestion) string {
	type row struct {
		S string  `json:"s"`
		W float64 `json:"w"`
		A float64 `json:"a"`
	}
	payload := struct {
		Schema   string  `json:"schema"`
		Date     string  `json:"date"`
		Equity   float64 `json:"equity"`
		Cur      []row   `json:"cur"`
		Tgt      []row   `json:"tgt"`
		Single   float64 `json:"single"`
		Gross    float64 `json:"gross"`
		BlockNew bool    `json:"block_new"`
		MinDW    float64 `json:"min_dw"`
		Buys     []row   `json:"buys"`
		Reds     []row   `json:"reds"`
		Exits    []row   `json:"exits"`
		RiskCut  float64 `json:"risk_cut"`
	}{
		Schema:   SuggestSchemaVersion,
		Date:     out.TradeDate,
		Equity:   in.Current.Equity,
		Single:   in.Resolved.MaxSingleWeight,
		Gross:    in.Resolved.MaxGrossExposurePct,
		BlockNew: in.Resolved.BlockNewEntries,
		MinDW:    in.Options.MinDeltaWeight,
		RiskCut:  out.RiskImpact.BuyNotionalCut,
	}
	for _, p := range in.Current.Positions {
		payload.Cur = append(payload.Cur, row{S: normSym(p.Symbol), W: p.Weight, A: p.MarketValue})
	}
	for _, p := range in.Target.Positions {
		payload.Tgt = append(payload.Tgt, row{S: normSym(p.Symbol), W: p.TargetWeight, A: p.TargetAmount})
	}
	sort.Slice(payload.Cur, func(i, j int) bool { return payload.Cur[i].S < payload.Cur[j].S })
	sort.Slice(payload.Tgt, func(i, j int) bool { return payload.Tgt[i].S < payload.Tgt[j].S })
	for _, b := range out.Buys {
		payload.Buys = append(payload.Buys, row{S: b.Symbol, W: b.DeltaWeight, A: b.DeltaNotional})
	}
	for _, b := range out.Reduces {
		payload.Reds = append(payload.Reds, row{S: b.Symbol, W: b.DeltaWeight, A: b.DeltaNotional})
	}
	for _, b := range out.Exits {
		payload.Exits = append(payload.Exits, row{S: b.Symbol, W: b.DeltaWeight, A: b.DeltaNotional})
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
