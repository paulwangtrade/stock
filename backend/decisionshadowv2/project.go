package decisionshadowv2

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"
)

func normalizeSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func legacyAmount(in Input) float64 {
	if in.LegacyAmountPerName > 0 {
		return in.LegacyAmountPerName
	}
	return DefaultLegacyAmount
}

func legacyLimit(in Input) int {
	if in.SelectionLimitLegacy > 0 {
		return in.SelectionLimitLegacy
	}
	return 5
}

func portfolioLimit(in Input, resolved portfoliolayer.ResolvedConstraints) int {
	if in.SelectionLimitPortfolio > 0 {
		if resolved.MaxNewNames > 0 && resolved.MaxNewNames < in.SelectionLimitPortfolio {
			return resolved.MaxNewNames
		}
		return in.SelectionLimitPortfolio
	}
	if resolved.MaxNewNames > 0 {
		return resolved.MaxNewNames
	}
	return legacyLimit(in)
}

// stopNewNames detects preference MaxNewNames=0 (tighten) before Resolve() replaces 0 with default.
func stopNewNames(cs portfoliolayer.ConstraintSet, resolved portfoliolayer.ResolvedConstraints) bool {
	if resolved.RiskBlockNewEntries {
		return true
	}
	for _, p := range []*int{cs.Portfolio.MaxNewNames, cs.User.MaxNewNames, cs.Strategy.MaxNewNames} {
		if p != nil && *p == 0 {
			return true
		}
	}
	return false
}

func rankedCandidates(pool []selection.Candidate) []selection.Candidate {
	out := make([]selection.Candidate, 0, len(pool))
	for i, c := range pool {
		cc := c
		if cc.Rank <= 0 {
			cc.Rank = i + 1
		}
		cc.StockCode = normalizeSym(cc.StockCode)
		if cc.StockCode == "" {
			continue
		}
		out = append(out, cc)
	}
	return out
}

func projectLegacy(in Input, ranked []selection.Candidate) PlanProjection {
	limit := legacyLimit(in)
	amt := legacyAmount(in)
	proj := PlanProjection{
		ProviderIdentity: "fixed_amount",
		Selected:         []SelectedName{},
		Lines:            []PlanLine{},
		OK:               true,
	}
	equity := 0.0
	cash := 0.0
	exposure := 0.0
	holdMV := map[string]float64{}
	if in.Snapshot != nil && in.Snapshot.Found {
		equity = in.Snapshot.Equity
		cash = in.Snapshot.Cash
		exposure = in.Snapshot.Exposure
		holdMV = holdingMV(in.Snapshot)
	}

	n := 0
	sum := 0.0
	for i, c := range ranked {
		role := "waitlist"
		inSet := i < limit
		if inSet {
			role = "selected"
			n++
			sum += amt
		}
		lineAmt := 0.0
		reason := "waitlist"
		if inSet {
			lineAmt = amt
			reason = "fixed_amount"
		}
		wDelta := 0.0
		postW := 0.0
		if equity > 0 {
			wDelta = lineAmt / equity
			postW = (holdMV[c.StockCode] + lineAmt) / equity
		}
		ind := industryOf(in, c)
		proj.Selected = append(proj.Selected, SelectedName{Symbol: c.StockCode, Role: role})
		if inSet || in.Options.IncludeWaitlistInDiff {
			proj.Lines = append(proj.Lines, PlanLine{
				Symbol:            c.StockCode,
				TargetAmount:      lineAmt,
				TargetWeightDelta: wDelta,
				ImpliedPostWeight: postW,
				Industry:          ind,
				AllocationReason:  reason,
				RiskBinding:       "legacy_fixed",
				Role:              role,
			})
		}
	}
	proj.Totals = PlanTotals{
		BuyNotionalSum: sum,
		NameCount:      n,
		BudgetBinding:  "legacy_fixed_amount",
	}
	if in.Options.ProjectPostBuyBook {
		proj.BookProjection = buildBook(in, proj.Lines, equity, cash, exposure)
	}
	return proj
}

