package portfoliolayer

import (
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/selection"
)

func i(v int) *int         { return &v }
func b(v bool) *bool       { return &v }
func f(v float64) *float64 { return &v }

func testLedger() *portfolio.Snapshot {
	return &portfolio.Snapshot{
		AsOf:          time.Date(2026, 8, 20, 15, 30, 0, 0, time.Local),
		AccountID:     1,
		AccountName:   "paper_sim_default",
		Found:         true,
		TotalEquity:   1_000_000,
		Cash:          800_000,
		AvailableCash: 800_000,
		ReservedCash:  0,
		MarketValue:   200_000,
		TotalExposure: 200_000,
		PositionCount: 1,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", StockName: "Ping An", Volume: 100, MarketValue: 200_000, Weight: 0.20},
		},
	}
}

func ranked(codes ...string) []selection.Candidate {
	out := make([]selection.Candidate, len(codes))
	for i, c := range codes {
		out[i] = selection.Candidate{StockCode: c, StockName: c, Rank: i + 1, Score: float64(100 - i)}
	}
	return out
}
