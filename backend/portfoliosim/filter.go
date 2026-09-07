package portfoliosim

import (
	"strings"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/risk"
	"go-stock/backend/selection"
)

func runFilterCompat(
	snap *portfoliolayer.PortfolioSnapshot,
	resolved portfoliolayer.ResolvedConstraints,
	sel *portfoliolayer.PortfolioSelectionResult,
	alloc *portfoliolayer.AllocationResult,
	req FilterCompatRequest,
) SimulatedFilterResult {
	out := SimulatedFilterResult{
		Pending: []SimulatedFilterLine{},
		Skipped: []SimulatedFilterLine{},
	}
	maxNames := req.MaxNames
	if maxNames <= 0 {
		maxNames = resolved.MaxNewNames
	}
	if maxNames <= 0 {
		maxNames = portfoliolayer.DefaultMaxNewNames
	}
	scanLimit := req.ScanLimit
	if scanLimit <= 0 {
		scanLimit = DefaultScanLimit
	}
	out.MaxNames = maxNames
	out.ScanLimit = scanLimit

	if req.SkipCompatibilityCheck {
		out.Ran = false
		out.Status = FilterStatusNotRun
		return out
	}

	amountBy := map[string]portfoliolayer.NameAllocation{}
	if alloc != nil {
		for _, item := range alloc.Items {
			amountBy[norm(item.StockCode)] = item
		}
	}
	inSet := map[string]bool{}
	if sel != nil {
		for _, p := range sel.Selected {
			inSet[norm(p.Candidate.StockCode)] = true
		}
	}

	scan := []selection.Candidate{}
	if sel != nil {
		scan = sel.ScanList()
	}

	planCands := make([]risk.PlanCandidate, 0, len(scan))
	zeroSkip := map[int]SimulatedFilterLine{}

	for i, c := range scan {
		item, ok := amountBy[norm(c.StockCode)]
		amt := 0.0
		if ok {
			amt = item.TargetAmount
		}
		line := SimulatedFilterLine{
			StockCode:       c.StockCode,
			TargetAmount:    amt,
			InAllocationSet: inSet[norm(c.StockCode)],
		}
		if amt <= 0 {
			line.Status = "skipped"
			line.RiskCode = string(risk.ReasonInvalidOrder)
			zeroSkip[i] = line
			continue
		}
		planCands = append(planCands, risk.PlanCandidate{
			StockCode:    c.StockCode,
			StockName:    c.StockName,
			Rank:         c.Rank,
			Score:        c.Score,
			Reason:       c.Reason,
			TargetAmount: amt,
		})
	}

	ctx := buildPlanContext(snap, resolved, maxNames, scanLimit)
	got := risk.PlanFilter(planCands, ctx)

	out.Ran = true
	out.Status = FilterStatusCalled
	if got != nil {
		out.RiskStatus = got.RiskStatus
		filterByCode := map[string]risk.PlanFilterItem{}
		for _, it := range got.Items {
			filterByCode[norm(it.Candidate.StockCode)] = it
		}
		for i, c := range scan {
			if z, ok := zeroSkip[i]; ok {
				out.Skipped = append(out.Skipped, z)
				continue
			}
			it, ok := filterByCode[norm(c.StockCode)]
			if !ok {
				continue
			}
			line := SimulatedFilterLine{
				StockCode:       c.StockCode,
				TargetAmount:    it.Candidate.TargetAmount,
				Status:          it.Status,
				InAllocationSet: inSet[norm(c.StockCode)],
			}
			if !it.Allowed {
				line.RiskCode = string(it.RiskCode)
				out.Skipped = append(out.Skipped, line)
				continue
			}
			out.Pending = append(out.Pending, line)
		}
		out.AcceptedCount = len(out.Pending)
	}
	return out
}

func buildPlanContext(
	snap *portfoliolayer.PortfolioSnapshot,
	resolved portfoliolayer.ResolvedConstraints,
	maxNames, scanLimit int,
) risk.PlanContext {
	ctx := risk.PlanContext{
		Enabled:             true,
		MarketLevel:         resolved.RiskMarketLevel,
		BlockNewEntries:     resolved.RiskBlockNewEntries,
		MaxGrossExposurePct: resolved.MaxGrossExposurePct,
		MaxSingleNamePct:    resolved.MaxSingleWeight,
		MaxDailyLossPct:     resolved.RiskMaxDailyLossPct,
		MaxNames:            maxNames,
		ScanLimit:           scanLimit,
		NameMarketValue:     map[string]float64{},
	}
	if ctx.MarketLevel <= 0 {
		ctx.MarketLevel = 3
	}
	if snap != nil && snap.Found {
		// Same cash field as strategy.loadPlanFilterContext: snapshot.Cash, not AvailableCash.
		ctx.Cash = snap.Cash
		ctx.EquityBase = snap.Equity
		ctx.LongMarketValue = snap.MarketValue
		for _, p := range snap.Positions {
			code := norm(p.StockCode)
			if code == "" || p.Volume <= 0 {
				continue
			}
			ctx.NameMarketValue[code] = p.MarketValue
		}
		if ctx.EquityBase <= 0 {
			ctx.EquityBase = ctx.Cash + ctx.LongMarketValue
		}
	}
	return ctx
}

func norm(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
