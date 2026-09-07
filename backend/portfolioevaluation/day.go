package portfolioevaluation

import (
	"sort"
	"strings"

	"go-stock/backend/decisionshadowv2"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfolioinsight"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/portfoliosim"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/selection"
)

func evaluateDay(day portfoliovalidation.DayCase, opt Options) DayEvaluation {
	out := DayEvaluation{
		CaseID:    strings.TrimSpace(day.CaseID),
		TradeDate: strings.TrimSpace(day.TradeDate),
	}
	if out.CaseID == "" {
		out.CaseID = out.TradeDate
	}
	if day.Candidates == nil || len(day.Candidates.RankedCandidates) == 0 {
		out.Error = "candidates_required"
		return out
	}

	legacyAmt := day.LegacyAmountPerName
	if legacyAmt <= 0 {
		legacyAmt = DefaultLegacyAmount
	}
	filter := portfoliosim.FilterCompatRequest{SkipCompatibilityCheck: opt.SkipFilterCompat}

	legacyRes := portfoliosim.Simulate(portfoliosim.Input{
		Snapshot:            day.Snapshot,
		Candidates:          day.Candidates,
		Constraints:         day.Constraints,
		Budget:              legacyBasketBudget(day, legacyAmt),
		Filter:              filter,
		DecisionTime:        day.DecisionTime,
		SkipRiskTighten:     true,
		LegacyAmountPerName: legacyAmt,
	})
	portRes := portfoliosim.Simulate(portfoliosim.Input{
		Snapshot:        day.Snapshot,
		Candidates:      day.Candidates,
		Constraints:     day.Constraints,
		Budget:          day.Budget,
		Filter:          filter,
		DecisionTime:    day.DecisionTime,
		SkipRiskTighten: false,
	})
	if legacyRes == nil || portRes == nil {
		out.Error = "simulate_nil"
		return out
	}

	industry := mergeIndustry(day)
	out.Legacy = sideFromSim("fixed_amount", legacyRes, day.Snapshot, industry, true)
	out.Portfolio = sideFromSim("portfolio_allocation", portRes, day.Snapshot, industry, false)

	if opt.AttachDecisionShadowV2 {
		v2 := decisionshadowv2.Run(decisionshadowv2.Input{
			Enabled:                 true,
			AsOf:                    day.DecisionTime,
			TradeDate:               day.TradeDate,
			CandidatePool:           day.Candidates.RankedCandidates,
			Snapshot:                day.Snapshot,
			IndustryBySymbol:        industry,
			Objective:               day.Objective,
			RiskCeilingTemplate:     ceilingOr(day),
			SelectionLimitLegacy:    selectionLimit(day),
			SelectionLimitPortfolio: selectionLimit(day),
			LegacyAmountPerName:     legacyAmt,
			Options: decisionshadowv2.Options{
				Phase:              "shadow_only",
				ProjectPostBuyBook: true,
			},
		}, true)
		if v2 != nil && !v2.Skipped {
			out.DecisionShadowV2Attached = true
			enrichFromV2(&out.Legacy, &out.Portfolio, v2)
		}
	}

	if opt.AttachInsight {
		risk := portRes.PortfolioRiskSnapshot
		if risk == nil && day.Snapshot != nil {
			risk = portfoliorisk.Build(portfoliorisk.BuildInput{
				Snapshot:         day.Snapshot.Ledger(),
				IndustryBySymbol: industry,
				Constraints:      &day.Constraints,
			})
		}
		ins := portfolioinsight.Build(portfolioinsight.Input{
			AsOf:      day.DecisionTime,
			TradeDate: day.TradeDate,
			Risk:      risk,
		})
		if ins != nil {
			out.InsightAttached = true
			out.InsightRiskLevel = ins.PortfolioSummary.RiskLevel
			out.InsightDisclaimer = ins.Disclaimer
		}
	}

	out.OK = true
	return out
}

