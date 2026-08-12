// Package portfolioobs is the read-only Portfolio Management Observation layer (Phase10-E.2).
//
// Aggregates Snapshot + Evaluation + Decision + Rebalance Diff. Does not trade, write DB, or recalculate those engines.
package portfolioobs

const Disclaimer = "观察结果不是交易建议。不生成买卖单，不执行调仓，不会自动卖出。"

const actionNone = "none"

// E.3 aging buckets (independent of Evaluation SHORT_TERM/MID_TERM/LONG_TERM 5/20).
const (
	AgingShort   = "SHORT"   // holding_days < 5
	AgingMedium  = "MEDIUM"  // 5–30 inclusive
	AgingLong    = "LONG"    // > 30
	AgingUnknown = "UNKNOWN" // missing buy date; never upgraded to LONG
)

const (
	HealthHealthy = "HEALTHY" // 80–100
	HealthNormal  = "NORMAL"  // 50–80
	HealthWatch   = "WATCH"   // 30–50
	HealthRisk    = "RISK"    // <30
	HealthUnknown = "UNKNOWN" // missing price+return; not a risk upgrade
)

const OpportunityCostUnknown = "UNKNOWN"

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
	Symbol               string                  `json:"symbol"`
	StockName            string                  `json:"stock_name,omitempty"`
	CurrentWeight        float64                 `json:"current_weight"`
	CurrentAmount        float64                 `json:"current_amount"`
	TargetWeight         float64                 `json:"target_weight"`
	TargetAmount         float64                 `json:"target_amount"`
	DeltaWeight          float64                 `json:"delta_weight"`
	Cost                 *float64                `json:"cost,omitempty"`
	CurrentPrice         *float64                `json:"current_price,omitempty"`
	PnL                  *float64                `json:"pnl,omitempty"`
	Return               *float64                `json:"return,omitempty"`
	HoldingDays          int                     `json:"holding_days,omitempty"`
	FirstBuyDate         string                  `json:"first_buy_date,omitempty"`
	HoldingPeriodBucket  string                  `json:"holding_period_bucket,omitempty"`
	IsAging              bool                    `json:"is_aging"`
	RiskState            string                  `json:"risk_state,omitempty"`
	ProfitState          string                  `json:"profit_state,omitempty"`
	HealthScore          *float64                `json:"health_score,omitempty"`
	HealthLevel          string                  `json:"health_level,omitempty"`
	DecisionState        string                  `json:"decision_state,omitempty"`
	DecisionReason       string                  `json:"decision_reason,omitempty"`
	DecisionHistory      []DecisionHistoryPoint  `json:"decision_history,omitempty"`
	DecisionHistoryNote  string                  `json:"decision_history_note,omitempty"`
	OpportunityCostLevel string                  `json:"opportunity_cost_level,omitempty"`
	RebalanceAction      string                  `json:"rebalance_action,omitempty"`
	RebalanceReason      string                  `json:"rebalance_reason,omitempty"`
	Action               string                  `json:"action"`
}

// DecisionHistoryPoint is one observed decision state at an as_of date (not a signal).
type DecisionHistoryPoint struct {
	AsOf   string `json:"as_of"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	Source string `json:"source,omitempty"` // current | injected
}

// DecisionHistory is a symbol's decision timeline projection.
type DecisionHistory struct {
	Symbol           string                 `json:"symbol"`
	Points           []DecisionHistoryPoint `json:"points"`
	TransitionCount  int                    `json:"transition_count"`
	Complete         bool                   `json:"complete"`
	Note             string                 `json:"note,omitempty"`
}

// PortfolioHealthSummary is portfolio-level observation counts (not trade advice).
type PortfolioHealthSummary struct {
	PortfolioHealth    string `json:"portfolio_health"`
	HealthyPositions   int    `json:"healthy_positions"`
	WatchPositions     int    `json:"watch_positions"`
	ReviewPositions    int    `json:"review_positions"`
	AgingPositions     int    `json:"aging_positions"`
	RiskPositions      int    `json:"risk_positions"`
	UnknownHealthCount int    `json:"unknown_health_count"`
}

// OpportunityCostView is E.3 stub only (no CandidatePool model).
type OpportunityCostView struct {
	Available bool   `json:"available"`
	Level     string `json:"level"`
	Note      string `json:"note"`
}

// Observation is the unified Portfolio Management Observation DTO.
type Observation struct {
	AsOf                string                 `json:"as_of,omitempty"`
	ObservationTime     string                 `json:"observation_time"`
	Account             AccountSummary         `json:"account"`
	Decision            DecisionSummary        `json:"decision"`
	Rebalance           RebalanceSummary       `json:"rebalance"`
	Health              PortfolioHealthSummary `json:"health"`
	OpportunityCost     OpportunityCostView    `json:"opportunity_cost"`
	Positions           []PositionRow          `json:"positions"`
	Disclaimer          string                 `json:"disclaimer"`
	Action              string                 `json:"action"`
	Warnings            []string               `json:"warnings,omitempty"`
	DataSourceNotes     map[string]string      `json:"data_source_notes"`
	RebalanceRunID      string                 `json:"rebalance_run_id,omitempty"`
	PortfolioSnapshotID string                 `json:"portfolio_snapshot_id,omitempty"`
}
