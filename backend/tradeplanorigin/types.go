package tradeplanorigin

const (
	// Missing is returned for projection fields with no resolvable source.
	Missing = "missing"
)

// ItemOrigin is a read-only per-stock projection from TradePlanItem + CandidatePool + signal snapshots.
type ItemOrigin struct {
	StockCode         string `json:"stock_code"`
	PlanID            string `json:"plan_id"`
	SignalTime        string `json:"signal_time"`
	SignalPrice       string `json:"signal_price"`
	SignalTag         string `json:"signal_tag"`
	SignalSnapshotID  uint   `json:"signal_snapshot_id,omitempty"`
	SourceReason      string `json:"source_reason"`
	SelectionReason   string `json:"selection_reason"`
	StrategyName      string `json:"strategy_name"`
	Score             string `json:"score"`
}