func sideFromSim(
	identity string,
	res *portfoliosim.SimulatedPortfolioDecisionResult,
	snap *portfoliolayer.PortfolioSnapshot,
	industry map[string]string,
	legacy bool,
) SideDayMetrics {
	side := SideDayMetrics{
		ProviderIdentity:    identity,
		FilterRejectReasons: map[string]int{},
		AllocationReasons:   map[string]int{},
		SelectionReasons:    map[string]int{},
		RiskReasons:         map[string]int{},
	}
	if res == nil {
		return side
	}

	side.SelectedCount = res.Evaluation.SelectedCount
	side.WaitlistCount = res.Evaluation.WaitlistCount
	side.BuyNotional = res.Evaluation.AllocationSetNotional
	if legacy && res.LegacyCompare != nil {
		if side.SelectedCount == 0 {
			side.SelectedCount = res.LegacyCompare.SelectedCount
		}
		if side.BuyNotional == 0 {
			side.BuyNotional = res.LegacyCompare.SumAllocationSet
		}
	}

	allocN := 0
	if res.Allocation != nil {
		side.BudgetBinding = res.Allocation.Budget.Binding
		side.BudgetCut = isBudgetCut(side.BudgetBinding)
		for _, it := range res.Allocation.Items {
			if it.InAllocationSet && it.TargetAmount > 0 {
				allocN++
			}
			reason := strings.TrimSpace(it.AllocationReason)
			if reason != "" {
				side.AllocationReasons[reason]++
			}
		}
	}
	side.AllocationCount = allocN

	side.GrossExposure = projectGross(snap, side.BuyNotional)
	if cr, ok := projectCashRatio(snap, side.BuyNotional); ok {
		side.CashRatio = &cr
	}
	side.Top1Weight, side.Top5Weight = projectTopWeights(snap, res)

	side.SectorAvailable, side.SectorExposure, side.MaxSectorWeight = projectSectors(snap, res, industry)

	if res.RiskConstraintTrace != nil {
		side.TightenApplied = res.RiskConstraintTrace.Applied || res.RiskConstraintTrace.HasPatches
		side.TightenCount = len(res.RiskConstraintTrace.Notes)
		for _, n := range res.RiskConstraintTrace.Notes {
			r := strings.TrimSpace(n.Reason)
			if r == "" {
				r = strings.TrimSpace(n.Field)
			}
			if r != "" {
				side.TightenReasons = append(side.TightenReasons, r)
				side.RiskReasons["tighten:"+r]++
			}
		}
		sort.Strings(side.TightenReasons)
	}

	for _, sk := range res.PotentialFilter.Skipped {
		code := strings.TrimSpace(sk.RiskCode)
		if code == "" {
			code = "unknown"
		}
		side.FilterRejectReasons[code]++
		side.RiskReasons["filter:"+code]++
	}

	collectSelectionReasons(&side, res.Selection)
	return side
}

func collectSelectionReasons(side *SideDayMetrics, sel *selection.CandidateSelectionResult) {
	if side == nil || sel == nil {
		return
	}
	if len(sel.CandidateDecisions) > 0 {
		for _, d := range sel.CandidateDecisions {
			r := strings.TrimSpace(d.SelectionReason)
			if r == "" {
				r = strings.TrimSpace(d.SkippedReason)
			}
			if r == "" {
				r = strings.TrimSpace(d.Candidate.Reason)
			}
			if r == "" {
				continue
			}
			side.SelectionReasons[r]++
		}
		return
	}
	for _, c := range sel.RankedCandidates {
		r := strings.TrimSpace(c.Reason)
		if r == "" {
			r = selection.ReasonRankTop
		}
		side.SelectionReasons[r]++
	}
}

