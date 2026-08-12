// Package portfolioobs is the read-only Portfolio Management Observation layer (Phase10-E.2).
//
// Aggregates Snapshot + Evaluation + Decision + Rebalance Diff. Does not trade, write DB, or recalculate those engines.
package portfolioobs

const Disclaimer = "观察结果不是交易建议。不生成买卖单，不执行调仓。"

const actionNone = "none"

// AccountSummary is Snapshot-backed account facts (persisted mark, not overlay).
type AccountSummary struct {
	TotalEquity   float64 `json:"total_equity"`
	Cash          float64 `json:"cash"`
	MarketValue   float64 `json:"market_value"`
	Exposure      float64 `json:"exposure"`
	PositionCount int     `json:"position_count"`
	Found         bool    `json:"found"`
	AccountID     uint    `json:"account_id,omitempty"`
	AccountName   string  `json:"account_name,omitempty"`
}

// DecisionSummary is Holding Decision histogram + overlay market-value weights.
type DecisionSummary struct {
	NormalCount         int     `json:"normal_count"`
	WatchCount          int     `json:"watch_count"`
	ReviewCount         int     `json:"review_count"`
	ExitCandidateCount  int     `json:"exit_candidate_count"`
	NormalWeight        float64 `json:"normal_weight"`
	WatchWeight         float64 `json:"watch_weight"`
	ReviewWeight        float64 `json:"review_weight"`
	ExitCandidateWeight float64 `json:"exit_candidate_weight"`
	WeightBasis         string  `json:"weight_basis,omitempty"`
}

// RebalanceSummary is Diff action counts (not orders).
type RebalanceSummary struct {
	KeepCount             int    `json:"keep_count"`
	AddCount              int    `json:"add_count"`
	IncreaseCount         int    `json:"increase_count"`
	DecreaseCount         int    `json:"decrease_count"`
	RemoveCount           int    `json:"remove_count"`
	CurrentPositionCount  int    `json:"current_position_count"`
	TargetPositionCount   int    `json:"target_position_count"`
	BlockedSwitchCount    int    `json:"blocked_switch_count"`
	Available             bool   `json:"available"`
}

// PositionRow is one symbol's joined observation (action always none).
type PositionRow struct {
	Symbol           string   `json:"symbol"`
	StockName        string   `json:"stock_name,omitempty"`
	CurrentWeight    float64  `json:"current_weight"`
	CurrentAmount    float64  `json:"current_amount"`
	TargetWeight     float64  `json:"target_weight"`
	TargetAmount     float64  `json:"target_amount"`
	DeltaWeight      float64  `json:"delta_weight"`
	Cost             *float64 `json:"cost,omitempty"`
	CurrentPrice     *float64 `json:"current_price,omitempty"`
	PnL              *float64 `json:"pnl,omitempty"`
	Return           *float64 `json:"return,omitempty"`
	HoldingDays      int      `json:"holding_days,omitempty"`
	DecisionState    string   `json:"decision_state,omitempty"`
	DecisionReason   string   `json:"decision_reason,omitempty"`
	RebalanceAction  string   `json:"rebalance_action,omitempty"`
	RebalanceReason  string   `json:"rebalance_reason,omitempty"`
	Action           string   `json:"action"`
}

// Observation is the unified Portfolio Management Observation DTO.
type Observation struct {
	AsOf             string           `json:"as_of,omitempty"`
	ObservationTime  string           `json:"observation_time"`
	Account          AccountSummary   `json:"account"`
	Decision         DecisionSummary  `json:"decision"`
	Rebalance        RebalanceSummary `json:"rebalance"`
	Positions        []PositionRow    `json:"positions"`
	Disclaimer       string           `json:"disclaimer"`
	Action           string           `json:"action"`
	Warnings         []string         `json:"warnings,omitempty"`
	DataSourceNotes  map[string]string `json:"data_source_notes"`
	RebalanceRunID   string           `json:"rebalance_run_id,omitempty"`
	PortfolioSnapshotID string        `json:"portfolio_snapshot_id,omitempty"`
}
