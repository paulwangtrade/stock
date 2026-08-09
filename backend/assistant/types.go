// Package assistant provides AI Analysis Assistant domain types and Context Builder.
//
// Phase13-C: AssistantContextBuilder — no external AI API, no trade advice.
// Phase13-F: AssistantResponse + provider.AIProvider (Mock only).
package assistant

import "time"

// ContextSchemaVersion is the AssistantContext contract version.
const ContextSchemaVersion = "assist-ctx-1"

// AssistantScene selects which fact subsets are primary.
type AssistantScene string

const (
	SceneStockAnalysis    AssistantScene = "stock_analysis"
	SceneTradePlanExplain AssistantScene = "trade_plan_explain"
	SceneRiskExplain      AssistantScene = "risk_explain"
	ScenePositionSummary  AssistantScene = "position_summary"
)

// AssistantContext is the read-only insight bundle for a future AI Provider.
type AssistantContext struct {
	SchemaVersion  string         `json:"schema_version"`
	Scene          AssistantScene `json:"scene"`
	BuiltAt        time.Time      `json:"built_at"`
	Status         string         `json:"status"` // ok | gated | degraded | failed
	GateReason     string         `json:"gate_reason,omitempty"`
	Facts          FactBundle     `json:"facts"`
	Missing        []MissingFact  `json:"missing,omitempty"`
	Redactions      []string       `json:"redactions,omitempty"`
	PromptSkeleton string         `json:"prompt_skeleton"`
	Degraded       bool           `json:"degraded,omitempty"`
}

// FactBundle holds structured read-only facts (no order instructions).
type FactBundle struct {
	Plan           *PlanFact           `json:"plan,omitempty"`
	StrategySnap   *StrategySnapFact   `json:"strategy_snap,omitempty"`
	Risk           *RiskFact           `json:"risk,omitempty"`
	Execution      *ExecutionFact      `json:"execution,omitempty"`
	Stock          *StockFact          `json:"stock,omitempty"`
	Disclaimers    []string            `json:"disclaimers"`
}

// PlanFact is a sanitized TradePlan projection.
type PlanFact struct {
	PlanID            uint   `json:"plan_id"`
	TradeDate         string `json:"trade_date"`
	PlanVersion       int    `json:"plan_version"`
	Status            string `json:"status"`
	SourceSession      string `json:"source_session,omitempty"`
	RiskStatus        string `json:"risk_status,omitempty"`
	RiskAcceptedCount int    `json:"risk_accepted_count,omitempty"`
	RiskFilteredCount int    `json:"risk_filtered_count,omitempty"`
	ItemCount         int    `json:"item_count,omitempty"`
	PricingStage      string `json:"pricing_stage,omitempty"`
}

// StrategySnapFact is a compact StrategySnapshot projection.
type StrategySnapFact struct {
	SnapshotID      string  `json:"snapshot_id"`
	Scope           string  `json:"scope"`
	PlanID          uint    `json:"plan_id"`
	PlanItemID      uint    `json:"plan_item_id,omitempty"`
	StrategyName    string  `json:"strategy_name,omitempty"`
	StrategyVersion string  `json:"strategy_version,omitempty"`
	SignalTag       string  `json:"signal_tag,omitempty"`
	SignalScore     float64 `json:"signal_score,omitempty"`
	RiskCode        string  `json:"risk_code,omitempty"`
	RefPrice        float64 `json:"ref_price,omitempty"`
	RefSource       string  `json:"ref_source,omitempty"`
}

// RiskFact projects RiskReport summary (not a trade gate).
type RiskFact struct {
	ReportID    string   `json:"report_id,omitempty"`
	Overall     int      `json:"overall_score"`
	Band        string   `json:"band"`
	DataQuality string   `json:"data_quality,omitempty"`
	FactorCodes []string `json:"factor_codes,omitempty"`
	WarningMsgs []string `json:"warning_msgs,omitempty"`
	SuggestMsgs []string `json:"suggest_msgs,omitempty"` // review/monitor only
}

// ExecutionFact projects ExecutionSummary (counts only).
type ExecutionFact struct {
	TradeDate    string  `json:"trade_date,omitempty"`
	TotalOrders  int     `json:"total_orders"`
	FilledOrders int     `json:"filled_orders"`
	FailedOrders int     `json:"failed_orders"`
	FillRate     float64 `json:"fill_rate"`
	DataNote     string  `json:"data_note,omitempty"`
}

// StockFact is a minimal stock identity (optional).
type StockFact struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

// MissingFact records an absent input (no invention).
type MissingFact struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}