func enrichFromV2(legacy, port *SideDayMetrics, v2 *decisionshadowv2.Report) {
	if v2 == nil {
		return
	}
	if v2.LegacyPlanProjection.BookProjection != nil {
		bp := v2.LegacyPlanProjection.BookProjection
		if bp.SectorAvailable {
			legacy.SectorAvailable = true
			legacy.SectorExposure = toSectorRows(bp.PostSectorExposure)
			legacy.MaxSectorWeight = maxSectorRow(legacy.SectorExposure)
		}
		if bp.PostCashRatio >= 0 {
			cr := bp.PostCashRatio
			legacy.CashRatio = &cr
		}
		if bp.PostTop1Weight > 0 {
			legacy.Top1Weight = bp.PostTop1Weight
		}
		if bp.PostGrossExposure > 0 {
			legacy.GrossExposure = bp.PostGrossExposure
		}
	}
	if v2.PortfolioPlanProjection.BookProjection != nil {
		bp := v2.PortfolioPlanProjection.BookProjection
		if bp.SectorAvailable {
			port.SectorAvailable = true
			port.SectorExposure = toSectorRows(bp.PostSectorExposure)
			port.MaxSectorWeight = maxSectorRow(port.SectorExposure)
		}
		if bp.PostCashRatio >= 0 {
			cr := bp.PostCashRatio
			port.CashRatio = &cr
		}
		if bp.PostTop1Weight > 0 {
			port.Top1Weight = bp.PostTop1Weight
		}
		if bp.PostGrossExposure > 0 {
			port.GrossExposure = bp.PostGrossExposure
		}
	}
	for _, line := range v2.PortfolioPlanProjection.Lines {
		if r := strings.TrimSpace(line.AllocationReason); r != "" {
			port.AllocationReasons[r]++
		}
		if r := strings.TrimSpace(line.RiskBinding); r != "" {
			port.RiskReasons["binding:"+r]++
		}
	}
	for _, n := range v2.ChainTrace.TightenNotes {
		n = strings.TrimSpace(n)
		if n != "" {
			port.RiskReasons["v2_tighten:"+n]++
			port.TightenReasons = appendUnique(port.TightenReasons, n)
			port.TightenCount = len(port.TightenReasons)
			port.TightenApplied = true
		}
	}
}

func toSectorRows(in []decisionshadowv2.SectorExposure) []SectorWeightRow {
	out := make([]SectorWeightRow, 0, len(in))
	for _, s := range in {
		out = append(out, SectorWeightRow{Industry: s.Industry, Weight: s.Weight})
	}
	return out
}

func maxSectorRow(rows []SectorWeightRow) float64 {
	top := 0.0
	for _, r := range rows {
		if r.Weight > top {
			top = r.Weight
		}
	}
	return top
}

func projectGross(snap *portfoliolayer.PortfolioSnapshot, buyNotional float64) float64 {
	if snap == nil || !snap.Found || snap.Equity <= 0 {
		return 0
	}
	return (snap.Exposure + buyNotional) / snap.Equity
}

func projectCashRatio(snap *portfoliolayer.PortfolioSnapshot, buyNotional float64) (float64, bool) {
	if snap == nil || !snap.Found || snap.Equity <= 0 {
		return 0, false
	}
	remain := snap.Cash - buyNotional
	if remain < 0 {
		remain = 0
	}
	return remain / snap.Equity, true
}

func projectTopWeights(snap *portfoliolayer.PortfolioSnapshot, res *portfoliosim.SimulatedPortfolioDecisionResult) (top1, top5 float64) {
	if snap == nil || !snap.Found || snap.Equity <= 0 {
		return 0, 0
	}
	mv := map[string]float64{}
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code == "" {
			continue
		}
		mv[code] += p.MarketValue
	}
	if res != nil && res.Allocation != nil {
		for _, it := range res.Allocation.Items {
			if !it.InAllocationSet || it.TargetAmount <= 0 {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(it.StockCode))
			mv[code] += it.TargetAmount
		}
	}
	weights := make([]float64, 0, len(mv))
	for _, v := range mv {
		weights = append(weights, v/snap.Equity)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(weights)))
	if len(weights) > 0 {
		top1 = weights[0]
	}
	n := 5
	if len(weights) < n {
		n = len(weights)
	}
	for i := 0; i < n; i++ {
		top5 += weights[i]
	}
	return top1, top5
}

