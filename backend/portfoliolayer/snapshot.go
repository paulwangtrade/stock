package portfoliolayer

import (
	"strings"
	"time"

	"go-stock/backend/portfolio"
)

// PortfolioSnapshot is the F.1 read-only account projection used by this layer.
// It is a view over portfolio.Snapshot (the ledger). This package does not query DB.
type PortfolioSnapshot struct {
	AccountID     uint
	AsOf          time.Time
	Found         bool
	Equity        float64
	Cash          float64
	AvailableCash float64
	ReservedCash  float64
	MarketValue   float64
	Exposure      float64
	PositionCount int
	Positions     []SnapshotPosition
	source        *portfolio.Snapshot
}

// SnapshotPosition is one holding in the F.1 view. Industry is optional enrich; not written to ledger.
type SnapshotPosition struct {
	StockCode       string
	StockName       string
	Volume          int64
	AvailableVolume int64
	LockedVolume    int64
	MarketValue     float64
	Weight          float64
	Industry        string
}

// FromLedger maps the authoritative paper_sim snapshot into the F.1 view.
// A nil or missing ledger becomes Found=false (do not pretend an empty book is fully selectable).
func FromLedger(s *portfolio.Snapshot) *PortfolioSnapshot {
	if s == nil {
		return &PortfolioSnapshot{Found: false}
	}
	view := &PortfolioSnapshot{
		AccountID:     s.AccountID,
		AsOf:          s.AsOf,
		Found:         s.Found,
		Equity:        s.TotalEquity,
		Cash:          s.Cash,
		AvailableCash: s.AvailableCash,
		ReservedCash:  s.ReservedCash,
		MarketValue:   s.MarketValue,
		Exposure:      s.TotalExposure,
		PositionCount: s.PositionCount,
		source:        s,
	}
	if len(s.Positions) == 0 {
		return view
	}
	view.Positions = make([]SnapshotPosition, len(s.Positions))
	for i, p := range s.Positions {
		view.Positions[i] = SnapshotPosition{
			StockCode:       p.StockCode,
			StockName:       p.StockName,
			Volume:          p.Volume,
			AvailableVolume: p.AvailableVolume,
			LockedVolume:    p.LockedVolume,
			MarketValue:     p.MarketValue,
			Weight:          p.Weight,
		}
	}
	return view
}

// Ledger returns the source portfolio.Snapshot without inventing a second book formula.
// Test-constructed views reconstruct a compatible ledger for allocation.Budget.
func (s *PortfolioSnapshot) Ledger() *portfolio.Snapshot {
	if s == nil {
		return nil
	}
	if s.source != nil {
		return s.source
	}
	positions := make([]portfolio.Position, len(s.Positions))
	for i, p := range s.Positions {
		positions[i] = portfolio.Position{
			StockCode:       p.StockCode,
			StockName:       p.StockName,
			Volume:          p.Volume,
			AvailableVolume: p.AvailableVolume,
			LockedVolume:    p.LockedVolume,
			MarketValue:     p.MarketValue,
			Weight:          p.Weight,
		}
	}
	return &portfolio.Snapshot{
		AsOf:          s.AsOf,
		AccountID:     s.AccountID,
		Found:         s.Found,
		TotalEquity:   s.Equity,
		Cash:          s.Cash,
		AvailableCash: s.AvailableCash,
		ReservedCash:  s.ReservedCash,
		MarketValue:   s.MarketValue,
		TotalExposure: s.Exposure,
		PositionCount: s.PositionCount,
		Positions:     positions,
	}
}

// HoldingCodes returns codes with volume > 0. Empty when snapshot is missing.
func (s *PortfolioSnapshot) HoldingCodes() map[string]struct{} {
	out := make(map[string]struct{})
	if s == nil || !s.Found {
		return out
	}
	for _, p := range s.Positions {
		if p.Volume <= 0 {
			continue
		}
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code == "" {
			continue
		}
		out[code] = struct{}{}
	}
	return out
}

// SectorWeight sums position weights for an industry (0 if unset / missing snapshot).
func (s *PortfolioSnapshot) SectorWeight(industry string) float64 {
	if s == nil || !s.Found {
		return 0
	}
	want := strings.TrimSpace(industry)
	if want == "" {
		return 0
	}
	var sum float64
	for _, p := range s.Positions {
		if strings.TrimSpace(p.Industry) == want {
			sum += p.Weight
		}
	}
	return sum
}

func isUsableSnapshot(s *PortfolioSnapshot) bool {
	return s != nil && s.Found
}
