// Package strategyexplain provides a read-only Strategy Explanation layer.
//
// Phase13-A MVP: StrategySnapshot → Explanation (Signal / Risk / Entry / Exit).
// Does not modify TradePlan, Strategy, or Execution; must not call Broker / Gateway / Fill.
package strategyexplain

import (
	"time"

	"go-stock/backend/featuregate"
)

// SchemaVersion is the Explanation contract version (Phase12-C).
const SchemaVersion = "sexpl-1"

// Status values for Explanation.
const (
	StatusOK       = "ok"
	StatusGated    = "gated"
	StatusDegraded = "degraded"
	StatusMissing  = "missing"
	StatusFailed   = "failed"
)

// ExitMode for ExitReason.
const (
	ExitModeReview      = "review"
	ExitModeNone        = "none"
	ExitModeUnavailable = "unavailable"
)

// ExplainRequest loads a snapshot and renders a deterministic Explanation.
type ExplainRequest struct {
	User            *featuregate.User `json:"-"`
	UserID          string            `json:"user_id,omitempty"`
	Tier            string            `json:"tier,omitempty"` // shell convenience when User unset
	SnapshotID      string            `json:"snapshot_id,omitempty"`
	PlanID          uint              `json:"plan_id,omitempty"`
	PlanItemID      uint              `json:"plan_item_id,omitempty"`
	Locale          string            `json:"locale,omitempty"`
	IncludeExit     bool              `json:"include_exit,omitempty"`
	ExitOverlay     *ExitOverlay      `json:"exit_overlay,omitempty"`
	EntryReasonHint string            `json:"entry_reason_hint,omitempty"`
	EntryRuleHint   string            `json:"entry_rule_hint,omitempty"`
	Now             time.Time         `json:"-"`
}

// ExitOverlay is a read-only holding re-assessment label (not a sell order).
type ExitOverlay struct {
	HoldingDays      int      `json:"holding_days,omitempty"`
	UnrealizedReturn *float64 `json:"unrealized_return,omitempty"`
	ExitState        string   `json:"exit_state,omitempty"`
	ExitReasonCodes  []string `json:"exit_reason_codes,omitempty"`
}

// Explanation is the rendered strategy thesis (why buy / why review exit).
type Explanation struct {
	ExplanationID string    `json:"explanation_id"`
	SchemaVersion string    `json:"schema_version"`
	SnapshotID    string    `json:"snapshot_id,omitempty"`
	PlanID        uint      `json:"plan_id,omitempty"`
	PlanItemID    uint      `json:"plan_item_id,omitempty"`
	TradeDate     string    `json:"trade_date,omitempty"`
	GeneratedAt   time.Time `json:"generated_at"`
	Status        string    `json:"status"`
	GateReason    string    `json:"gate_reason,omitempty"`

	Headline    string               `json:"headline"`
	Sections    ExplanationSections  `json:"sections"`
	Citations   []Citation           `json:"citations,omitempty"`
	Disclaimers []string             `json:"disclaimers"`
}

// ExplanationSections holds the four narrative blocks.
type ExplanationSections struct {
	Signal SignalExplanation `json:"signal"`
	Risk   RiskExplanation   `json:"risk"`
	Entry  EntryReason       `json:"entry"`
	Exit   *ExitReason       `json:"exit,omitempty"`
}

// SignalExplanation narrates Snapshot.SignalResult (no rescore).
type SignalExplanation struct {
	Available     bool    `json:"available"`
	Tag           string  `json:"tag,omitempty"`
	Score         float64 `json:"score,omitempty"`
	SnapshotRef   uint    `json:"snapshot_ref,omitempty"`
	Narrative     string  `json:"narrative,omitempty"`
	MissingReason string  `json:"missing_reason,omitempty"`
}

// RiskExplanation narrates Snapshot.RiskDecision.
type RiskExplanation struct {
	Available     bool   `json:"available"`
	Accepted      *bool  `json:"accepted,omitempty"`
	RiskCode      string `json:"risk_code,omitempty"`
	RiskMessage   string `json:"risk_message,omitempty"`
	PlanStatus    string `json:"plan_status,omitempty"`
	Narrative     string `json:"narrative,omitempty"`
	MissingReason string `json:"missing_reason,omitempty"`
}

// EntryReason answers "why buy" from Snapshot facts.
type EntryReason struct {
	StrategyName     string  `json:"strategy_name,omitempty"`
	StrategyVersion  string  `json:"strategy_version,omitempty"`
	EntryReasonRaw   string  `json:"entry_reason_raw,omitempty"`
	EntryRule        string  `json:"entry_rule,omitempty"`
	RefPrice         float64 `json:"ref_price,omitempty"`
	RefSource        string  `json:"ref_source,omitempty"`
	Narrative        string  `json:"narrative,omitempty"`
	ThesisIntactNote string  `json:"thesis_intact_note,omitempty"`
}

// ExitReason answers "why review exit thesis" — never a sell instruction.
type ExitReason struct {
	Mode            string   `json:"mode"`
	ExitState       string   `json:"exit_state,omitempty"`
	ReasonCodes     []string `json:"reason_codes,omitempty"`
	Narrative       string   `json:"narrative,omitempty"`
	ComparedToEntry string   `json:"compared_to_entry,omitempty"`
}

// Citation points at a fact source.
type Citation struct {
	Kind  string `json:"kind"` // snapshot | exit_code | parameter
	Ref   string `json:"ref"`
	Label string `json:"label,omitempty"`
}
