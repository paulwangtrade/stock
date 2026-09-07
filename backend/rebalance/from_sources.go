package rebalance

import (
	"strings"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfolio"
)

// CurrentPortfolioFromSnapshot projects portfolio.Snapshot → H4 CurrentPortfolio.
func CurrentPortfolioFromSnapshot(snap *portfolio.Snapshot) CurrentPortfolio {
	out := CurrentPortfolio{
		Source:    SourceSnapshot,
		Positions: []CurrentPortfolioPos{},
	}
	if snap == nil || !snap.Found {
		out.Found = false
		return out
	}
	out.Found = true
	out.AccountID = snap.AccountID
	out.Equity = snap.TotalEquity
	out.Cash = snap.Cash
	out.Exposure = snap.TotalExposure
	for _, p := range snap.Positions {
		code := strings.TrimSpace(p.StockCode)
		if code == "" || p.Volume <= 0 {
			continue
		}
		out.Positions = append(out.Positions, CurrentPortfolioPos{
			Symbol:      code,
			Qty:         p.Volume,
			MarketValue: p.MarketValue,
			Weight:      p.Weight,
		})
	}
	return out
}

// TargetFromAllocation builds TargetPortfolio weights from H3 AllocationResult (allocated set).
// TargetWeight = TargetAmount / equity. Does not write TradePlan or mutate holdings.
func TargetFromAllocation(result *allocationengine.AllocationResult, equity float64) TargetPortfolio {
	out := TargetPortfolio{
		EquityRef:        equity,
		Positions:        []TargetPosition{},
		ConstructionNote: "target from portfolio AllocationEngine · suggest_only; not a trade plan",
	}
	if result == nil || equity <= 0 {
		return out
	}
	items := result.Allocated
	if len(items) == 0 {
		items = result.Items
	}
	for _, it := range items {
		if !it.InAllocationSet && len(result.Allocated) > 0 {
			continue
		}
		code := strings.TrimSpace(it.StockCode)
		if code == "" || it.TargetAmount <= 0 {
			continue
		}
		w := it.TargetAmount / equity
		out.Positions = append(out.Positions, TargetPosition{
			Symbol:       code,
			TargetWeight: w,
			TargetAmount: it.TargetAmount,
			Source:       SourcePortfolioAllocation,
			Reason:       it.AllocationReason,
		})
	}
	var sumW float64
	for _, p := range out.Positions {
		sumW += p.TargetWeight
	}
	if sumW > 0 && sumW < 1 {
		out.CashBufferWeight = 1 - sumW
	}
	return out
}
