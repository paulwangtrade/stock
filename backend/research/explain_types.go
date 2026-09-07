package research

import "time"

// ResearchExplain schema (Phase13-A8 / A7 rexplain-1).
const ExplainSchemaVersion = "rexplain-1"

// Explain types (no llm_generated / execution types in MVP).
const (
	ExplainTypeSignalDerived    = "signal_derived"
	ExplainTypeManualAnnotation = "manual_annotation"
	ExplainTypeHybrid           = "hybrid"
	ExplainTypeUnavailable      = "unavailable"
)

// ResearchReason kinds.
const (
	ReasonKindRule         = "rule"
	ReasonKindSignalText   = "signal_text"
	ReasonKindAnalystNote  = "analyst_note"
	ReasonKindUnknown      = "unknown"
)

// Risk severities (research hint only — not QuantityPolicy / Gateway).
const (
	RiskSeverityInfo = "info"
	RiskSeverityWarn = "warn"
	RiskSeverityHigh = "high"
)

// ExplainEvidence is the minimal evidence slice (lives on Explain only).
type ExplainEvidence struct {
	AsOf             string   `json:"as_of,omitempty"`
	SignalSnapshotID *uint    `json:"signal_snapshot_id,omitempty"`
	Source           string   `json:"source,omitempty"`
	SourceRef        string   `json:"source_ref,omitempty"`
	SignalTag        string   `json:"signal_tag,omitempty"`
	SignalScore      *float64 `json:"signal_score,omitempty"`
	Direction        string   `json:"direction,omitempty"`
	Price            string   `json:"price,omitempty"`
	StatusText       string   `json:"status_text,omitempty"`
	ScoreSteps       []string `json:"score_steps,omitempty"`
	EvidenceHash     string   `json:"evidence_hash,omitempty"`
}

// ResearchReason answers "why research" (Explain-only; not Candidate inflation).
type ResearchReason struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// RiskNote is a research-side hint (not execution gate).
type RiskNote struct {
	Severity string `json:"severity"`
	Text     string `json:"text"`
}

// Explain is the independent Research Explain entity (A7/A8).
type Explain struct {
	ID                 string          `json:"id"`
	SchemaVersion      string          `json:"schema_version"`
	CandidateID        string          `json:"candidate_id"`
	TradeDate          string          `json:"trade_date,omitempty"`
	StockCode          string          `json:"stock_code,omitempty"`
	ExplainType        string          `json:"explain_type"`
	Available          bool            `json:"available"`
	MissingReason      string          `json:"missing_reason,omitempty"`
	Summary            string          `json:"summary"`
	Evidence           ExplainEvidence `json:"evidence"`
	ResearchReason     ResearchReason  `json:"research_reason"`
	RiskNote           *RiskNote       `json:"risk_note"`
	StrategyIntentRef  *string         `json:"strategy_intent_ref"` // always null in MVP
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ExplainPatch is PUT/PATCH body for manual explain edits (no AI).
// Nil pointer = leave unchanged.
type ExplainPatch struct {
	Summary        *string         `json:"summary"`
	ResearchReason *ResearchReason `json:"research_reason"`
	RiskNote       *RiskNote       `json:"risk_note"`
	ClearRiskNote  bool            `json:"clear_risk_note,omitempty"`
}
