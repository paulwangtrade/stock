// Package portfolio provides a read-only Portfolio Snapshot layer (Phase10-D.2).
//
// It does not size orders, write paper_sim tables, or change TradePlan / Execution / Fill.
package portfolio

import "time"

const (
	defaultAccountName = "paper_sim_default"

	dataSourceNote = "Portfolio Snapshot · read-only paper_sim_accounts + paper_sim_positions.mark_price; no quote overlay; no sell; not a sizing input unless a future sizer opts in"
)

// Snapshot is the GET-time portfolio projection. No new DB columns.
type Snapshot struct {
	AsOf           time.Time  `json:"as_of"`
	AccountID      uint       `json:"account_id,omitempty"`
	AccountName    string     `json:"account_name,omitempty"`
	TotalEquity    float64    `json:"total_equity"`
	Cash           float64    `json:"cash"`
	MarketValue    float64    `json:"market_value"`
	PositionCount  int        `json:"position_count"`
	TotalExposure  float64    `json:"total_exposure"`
	AvailableCash  float64    `json:"available_cash"`
	ReservedCash   float64    `json:"reserved_cash"`
	Positions      []Position `json:"positions,omitempty"`
	Found          bool       `json:"found"`
	DataSourceNote string     `json:"data_source_note"`
}

// Position is one net holding in the snapshot (not a lot / not a sell target).
type Position struct {
	StockCode         string  `json:"stock_code"`
	StockName         string  `json:"stock_name,omitempty"`
	Volume            int64   `json:"volume"`
	AvailableVolume   int64   `json:"available_volume"`
	LockedVolume      int64   `json:"locked_volume"`
	AvgCost           float64 `json:"avg_cost"`
	MarkPrice         float64 `json:"mark_price"`
	MarketValue       float64 `json:"market_value"`
	Weight            float64 `json:"weight"` // market_value / total_equity; 0 if equity<=0
	UnrealizedPnL     float64 `json:"unrealized_pnl"`
}

// emptySnapshot is returned when no paper_sim account exists (does not create one).
func emptySnapshot(asOf time.Time, name string) *Snapshot {
	if asOf.IsZero() {
		asOf = time.Now()
	}
	return &Snapshot{
		AsOf:           asOf,
		AccountName:    name,
		Found:          false,
		DataSourceNote: dataSourceNote,
	}
}