func projectPortfolio(
	in Input,
	ranked []selection.Candidate,
	cs portfoliolayer.ConstraintSet,
	resolved portfoliolayer.ResolvedConstraints,
	allocOpts allocationengine.Options,
) (PlanProjection, *portfoliolayer.PortfolioSelectionResult, *allocationengine.AllocationResult, string) {
	proj := PlanProjection{
		ProviderIdentity: "portfolio_allocation",
		Selected:         []SelectedName{},
		Lines:            []PlanLine{},
		OK:               true,
	}

	if stopNewNames(cs, resolved) {
		proj.Totals = PlanTotals{
			BuyNotionalSum: 0,
			NameCount:      0,
			BudgetBinding:  "blocked_stop_new_names",
		}
		budgetSummary := "available=0 reserve=0 binding=blocked_stop_new_names method=equal_weight selected=0"
		emptySel := &portfoliolayer.PortfolioSelectionResult{
			Selected: []portfoliolayer.PortfolioPick{},
			Waitlist: []portfoliolayer.PortfolioPick{},
			Rejected: []portfoliolayer.PortfolioReject{},
			Reasons:  []string{"max_new_names_stop"},
		}
		for _, c := range ranked {
			sym := normalizeSym(c.StockCode)
			proj.Selected = append(proj.Selected, SelectedName{Symbol: sym, Role: "waitlist"})
		}
		if in.Options.ProjectPostBuyBook {
			equity, cash, exposure := 0.0, 0.0, 0.0
			if in.Snapshot != nil && in.Snapshot.Found {
				equity, cash, exposure = in.Snapshot.Equity, in.Snapshot.Cash, in.Snapshot.Exposure
			}
			proj.BookProjection = buildBook(in, proj.Lines, equity, cash, exposure)
		}
		return proj, emptySel, nil, budgetSummary
	}

	limit := portfolioLimit(in, resolved)
	sel := portfoliolayer.SelectPortfolio(portfoliolayer.PortfolioSelectionInput{
		RankedCandidates: ranked,
		Snapshot:         in.Snapshot,
		Constraints: portfoliolayer.PortfolioConstraints{
			MaxNewNames:        limit,
			SkipAlreadyHolding: resolved.SkipAlreadyHolding,
			MaxSectorWeight:    resolved.MaxSectorWeight,
			MaxNamesPerSector:  resolved.MaxNamesPerSector,
			AllowAddToHolding:  resolved.AllowAddToHolding,
		},
	})

	engineSel := allocationengine.SelectionResult{}
	if sel != nil {
		for _, p := range sel.Selected {
			engineSel.Selected = append(engineSel.Selected, normalizeSym(p.Candidate.StockCode))
		}
		for _, p := range sel.Waitlist {
			engineSel.Waitlist = append(engineSel.Waitlist, normalizeSym(p.Candidate.StockCode))
		}
	}

	engSnap := &allocationengine.PortfolioSnapshot{Found: false}
	if in.Snapshot != nil {
		engSnap = &allocationengine.PortfolioSnapshot{
			Found:        in.Snapshot.Found,
			Equity:       in.Snapshot.Equity,
			Cash:         in.Snapshot.Cash,
			ReservedCash: in.Snapshot.ReservedCash,
			Exposure:     in.Snapshot.Exposure,
		}
	}
	policy := allocationengine.ResolvedConstraints{
		MaxGrossExposurePct: resolved.MaxGrossExposurePct,
		MaxSingleWeight:     resolved.MaxSingleWeight,
		ReserveCashRatio:    resolved.ReserveCashRatio,
		MinOrderAmount:      resolved.MinOrderAmount,
		BlockNewEntries:     resolved.RiskBlockNewEntries,
	}
	eng := allocationengine.Allocate(allocationengine.EngineInput{
		Selection: engineSel,
		Snapshot:  engSnap,
		Resolved:  policy,
		Options:   allocOpts,
	})

	equity := 0.0
	cash := 0.0
	exposure := 0.0
	holdMV := map[string]float64{}
	if in.Snapshot != nil && in.Snapshot.Found {
		equity = in.Snapshot.Equity
		cash = in.Snapshot.Cash
		exposure = in.Snapshot.Exposure
		holdMV = holdingMV(in.Snapshot)
	}

	reasonBySym := map[string]string{}
	amtBySym := map[string]float64{}
	binding := ""
	if eng != nil {
		binding = eng.Budget.Binding
		proj.Totals.ReserveCash = eng.Budget.ReserveCash
		proj.Totals.AvailableCapital = eng.Budget.AvailableCapital
		proj.Totals.BudgetBinding = eng.Budget.Binding
		for _, it := range eng.Items {
			sym := normalizeSym(it.StockCode)
			amtBySym[sym] = it.TargetAmount
			reasonBySym[sym] = it.AllocationReason
		}
	}

	sum := 0.0
	n := 0
	if sel != nil {
		for _, p := range sel.Selected {
			sym := normalizeSym(p.Candidate.StockCode)
			amt := amtBySym[sym]
			sum += amt
			if amt > 0 {
				n++
			}
			wDelta, postW := 0.0, 0.0
			if equity > 0 {
				wDelta = amt / equity
				postW = (holdMV[sym] + amt) / equity
			}
			proj.Selected = append(proj.Selected, SelectedName{Symbol: sym, Role: "selected"})
			proj.Lines = append(proj.Lines, PlanLine{
				Symbol:            sym,
				TargetAmount:      amt,
				TargetWeightDelta: wDelta,
				ImpliedPostWeight: postW,
				Industry:          industryOf(in, p.Candidate),
				AllocationReason:  reasonBySym[sym],
				RiskBinding:       binding,
				Role:              "selected",
			})
		}
		for _, p := range sel.Waitlist {
			sym := normalizeSym(p.Candidate.StockCode)
			proj.Selected = append(proj.Selected, SelectedName{Symbol: sym, Role: "waitlist"})
			if !in.Options.IncludeWaitlistInDiff {
				continue
			}
			amt := amtBySym[sym]
			proj.Lines = append(proj.Lines, PlanLine{
				Symbol:           sym,
				TargetAmount:     amt,
				Industry:         industryOf(in, p.Candidate),
				AllocationReason: reasonBySym[sym],
				RiskBinding:      binding,
				Role:             "waitlist",
			})
		}
	}
	proj.Totals.BuyNotionalSum = sum
	proj.Totals.NameCount = n

	budgetSummary := fmt.Sprintf(
		"available=%.0f reserve=%.0f binding=%s method=%s selected=%d",
		proj.Totals.AvailableCapital, proj.Totals.ReserveCash, proj.Totals.BudgetBinding,
		allocMethod(eng), len(engineSel.Selected),
	)
	if in.Options.ProjectPostBuyBook {
		proj.BookProjection = buildBook(in, proj.Lines, equity, cash, exposure)
	}
	return proj, sel, eng, budgetSummary
}

