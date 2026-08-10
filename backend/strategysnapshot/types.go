// Package strategysnapshot provides a read-only Strategy Snapshot bypass for explainability.
//
// Phase11-G: capture after TradePlan create / strategy evaluation.
// Snapshots never drive Buy/Sell, Broker, Gateway, Execution, Fill, or Settlement.
package strategysnapshot

import "time"

// SchemaVersion is the Strategy Snapshot contract version (Phase11-E).
const SchemaVersion = "sshot-1"

// Scope identifies snapshot granularity.
type Scope string

const (
	ScopePlan Scope = "plan"
	ScopeItem Scope = "item"
)

// Trigger names where capture is allowed (audit only).
const (
	TriggerTradePlanCreate     = "trade_plan_create"
	TriggerStrategyEvaluation  = "strategy_evaluation"
)

// StrategySnapshot is the unified read-only explainability record.
type StrategySnapshot struct {
	SnapshotID   string    `json:"snapshot_id"`
	Scope        Scope     `json:"scope"`
	PlanID       uint      `json:"plan_id"`
	PlanItemID   uint      `json:"plan_item_id,omitempty"`
	TradeDate    string    `json:"trade_date"`
	PlanVersion  int       `json:"plan_version"`
	CapturedAt   time.Time `json:"captured_at"`
	SchemaVersion string   `json:"schema_version"`
	Trigger      string    `json:"trigger,omitempty"`

	StrategyVersion StrategyVersion      `json:"strategy_version"`
	Parameters      Parameters           `json:"parameters"`
	MarketDataRef   MarketDataReference  `json:"market_data_ref"`
	SignalResult    SignalResult         `json:"signal_result"`
	RiskDecision    RiskDecision         `json:"risk_decision"`
}

// StrategyVersion identifies which strategy produced the thesis.
type StrategyVersion struct {
	StrategyID   string `json:"strategy_id,omitempty"`
	StrategyName string `json:"strategy_name,omitempty"`
	Version      string `json:"version,omitempty"`
	Source       string `json:"source,omitempty"`
	SourceRef    string `json:"source_ref,omitempty"`
}

// Parameters freezes generation-time knobs (not live strategy reload).
type Parameters struct {
	Values          map[string]any `json:"values,omitempty"`
	ParamsHash      string         `json:"params_hash,omitempty"`
	SignalParamsRef string         `json:"signal_params_ref,omitempty"`
	PoolConfigRef   map[string]any `json:"pool_config_ref,omitempty"`
}

// MarketDataReference points at price facts used for intent (stored fields, not live MDS).
type MarketDataReference struct {
	RefPrice      float64 `json:"ref_price,omitempty"`
	RefSource     string  `json:"ref_source,omitempty"`
	RefAsOf       string  `json:"ref_as_of,omitempty"`
	OpenRefPrice  float64 `json:"open_ref_price,omitempty"`
	LimitPrice    float64 `json:"limit_price,omitempty"`
	PricingStage  string  `json:"pricing_stage,omitempty"`
	BarAsOfDate   string  `json:"bar_as_of_date,omitempty"`
	ProviderHint  string  `json:"provider_hint,omitempty"`
	Unavailable   bool    `json:"unavailable,omitempty"`
	MissingReason string  `json:"missing_reason,omitempty"`
}

// SignalResult summarizes or references a SignalScanSnapshot.
type SignalResult struct {
	SignalSnapshotID uint    `json:"signal_snapshot_id,omitempty"`
	TradeDate        string  `json:"trade_date,omitempty"`
	Session          string  `json:"session,omitempty"`
	Tag              string  `json:"tag,omitempty"`
	Score            float64 `json:"score,omitempty"`
	Unavailable      bool    `json:"unavailable,omitempty"`
	MissingReason    string  `json:"missing_reason,omitempty"`
}

// RiskDecision archives plan/item risk filter + shallow approve metadata.
type RiskDecision struct {
	RiskStatus         string         `json:"risk_status,omitempty"`
	MarketLevel        int            `json:"market_level,omitempty"`
	RiskFilteredCount  int            `json:"risk_filtered_count,omitempty"`
	RiskAcceptedCount  int            `json:"risk_accepted_count,omitempty"`
	RiskSummary        string         `json:"risk_summary,omitempty"`
	PlanSnapshot       map[string]any `json:"plan_snapshot,omitempty"`
	ItemAccepted       *bool          `json:"item_accepted,omitempty"`
	RiskCode           string         `json:"risk_code,omitempty"`
	RiskMessage        string         `json:"risk_message,omitempty"`
	ApprovedAt         *time.Time     `json:"approved_at,omitempty"`
	ApprovedBy         string         `json:"approved_by,omitempty"`
	ApprovedSource     string         `json:"approved_source,omitempty"`
	Unavailable        bool           `json:"unavailable,omitempty"`
	MissingReason      string         `json:"missing_reason,omitempty"`
}

// PlanReference links a TradePlan to captured snapshot IDs (lightweight index).
type PlanReference struct {
	PlanID           uint              `json:"plan_id"`
	TradeDate        string            `json:"trade_date"`
	PlanVersion      int               `json:"plan_version"`
	PlanSnapshotID   string            `json:"plan_snapshot_id,omitempty"`
	ItemSnapshotIDs  map[uint]string   `json:"item_snapshot_ids,omitempty"` // plan_item_id → snapshot_id
	CapturedAt       time.Time         `json:"captured_at"`
	Trigger          string            `json:"trigger"`
}
