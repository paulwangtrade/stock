// Package projection assembles read-only OpportunityProjection views (Phase16-C0).
// It does not write DB, trigger scans, or participate in TradePlan generation.
package projection

import "time"

const (
	QualityComplete = "complete"
	QualityPartial  = "partial"

	DecisionStatusBuyCandidate = "BUY_CANDIDATE"
	DecisionStatusWatch        = "WATCH"
	DecisionStatusReject       = "REJECT"
	DecisionStatusNotInPlan    = "NOT_IN_PLAN"
	DecisionStatusUnknown      = "UNKNOWN"

	CandidateStatusInPool        = "IN_POOL"
	CandidateStatusNotInPool     = "NOT_IN_POOL"
	CandidateStatusPlanPending   = "PLAN_PENDING"
	CandidateStatusPlanSkipped   = "PLAN_SKIPPED"
	CandidateStatusPlanFilled    = "PLAN_FILLED"

	HoldingStatusHeld    = "HELD"
	HoldingStatusNotHeld = "NOT_HELD"

	SourceTypePool     = "candidate_pool"
	SourceTypeSignal   = "signal_snapshot"
	SourceTypeResearch = "research_candidate"
)

// OpportunityProjection is the unified read model for Signal → Opportunity → Decision → TradePlan → Portfolio.
type OpportunityProjection struct {
	OpportunityID string `json:"opportunity_id"`
	StockCode     string `json:"stock_code"`
	StockName     string `json:"stock_name,omitempty"`
	TradeDate     string `json:"trade_date"`

	Signal      SignalBlock      `json:"signal"`
	Opportunity OpportunityBlock `json:"opportunity"`
	Decision    DecisionBlock    `json:"decision"`
	TradePlan   TradePlanBlock   `json:"trade_plan"`
	Portfolio   PortfolioBlock   `json:"portfolio"`
	Research    *ResearchBlock   `json:"research,omitempty"`
	Metadata    MetadataBlock    `json:"metadata"`
}

// SignalBlock — Layer 1; fields from SignalScanSnapshot hits only.
type SignalBlock struct {
	Present       bool    `json:"present"`
	SnapshotID    uint    `json:"snapshot_id,omitempty"`
	Session       string  `json:"session,omitempty"`
	StrategyID    string  `json:"strategy_id,omitempty"`
	SignalTag     string  `json:"signal_tag,omitempty"`
	SignalPrice   float64 `json:"signal_price,omitempty"`
	SignalTime    string  `json:"signal_time,omitempty"`
	SignalStatus  string  `json:"signal_status,omitempty"`
	TriggerReason string  `json:"trigger_reason,omitempty"`
	SchemaVersion string  `json:"schema_version,omitempty"`
}

// OpportunityBlock — Layer 2; Score/Rank authoritative from CandidatePoolItem only.
type OpportunityBlock struct {
	Present        bool    `json:"present"`
	PoolID         uint    `json:"pool_id,omitempty"`
	Score          float64 `json:"score,omitempty"`
	Rank           int     `json:"rank,omitempty"`
	StrategySource string  `json:"strategy_source,omitempty"`
	StrategyName   string  `json:"strategy_name,omitempty"`
	PoolSource     string  `json:"pool_source,omitempty"`
}

// DecisionBlock — Layer 3; derived from TradePlanItem when present.
type DecisionBlock struct {
	DecisionStatus  string  `json:"decision_status"`
	CandidateStatus string  `json:"candidate_status"`
	PlanID          uint    `json:"plan_id,omitempty"`
	PlanItemID      uint    `json:"plan_item_id,omitempty"`
	ItemStatus      string  `json:"item_status,omitempty"`
	RiskCode        string  `json:"risk_code,omitempty"`
	TargetAmount    float64 `json:"target_amount,omitempty"`
	DecisionProvider string `json:"decision_provider,omitempty"`
}

// TradePlanBlock — Layer 4.
type TradePlanBlock struct {
	Present    bool   `json:"present"`
	PlanID     uint   `json:"plan_id,omitempty"`
	PlanStatus string `json:"plan_status,omitempty"`
	ItemStatus string `json:"item_status,omitempty"`
	TradeDate  string `json:"trade_date,omitempty"`
	Frozen     bool   `json:"frozen"`
}

// PortfolioBlock — holding projection from portfolio snapshot.
type PortfolioBlock struct {
	HoldingStatus string  `json:"holding_status"`
	PositionQty   float64 `json:"position_qty,omitempty"`
}

// ResearchBlock — optional overlay; never overrides Opportunity Score/Rank.
type ResearchBlock struct {
	ResearchID   string   `json:"research_id"`
	Status       string   `json:"status,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	SignalScore  *float64 `json:"signal_score,omitempty"`
	ExplainSummary string `json:"explain_summary,omitempty"`
}

// MetadataBlock aggregates projection provenance.
type MetadataBlock struct {
	SourceType string    `json:"source_type"`
	Quality    string    `json:"quality"`
	Missing    []string  `json:"missing,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProjectOptions selects read inputs for one or many projections.
type ProjectOptions struct {
	TradeDate       string
	StockCode       string
	Session         string
	PlanID          uint
	IncludeResearch bool
	Limit           int
}
