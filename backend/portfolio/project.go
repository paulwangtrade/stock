package portfolio

import (
	"strings"
	"time"

	"go-stock/backend/papertrading"
)

// ProjectSnapshot builds a Snapshot from already-loaded paper_sim rows.
// Pure: no DB I/O, no writes. Market value is Σ mark_price×volume (persisted mark, not overlay).
func ProjectSnapshot(asOf time.Time, acc *papertrading.PaperSimAccount, positions []papertrading.PaperSimPosition) *Snapshot {
	if asOf.IsZero() {
		asOf = time.Now()
	}
	if acc == nil {
		return emptySnapshot(asOf, defaultAccountName)
	}

	out := &Snapshot{
		AsOf:           asOf,
		AccountID:      acc.ID,
		AccountName:    strings.TrimSpace(acc.Name),
		Cash:           acc.Cash,
		AvailableCash:  acc.Cash, // MVP: no reserved-cash ledger
		ReservedCash:   0,
		Found:          true,
		DataSourceNote: dataSourceNote,
	}

	names := make([]Position, 0, len(positions))
	mv := 0.0
	count := 0
	for _, p := range positions {
		vol := p.TotalVolume
		if vol <= 0 {
			continue
		}
		rowMV := p.MarkPrice * float64(vol)
		mv += rowMV
		count++
		names = append(names, Position{
			StockCode:       strings.TrimSpace(p.StockCode),
			StockName:       strings.TrimSpace(p.StockName),
			Volume:          vol,
			AvailableVolume: p.AvailableVolume,
			LockedVolume:    p.LockedVolume,
			AvgCost:         p.AvgCost,
			MarkPrice:       p.MarkPrice,
			MarketValue:     rowMV,
			UnrealizedPnL:   (p.MarkPrice - p.AvgCost) * float64(vol),
		})
	}

	out.MarketValue = mv
	out.TotalExposure = mv // long-only gross
	out.PositionCount = count
	out.TotalEquity = out.Cash + mv
	if out.TotalEquity > 0 {
		for i := range names {
			names[i].Weight = names[i].MarketValue / out.TotalEquity
		}
	}
	out.Positions = names
	return out
}
