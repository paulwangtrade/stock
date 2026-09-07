package riskallocshadow

import (
	"go-stock/backend/allocationengine"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
)

func budgetView(b allocationengine.AllocationBudget) BudgetView {
	return BudgetView{
		AvailableCash:    b.AvailableCash,
		ReserveCash:      b.ReserveCash,
		RiskBudget:       b.RiskBudget,
		AvailableCapital: b.AvailableCapital,
		Binding:          b.Binding,
		PolicyGrossPct:   b.PolicyGrossPct,
	}
}

func toEngineSnapshot(ledger *portfolio.Snapshot) *allocationengine.PortfolioSnapshot {
	if ledger == nil {
		return &allocationengine.PortfolioSnapshot{Found: false}
	}
	return &allocationengine.PortfolioSnapshot{
		Found:        ledger.Found,
		Equity:       ledger.TotalEquity,
		Cash:         ledger.Cash,
		ReservedCash: ledger.ReservedCash,
		Exposure:     ledger.TotalExposure,
	}
}

func toEnginePolicy(r portfoliolayer.ResolvedConstraints) allocationengine.ResolvedConstraints {
	return allocationengine.ResolvedConstraints{
		MaxGrossExposurePct: r.MaxGrossExposurePct,
		MaxSingleWeight:     r.MaxSingleWeight,
		ReserveCashRatio:    r.ReserveCashRatio,
		MinOrderAmount:      r.MinOrderAmount,
		BlockNewEntries:     r.RiskBlockNewEntries,
	}
}

func summarizeLegacy(sel allocationengine.SelectionResult, amount float64) AllocSideSummary {
	sum := amount * float64(len(sel.Selected))
	return AllocSideSummary{
		OK:               true,
		Method:           "fixed_amount",
		SelectedCount:    len(sel.Selected),
		WaitlistCount:    len(sel.Waitlist),
		UniformOrScalar:  amount,
		SumAllocationSet: sum,
		Budget: BudgetView{
			AvailableCapital: sum,
		},
	}
}

func summarizeEngine(a *allocationengine.AllocationResult, method string) AllocSideSummary {
	out := AllocSideSummary{Method: method, Budget: BudgetView{}}
	if a == nil {
		return out
	}
	out.OK = true
	out.Method = a.Method
	if out.Method == "" {
		out.Method = method
	}
	out.UniformOrScalar = a.UniformAmount
	out.Budget = budgetView(a.Budget)
	out.Binding = a.Budget.Binding
	out.SelectedCount = len(a.Allocated)
	out.WaitlistCount = len(a.Waitlist)
	for _, it := range a.Allocated {
		out.SumAllocationSet += it.TargetAmount
	}
	return out
}

func sectorChange(
	snap *portfoliorisk.PortfolioRiskSnapshot,
	base, adj portfoliolayer.ResolvedConstraints,
	sug portfoliorisk.TightenSuggestion,
) SectorConstraintChange {
	out := SectorConstraintChange{
		MaxSectorWeightBefore:   base.MaxSectorWeight,
		MaxSectorWeightAfter:    adj.MaxSectorWeight,
		MaxNamesPerSectorBefore: base.MaxNamesPerSector,
		MaxNamesPerSectorAfter:  adj.MaxNamesPerSector,
	}
	if snap == nil || !snap.Sector.Available {
		out.Available = false
		out.Applied = false
		out.Note = "sector_unavailable_no_sector_limit"
		return out
	}
	out.Available = true
	for _, n := range sug.Notes {
		if n.Field == "max_sector_weight" || n.Field == "max_names_per_sector" {
			out.Applied = true
			break
		}
	}
	return out
}

func diffAmounts(
	legacy AllocSideSummary,
	adj *allocationengine.AllocationResult,
	sel allocationengine.SelectionResult,
	legacyAmt float64,
) []AmountDiff {
	portMap := map[string]float64{}
	if adj != nil {
		for _, it := range adj.Items {
			portMap[it.StockCode] = it.TargetAmount
		}
	}
	out := []AmountDiff{}
	seen := map[string]bool{}
	add := func(sym string, legPresent bool) {
		if sym == "" || seen[sym] {
			return
		}
		seen[sym] = true
		d := AmountDiff{Symbol: sym}
		if legPresent {
			v := legacyAmt
			d.LegacyAmount = &v
		}
		if pv, ok := portMap[sym]; ok {
			cp := pv
			d.PortfolioAmount = &cp
		}
		switch {
		case d.LegacyAmount != nil && d.PortfolioAmount != nil:
			delta := *d.PortfolioAmount - *d.LegacyAmount
			d.Delta = &delta
			d.Kind = "both_present"
		case d.LegacyAmount != nil && d.PortfolioAmount == nil:
			d.Kind = "legacy_only"
		case d.LegacyAmount == nil && d.PortfolioAmount != nil:
			d.Kind = "portfolio_only"
		default:
			d.Kind = "both_absent"
		}
		if d.Kind == "both_present" && d.Delta != nil && *d.Delta == 0 {
			return
		}
		out = append(out, d)
	}
	for _, s := range sel.Selected {
		add(s, true)
	}
	for _, s := range sel.Waitlist {
		add(s, true) // Legacy waitlist still carries scalar in fixed_amount path
	}
	if adj != nil {
		for _, it := range adj.Items {
			add(it.StockCode, false)
		}
	}
	_ = legacy
	return out
}
