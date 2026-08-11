package rebalance

import (
	"strings"

	"go-stock/backend/portfolio"
)

// CurrentFromSnapshot projects Snapshot → CurrentView (snapshot_mark basis).
func CurrentFromSnapshot(snap *portfolio.Snapshot) *CurrentView {
	out := &CurrentView{
		PriceBasis: PriceBasisSnapshotMark,
		Positions:  []CurrentPosition{},
	}
	if snap == nil {
		return out
	}
	out.AsOf = snap.AsOf
	out.AccountID = snap.AccountID
	out.Equity = snap.TotalEquity
	out.Cash = snap.Cash
	out.Exposure = snap.TotalExposure
	for _, p := range snap.Positions {
		code := strings.TrimSpace(p.StockCode)
		if code == "" || p.Volume <= 0 {
			continue
		}
		out.Positions = append(out.Positions, CurrentPosition{
			Symbol:          code,
			Volume:          p.Volume,
			AvailableVolume: p.AvailableVolume,
			LockedVolume:    p.LockedVolume,
			MarketValue:     p.MarketValue,
			Weight:          p.Weight,
		})
	}
	return out
}

// AttachDecisions copies stock-level decision states onto current positions.
func AttachDecisions(current *CurrentView, bySymbol map[string]string) {
	if current == nil || bySymbol == nil {
		return
	}
	for i := range current.Positions {
		if st, ok := bySymbol[current.Positions[i].Symbol]; ok {
			current.Positions[i].DecisionState = st
		}
	}
}