func allocMethod(eng *allocationengine.AllocationResult) string {
	if eng == nil {
		return ""
	}
	return eng.Method
}

func holdingMV(snap *portfoliolayer.PortfolioSnapshot) map[string]float64 {
	out := map[string]float64{}
	if snap == nil {
		return out
	}
	for _, h := range snap.Positions {
		sym := normalizeSym(h.StockCode)
		if sym == "" {
			continue
		}
		out[sym] += h.MarketValue
	}
	return out
}

func industryOf(in Input, c selection.Candidate) string {
	sym := normalizeSym(c.StockCode)
	if in.IndustryBySymbol != nil {
		if v, ok := in.IndustryBySymbol[sym]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		// also try original case keys
		if v, ok := in.IndustryBySymbol[c.StockCode]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return strings.TrimSpace(c.Industry)
}

func buildBook(in Input, lines []PlanLine, equity, cash, exposure float64) *BookProjection {
	book := &BookProjection{SectorAvailable: false}
	if equity <= 0 {
		return book
	}
	buySum := 0.0
	top1 := 0.0
	postW := map[string]float64{}
	holdMV := holdingMV(in.Snapshot)
	for sym, mv := range holdMV {
		postW[sym] = mv / equity
	}
	for _, ln := range lines {
		if ln.Role == "waitlist" && ln.TargetAmount <= 0 {
			continue
		}
		buySum += ln.TargetAmount
		w := (holdMV[ln.Symbol] + ln.TargetAmount) / equity
		postW[ln.Symbol] = w
		if w > top1 {
			top1 = w
		}
	}
	book.PostGrossExposure = (exposure + buySum) / equity
	book.PostCashRatio = math.Max(0, (cash-buySum)/equity)
	book.PostTop1Weight = top1

	sectorOK := industryMapUsable(in, lines)
	book.SectorAvailable = sectorOK
	if sectorOK {
		agg := map[string]float64{}
		for sym, w := range postW {
			ind := ""
			if in.IndustryBySymbol != nil {
				ind = strings.TrimSpace(in.IndustryBySymbol[sym])
			}
			if ind == "" {
				for _, ln := range lines {
					if ln.Symbol == sym {
						ind = ln.Industry
						break
					}
				}
			}
			if ind == "" {
				ind = "unknown"
			}
			agg[ind] += w
		}
		book.PostSectorExposure = sortedSectors(agg)
	}
	return book
}

func industryMapUsable(in Input, lines []PlanLine) bool {
	if len(in.IndustryBySymbol) == 0 {
		// fall back: all selected lines must carry industry
		for _, ln := range lines {
			if ln.Role != "selected" {
				continue
			}
			if strings.TrimSpace(ln.Industry) == "" {
				return false
			}
		}
		return len(lines) > 0
	}
	for _, ln := range lines {
		if ln.Role != "selected" {
			continue
		}
		if strings.TrimSpace(industryOf(in, selection.Candidate{StockCode: ln.Symbol, Industry: ln.Industry})) == "" {
			return false
		}
	}
	return true
}

func sortedSectors(agg map[string]float64) []SectorExposure {
	keys := make([]string, 0, len(agg))
	for k := range agg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SectorExposure, 0, len(keys))
	for _, k := range keys {
		out = append(out, SectorExposure{Industry: k, Weight: agg[k]})
	}
	return out
}