func projectSectors(
	snap *portfoliolayer.PortfolioSnapshot,
	res *portfoliosim.SimulatedPortfolioDecisionResult,
	industry map[string]string,
) (available bool, rows []SectorWeightRow, maxW float64) {
	if snap == nil || !snap.Found || snap.Equity <= 0 {
		return false, nil, 0
	}
	indOf := map[string]string{}
	for k, v := range industry {
		indOf[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code == "" {
			continue
		}
		if _, ok := indOf[code]; !ok && strings.TrimSpace(p.Industry) != "" {
			indOf[code] = strings.TrimSpace(p.Industry)
		}
	}
	// require all held + buy names classified
	need := map[string]struct{}{}
	for _, p := range snap.Positions {
		if p.Volume <= 0 && p.MarketValue <= 0 {
			continue
		}
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code != "" {
			need[code] = struct{}{}
		}
	}
	if res != nil && res.Allocation != nil {
		for _, it := range res.Allocation.Items {
			if !it.InAllocationSet || it.TargetAmount <= 0 {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(it.StockCode))
			if code != "" {
				need[code] = struct{}{}
			}
		}
	}
	for code := range need {
		if strings.TrimSpace(indOf[code]) == "" {
			return false, nil, 0
		}
	}
	secMV := map[string]float64{}
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		ind := indOf[code]
		if ind == "" {
			continue
		}
		secMV[ind] += p.MarketValue
	}
	if res != nil && res.Allocation != nil {
		for _, it := range res.Allocation.Items {
			if !it.InAllocationSet || it.TargetAmount <= 0 {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(it.StockCode))
			ind := indOf[code]
			if ind == "" {
				continue
			}
			secMV[ind] += it.TargetAmount
		}
	}
	keys := make([]string, 0, len(secMV))
	for k := range secMV {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows = make([]SectorWeightRow, 0, len(keys))
	for _, k := range keys {
		w := secMV[k] / snap.Equity
		rows = append(rows, SectorWeightRow{Industry: k, Weight: w})
		if w > maxW {
			maxW = w
		}
	}
	return true, rows, maxW
}

func isBudgetCut(binding string) bool {
	b := strings.ToUpper(strings.TrimSpace(binding))
	switch b {
	case "", "OK", "NONE", "CASH", "LEGACY_FIXED_AMOUNT":
		return false
	case "GROSS", "BLOCKED", "NO_ACCOUNT", "GROSS_HEADROOM", "RISK_BUDGET", "SINGLE_CAP", "RESERVE", "MIN_ORDER":
		return true
	default:
		lb := strings.ToLower(b)
		return strings.Contains(lb, "cut") || strings.Contains(lb, "cap") ||
			strings.Contains(lb, "headroom") || strings.Contains(lb, "tighten") ||
			strings.Contains(lb, "gross") || strings.Contains(lb, "block")
	}
}

func legacyBasketBudget(day portfoliovalidation.DayCase, amt float64) *portfoliolayer.AllocationBudget {
	n := 0
	if day.Candidates != nil {
		n = day.Candidates.SelectionLimit
		if n <= 0 {
			n = 5
		}
		if len(day.Candidates.RankedCandidates) < n {
			n = len(day.Candidates.RankedCandidates)
		}
	}
	if n <= 0 {
		n = 1
	}
	cap := amt * float64(n)
	return &portfoliolayer.AllocationBudget{
		AvailableCapital: cap,
		AvailableCash:    cap,
		Binding:          "legacy_fixed_amount",
	}
}

func ceilingOr(day portfoliovalidation.DayCase) portfoliolayer.ConstraintSet {
	if day.Ceiling.Risk.Enabled || day.Ceiling.Risk.MaxGrossExposurePct != nil || day.Ceiling.Risk.MaxSingleNamePct != nil {
		return day.Ceiling
	}
	return day.Constraints
}

func selectionLimit(day portfoliovalidation.DayCase) int {
	if day.Candidates != nil && day.Candidates.SelectionLimit > 0 {
		return day.Candidates.SelectionLimit
	}
	return 5
}

func mergeIndustry(day portfoliovalidation.DayCase) map[string]string {
	out := map[string]string{}
	for k, v := range day.IndustryBySymbol {
		kk := strings.ToLower(strings.TrimSpace(k))
		vv := strings.TrimSpace(v)
		if kk != "" && vv != "" {
			out[kk] = vv
		}
	}
	if day.Snapshot != nil {
		for _, p := range day.Snapshot.Positions {
			code := strings.ToLower(strings.TrimSpace(p.StockCode))
			ind := strings.TrimSpace(p.Industry)
			if code != "" && ind != "" {
				if _, ok := out[code]; !ok {
					out[code] = ind
				}
			}
		}
	}
	if day.Candidates != nil {
		for _, c := range day.Candidates.RankedCandidates {
			code := strings.ToLower(strings.TrimSpace(c.StockCode))
			ind := strings.TrimSpace(c.Industry)
			if code != "" && ind != "" {
				if _, ok := out[code]; !ok {
					out[code] = ind
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appendUnique(xs []string, add string) []string {
	add = strings.TrimSpace(add)
	if add == "" {
		return xs
	}
	for _, x := range xs {
		if x == add {
			return xs
		}
	}
	return append(xs, add)
}
