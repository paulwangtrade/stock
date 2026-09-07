package decisionshadowv2

import (
	"math"
	"sort"
	"strings"
)

const amountEps = 1e-6

func buildDifference(legacy, portfolio PlanProjection, risk RiskImpactDiff, industryAvailable bool) DecisionShadowDifference {
	legSet := selectedSet(legacy)
	portSet := selectedSet(portfolio)

	onlyL, onlyP, both := []string{}, []string{}, []string{}
	for s := range legSet {
		if _, ok := portSet[s]; ok {
			both = append(both, s)
		} else {
			onlyL = append(onlyL, s)
		}
	}
	for s := range portSet {
		if _, ok := legSet[s]; !ok {
			onlyP = append(onlyP, s)
		}
	}
	sort.Strings(onlyL)
	sort.Strings(onlyP)
	sort.Strings(both)

	legAmt := amountMap(legacy)
	portAmt := amountMap(portfolio)
	allSym := map[string]struct{}{}
	for s := range legAmt {
		allSym[s] = struct{}{}
	}
	for s := range portAmt {
		allSym[s] = struct{}{}
	}
	syms := sortedKeys(allSym)

	amountRows := make([]AmountDiffRow, 0, len(syms))
	changed := []string{}
	for _, s := range syms {
		la, lok := legAmt[s]
		pa, pok := portAmt[s]
		state := "actual"
		if !lok && !pok {
			state = "absent"
		} else if (!lok || la == 0) && (!pok || pa == 0) {
			state = "zero"
		} else if !lok || !pok {
			state = "absent"
			if (lok && la > 0) || (pok && pa > 0) {
				state = "actual"
			}
		}
		row := AmountDiffRow{
			Symbol:          s,
			LegacyAmount:    la,
			PortfolioAmount: pa,
			Delta:           pa - la,
			State:           state,
		}
		amountRows = append(amountRows, row)
		if math.Abs(pa-la) > amountEps {
			changed = append(changed, s)
		}
	}

	legW := weightMap(legacy)
	portW := weightMap(portfolio)
	wSyms := map[string]struct{}{}
	for s := range legW {
		wSyms[s] = struct{}{}
	}
	for s := range portW {
		wSyms[s] = struct{}{}
	}
	weightRows := make([]WeightDiffRow, 0, len(wSyms))
	maxWDelta := 0.0
	for _, s := range sortedKeys(wSyms) {
		d := portW[s] - legW[s]
		weightRows = append(weightRows, WeightDiffRow{
			Symbol: s, LegacyWeight: legW[s], PortfolioWeight: portW[s], Delta: d,
		})
		if math.Abs(d) > math.Abs(maxWDelta) {
			maxWDelta = d
		}
	}

	sector := SectorDiff{Available: false, UnavailableReason: "industry_by_symbol unavailable"}
	if industryAvailable && legacy.BookProjection != nil && portfolio.BookProjection != nil &&
		legacy.BookProjection.SectorAvailable && portfolio.BookProjection.SectorAvailable {
		sector = compareSectors(legacy.BookProjection.PostSectorExposure, portfolio.BookProjection.PostSectorExposure)
	}

	cap := CapitalDiff{
		LegacyBuyNotional:    legacy.Totals.BuyNotionalSum,
		PortfolioBuyNotional: portfolio.Totals.BuyNotionalSum,
		Delta:                portfolio.Totals.BuyNotionalSum - legacy.Totals.BuyNotionalSum,
		LegacyNameCount:      legacy.Totals.NameCount,
		PortfolioNameCount:   portfolio.Totals.NameCount,
	}

	return DecisionShadowDifference{
		SelectionDiff:  SelectionDiff{OnlyLegacy: onlyL, OnlyPortfolio: onlyP, Both: both},
		AmountDiff:     amountRows,
		WeightDiff:     weightRows,
		SectorDiff:     sector,
		CapitalDiff:    cap,
		RiskImpactDiff: risk,
		Summary: DiffSummary{
			LegacyBuyNotional:    legacy.Totals.BuyNotionalSum,
			PortfolioBuyNotional: portfolio.Totals.BuyNotionalSum,
			NotionalDelta:        portfolio.Totals.BuyNotionalSum - legacy.Totals.BuyNotionalSum,
			NamesAdded:           onlyP,
			NamesDropped:         onlyL,
			NamesAmountChanged:   changed,
			MaxSingleWeightDelta: maxWDelta,
			SectorsHotter:        sector.Hotter,
			SectorsCooler:        sector.Cooler,
		},
	}
}

func selectedSet(p PlanProjection) map[string]struct{} {
	out := map[string]struct{}{}
	for _, s := range p.Selected {
		if s.Role == "selected" || s.Role == "" {
			out[s.Symbol] = struct{}{}
		}
	}
	for _, ln := range p.Lines {
		if ln.Role == "selected" && ln.TargetAmount > amountEps {
			out[ln.Symbol] = struct{}{}
		}
	}
	return out
}

func amountMap(p PlanProjection) map[string]float64 {
	out := map[string]float64{}
	for _, ln := range p.Lines {
		if ln.Role == "waitlist" {
			continue
		}
		out[ln.Symbol] = ln.TargetAmount
	}
	return out
}

func weightMap(p PlanProjection) map[string]float64 {
	out := map[string]float64{}
	for _, ln := range p.Lines {
		if ln.Role == "waitlist" {
			continue
		}
		out[ln.Symbol] = ln.ImpliedPostWeight
	}
	return out
}

func compareSectors(leg, port []SectorExposure) SectorDiff {
	lm := map[string]float64{}
	pm := map[string]float64{}
	for _, s := range leg {
		lm[s.Industry] = s.Weight
	}
	for _, s := range port {
		pm[s.Industry] = s.Weight
	}
	keys := map[string]struct{}{}
	for k := range lm {
		keys[k] = struct{}{}
	}
	for k := range pm {
		keys[k] = struct{}{}
	}
	delta := []SectorExposure{}
	hotter, cooler := []string{}, []string{}
	for _, k := range sortedKeys(keys) {
		d := pm[k] - lm[k]
		delta = append(delta, SectorExposure{Industry: k, Weight: d})
		if d > amountEps {
			hotter = append(hotter, k)
		} else if d < -amountEps {
			cooler = append(cooler, k)
		}
	}
	return SectorDiff{
		Available: true,
		Legacy:    leg,
		Portfolio: port,
		Delta:     delta,
		Hotter:    hotter,
		Cooler:    cooler,
	}
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sectorAvailable(in Input, legacy, port PlanProjection) bool {
	if len(in.IndustryBySymbol) == 0 {
		// candidate/line industries may still work if book marked available
		if legacy.BookProjection != nil && port.BookProjection != nil {
			return legacy.BookProjection.SectorAvailable && port.BookProjection.SectorAvailable
		}
		return false
	}
	for _, ln := range append(append([]PlanLine{}, legacy.Lines...), port.Lines...) {
		if ln.Role == "waitlist" {
			continue
		}
		ind := strings.TrimSpace(ln.Industry)
		if ind == "" {
			if v := strings.TrimSpace(in.IndustryBySymbol[ln.Symbol]); v != "" {
				continue
			}
			return false
		}
	}
	return true
}
