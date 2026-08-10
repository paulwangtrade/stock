package papertrading

// ExecutionSummaryView is a read-only observation DTO used by Assistant / Risk product surfaces.
// Phase13-V.1 Beta tip ships the type only; full ExecutionReadService aggregation remains out of closure.
type ExecutionSummaryView struct {
	Enabled        bool     `json:"enabled"`
	TradeDate      string   `json:"trade_date,omitempty"`
	TotalOrders    int      `json:"total_orders"`
	FilledOrders   int      `json:"filled_orders"`
	FailedOrders   int      `json:"failed_orders"`
	FillRate       float64  `json:"fill_rate"`
	AvgSlippage    *float64 `json:"avg_slippage"`
	DataSourceNote string   `json:"data_source_note"`
}
