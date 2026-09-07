package portfolioevaluation

func aggregate(days []DayEvaluation) (DecisionStability, RiskBehavior, ConstraintBehavior, Explainability) {
	stab := DecisionStability{}
	risk := RiskBehavior{}
	cons := ConstraintBehavior{
		BudgetBindingTotals:          map[string]int{},
		FilterRejectReasonsLegacy:    map[string]int{},
		FilterRejectReasonsPortfolio: map[string]int{},
	}
	expl := Explainability{
		AllocationReasonTotals: map[string]int{},
		RiskReasonTotals:       map[string]int{},
		SelectionReasonTotals:  map[string]int{},
		InsightRiskLevelCounts: map[string]int{},
		Notes: []string{
			"explainability histograms are observational reason counts",
			"not trading advice; not auto-tune signals",
		},
	}

	legSelHist := map[int]int{}
	portSelHist := map[int]int{}
	waitDeltaHist := map[int]int{}

	var (
		legSel, portSel, selDelta               []int
		legAlloc, portAlloc                     []int
		legAmt, portAmt, amtDelta               []float64
		legWait, portWait, waitDelta            []float64
		legGross, portGross, grossDelta         []float64
		legCash, portCash, cashDelta            []float64
		legTop1, portTop1, top1Delta            []float64
		legTop5, portTop5, top5Delta            []float64
		legSec, portSec, secDelta               []float64
		cashComparable, sectorComparable        int
	)

	for _, d := range days {
		if !d.OK {
			continue
		}
		L, P := d.Legacy, d.Portfolio

		legSelHist[L.SelectedCount]++
		portSelHist[P.SelectedCount]++
		legSel = append(legSel, L.SelectedCount)
		portSel = append(portSel, P.SelectedCount)
		selDelta = append(selDelta, P.SelectedCount-L.SelectedCount)

		legAlloc = append(legAlloc, L.AllocationCount)
		portAlloc = append(portAlloc, P.AllocationCount)

		legAmt = append(legAmt, L.BuyNotional)
		portAmt = append(portAmt, P.BuyNotional)
		amtDelta = append(amtDelta, P.BuyNotional-L.BuyNotional)

		legWait = append(legWait, float64(L.WaitlistCount))
		portWait = append(portWait, float64(P.WaitlistCount))
		wd := P.WaitlistCount - L.WaitlistCount
		waitDelta = append(waitDelta, float64(wd))
		waitDeltaHist[wd]++

		legGross = append(legGross, L.GrossExposure)
		portGross = append(portGross, P.GrossExposure)
		grossDelta = append(grossDelta, P.GrossExposure-L.GrossExposure)

		if L.CashRatio != nil && P.CashRatio != nil {
			cashComparable++
			legCash = append(legCash, *L.CashRatio)
			portCash = append(portCash, *P.CashRatio)
			cashDelta = append(cashDelta, *P.CashRatio-*L.CashRatio)
		}

		legTop1 = append(legTop1, L.Top1Weight)
		portTop1 = append(portTop1, P.Top1Weight)
		top1Delta = append(top1Delta, P.Top1Weight-L.Top1Weight)
		legTop5 = append(legTop5, L.Top5Weight)
		portTop5 = append(portTop5, P.Top5Weight)
		top5Delta = append(top5Delta, P.Top5Weight-L.Top5Weight)

		if L.SectorAvailable && P.SectorAvailable {
			sectorComparable++
			legSec = append(legSec, L.MaxSectorWeight)
			portSec = append(portSec, P.MaxSectorWeight)
			secDelta = append(secDelta, P.MaxSectorWeight-L.MaxSectorWeight)
		} else {
			risk.SectorUnavailableDayCount++
		}

		cons.RiskTightenCountLegacy += L.TightenCount
		cons.RiskTightenCountPortfolio += P.TightenCount
		if P.TightenApplied || P.TightenCount > 0 {
			cons.RiskTightenDaysPortfolio++
		}
		if L.BudgetCut {
			cons.BudgetCutDaysLegacy++
		}
		if P.BudgetCut {
			cons.BudgetCutDaysPortfolio++
		}
		if P.BudgetBinding != "" {
			cons.BudgetBindingTotals[P.BudgetBinding]++
		}
		if L.BudgetBinding != "" {
			cons.BudgetBindingTotals["legacy:"+L.BudgetBinding]++
		}
		mergeCountMaps(cons.FilterRejectReasonsLegacy, L.FilterRejectReasons)
		mergeCountMaps(cons.FilterRejectReasonsPortfolio, P.FilterRejectReasons)

		mergeCountMaps(expl.AllocationReasonTotals, L.AllocationReasons)
		mergeCountMaps(expl.AllocationReasonTotals, P.AllocationReasons)
		mergeCountMaps(expl.RiskReasonTotals, L.RiskReasons)
		mergeCountMaps(expl.RiskReasonTotals, P.RiskReasons)
		mergeCountMaps(expl.SelectionReasonTotals, L.SelectionReasons)
		mergeCountMaps(expl.SelectionReasonTotals, P.SelectionReasons)

		if d.InsightAttached && d.InsightRiskLevel != "" {
			expl.InsightRiskLevelCounts[d.InsightRiskLevel]++
		}
	}

	stab.DailySelectedCountLegacy = binsFromInt(legSelHist)
	stab.DailySelectedCountPortfolio = binsFromInt(portSelHist)
	stab.SelectedCountLegacyMean = meanInts(legSel)
	stab.SelectedCountPortfolioMean = meanInts(portSel)
	stab.SelectedCountDeltaMean = meanInts(selDelta)
	stab.AllocationCountLegacyMean = meanInts(legAlloc)
	stab.AllocationCountPortfolioMean = meanInts(portAlloc)
	stab.AmountDistributionLegacy = scalarStats(legAmt)
	stab.AmountDistributionPortfolio = scalarStats(portAmt)
	stab.AmountDelta = scalarStats(amtDelta)
	stab.WaitlistLegacyMean = meanFloats(legWait)
	stab.WaitlistPortfolioMean = meanFloats(portWait)
	stab.WaitlistDeltaMean = meanFloats(waitDelta)
	stab.WaitlistDeltaBins = binsFromInt(waitDeltaHist)

	risk.GrossExposureLegacyMean = meanFloats(legGross)
	risk.GrossExposurePortfolioMean = meanFloats(portGross)
	risk.GrossExposureDelta = scalarStats(grossDelta)
	risk.CashRatioComparableDays = cashComparable
	risk.CashRatioLegacyMean = meanFloats(legCash)
	risk.CashRatioPortfolioMean = meanFloats(portCash)
	risk.CashRatioDelta = scalarStats(cashDelta)
	risk.Top1LegacyMean = meanFloats(legTop1)
	risk.Top1PortfolioMean = meanFloats(portTop1)
	risk.Top1Delta = scalarStats(top1Delta)
	risk.Top5LegacyMean = meanFloats(legTop5)
	risk.Top5PortfolioMean = meanFloats(portTop5)
	risk.Top5Delta = scalarStats(top5Delta)
	risk.SectorComparableDays = sectorComparable
	risk.MaxSectorLegacyMean = meanFloats(legSec)
	risk.MaxSectorPortfolioMean = meanFloats(portSec)
	risk.MaxSectorDelta = scalarStats(secDelta)

	return stab, risk, cons, expl
}
