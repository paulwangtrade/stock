// Execution Summary — read-only order/fill aggregates (Phase10-F.2 / H.2).
package papertrading

// ExecutionSummaryView is Observation execution totals (no price mutation).
type ExecutionSummaryView struct {
	Enabled        bool     `json:"enabled"`
	TradeDate      string   `json:"trade_date,omitempty"`
	TotalOrders    int      `json:"total_orders"`
	FilledOrders   int      `json:"filled_orders"`
	FailedOrders   int      `json:"failed_orders"`
	FillRate       float64  `json:"fill_rate"`
	AvgSlippage    *float64 `json:"avg_slippage"` // null without benchmark
	DataSourceNote string   `json:"data_source_note"`
}

// BuildExecutionSummary aggregates execution facts via ExecutionReadService (paper_sim primary).
func BuildExecutionSummary(tradeDate string) (*ExecutionSummaryView, error) {
	return DefaultExecutionReadService().BuildExecutionSummary(tradeDate)
}
