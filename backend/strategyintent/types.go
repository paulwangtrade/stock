// Package strategyintent — Phase13 Strategy Intent domain MVP (sint-1).
// B1-A read + B1-B manual lifecycle write. No AI, Promote, TradePlan, Broker, Execution, or formal DB.
package strategyintent

import "time"

// SchemaVersion is the Strategy Intent contract version.
const SchemaVersion = "sint-1"

// Intent lifecycle (research domain; ≠ TradePlan status).
const (
	StatusDraft     = "draft"
	StatusReviewing = "reviewing"
	StatusApproved  = "approved"
	StatusExpired   = "expired"
	StatusDiscarded = "discarded"
)

// IntentType values (ai_draft reserved; B1-A does not generate AI intents).
const (
	IntentTypeManual        = "manual"
	IntentTypeRuleSuggested = "rule_suggested"
	IntentTypeAIDraft       = "ai_draft"
	IntentTypeHybrid        = "hybrid"
)

// SchemaRef pins a Strategy Schema revision (never "current").
type SchemaRef struct {
	Unbound    bool   `json:"unbound"`
	StrategyID string `json:"strategy_id,omitempty"`
	Revision   string `json:"revision,omitempty"` // explicit version label
	ParamsHash string `json:"params_hash,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Conditions are declared propositions (not live market truth).
type Conditions struct {
	TradeDate      string  `json:"trade_date,omitempty"`
	Session        string  `json:"session,omitempty"` // open | close | any
	MinSignalScore float64 `json:"min_signal_score,omitempty"`
	ValidUntil     string  `json:"valid_until,omitempty"`
	Notes          string  `json:"notes,omitempty"`
}

// SizingIntentUpper is an upper-bound hint only (not order qty).
type SizingIntentUpper struct {
	Kind  string  `json:"kind"` // notional | weight | shares_hint
	Value float64 `json:"value"`
}

// Action is an action intention (not an order).
type Action struct {
	Verb               string             `json:"verb"` // watch | consider_buy | consider_sell | avoid | defer
	SideHint           string             `json:"side_hint,omitempty"`
	EntrySession       string             `json:"entry_session,omitempty"`
	SizingIntentUpper  *SizingIntentUpper `json:"sizing_intent_upper,omitempty"`
	Text               string             `json:"text,omitempty"`
}

// RiskConstraints are research declarations (not QuantityPolicy / Gateway results).
type RiskConstraints struct {
	SeverityCap     string   `json:"severity_cap,omitempty"`
	AvoidIf         []string `json:"avoid_if,omitempty"`
	MaxNotionalHint float64  `json:"max_notional_hint,omitempty"`
	Text            string   `json:"text,omitempty"`
}

// Intent is the StrategyIntent DTO (sint-1).
type Intent struct {
	ID                 string          `json:"id"`
	SchemaVersion      string          `json:"schema_version"`
	CandidateID        string          `json:"candidate_id"`
	StrategySchemaRef  SchemaRef       `json:"strategy_schema_ref"`
	SchemaRevision     string          `json:"schema_revision,omitempty"` // pinned revision; must match ref when bound
	IntentType         string          `json:"intent_type"`
	Conditions         Conditions      `json:"conditions"`
	Action             Action          `json:"action"`
	RiskConstraints    RiskConstraints `json:"risk_constraints"`
	Status             string          `json:"status"`
	Summary            string          `json:"summary,omitempty"`
	ExplainRef         string          `json:"explain_ref,omitempty"`
	PromotedPoolID     *int64          `json:"promoted_pool_id"` // always null in B1-A
	TradePlanID        *int64          `json:"trade_plan_id"`    // always null in B1-A
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// ListItem is a thin list row.
type ListItem struct {
	ID             string `json:"id"`
	CandidateID    string `json:"candidate_id"`
	Status         string `json:"status"`
	IntentType     string `json:"intent_type"`
	Summary        string `json:"summary,omitempty"`
	SchemaRevision string `json:"schema_revision,omitempty"`
	StrategyID     string `json:"strategy_id,omitempty"`
	Unbound        bool   `json:"unbound"`
	UpdatedAt      string `json:"updated_at"`
}

// ListResult is GET /api/strategy/intents.
type ListResult struct {
	Items   []ListItem `json:"items"`
	Message string     `json:"message,omitempty"`
}

// CreateDraftRequest creates a draft intent for a candidate.
type CreateDraftRequest struct {
	CandidateID       string          `json:"candidate_id"`
	ExplainRef        string          `json:"explain_ref,omitempty"`
	StrategySchemaRef SchemaRef       `json:"strategy_schema_ref"`
	SchemaRevision    string          `json:"schema_revision,omitempty"`
	IntentType        string          `json:"intent_type,omitempty"`
	Summary           string          `json:"summary,omitempty"`
	Conditions        Conditions      `json:"conditions"`
	Action            Action          `json:"action"`
	RiskConstraints   RiskConstraints `json:"risk_constraints"`
}

// UpdateDraftRequest patches draft intent content only.
type UpdateDraftRequest struct {
	ExplainRef        *string          `json:"explain_ref,omitempty"`
	StrategySchemaRef *SchemaRef       `json:"strategy_schema_ref,omitempty"`
	SchemaRevision    *string          `json:"schema_revision,omitempty"`
	IntentType        *string          `json:"intent_type,omitempty"`
	Summary           *string          `json:"summary,omitempty"`
	Conditions        *Conditions      `json:"conditions,omitempty"`
	Action            *Action          `json:"action,omitempty"`
	RiskConstraints   *RiskConstraints `json:"risk_constraints,omitempty"`
}

// WriteResult is returned by lifecycle write APIs.
type WriteResult struct {
	Intent  Intent `json:"intent"`
	Message string `json:"message,omitempty"`
}
