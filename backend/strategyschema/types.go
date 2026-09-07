// Package strategyschema — Phase13 Strategy Schema MVP (sschema-1).
// B3-A read + B3-B manual lifecycle write. No AI, Promote, TradePlan, Broker, Execution, or formal DB.
package strategyschema

import "time"

// SchemaVersion is the Strategy Schema contract version.
const SchemaVersion = "sschema-1"

// Definition status (shell, not TradePlan / Intent).
const (
	DefinitionStatusActive    = "active"
	DefinitionStatusArchived  = "archived"
	DefinitionStatusDraftOnly = "draft_only"
)

// Revision lifecycle (B3-B enforces transitions on write).
const (
	RevisionStatusDraft     = "draft"
	RevisionStatusReviewing = "reviewing"
	RevisionStatusActive    = "active"
	RevisionStatusRetired   = "retired"
	RevisionStatusDiscarded = "discarded"
)

// Definition is the stable strategy identity shell.
type Definition struct {
	StrategyID       string    `json:"strategy_id"`
	SchemaVersion    string    `json:"schema_version"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	Status           string    `json:"status"`
	CurrentRevision  string    `json:"current_revision,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UniverseSpec declares the selection universe (not live market data).
type UniverseSpec struct {
	Source     string   `json:"source,omitempty"`
	AdapterRef string   `json:"adapter_ref,omitempty"`
	Include    []string `json:"include,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
	ExcludeST  bool     `json:"exclude_st,omitempty"`
	MaxSize    int      `json:"max_size,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// SignalsSpec declares signal rules (deterministic preferred).
type SignalsSpec struct {
	Kind        string `json:"kind,omitempty"` // deterministic | adapter | unbound
	Notes       string `json:"notes,omitempty"`
	CompiledRef string `json:"compiled_ref,omitempty"`
	// Rules kept as raw-friendly map slice for MVP seed JSON.
	Rules []map[string]any `json:"rules,omitempty"`
}

// FiltersSpec declares hard filters.
type FiltersSpec struct {
	Notes string           `json:"notes,omitempty"`
	Rules []map[string]any `json:"rules,omitempty"`
}

// RankingSpec declares sort / top_n.
type RankingSpec struct {
	SortBy []string `json:"sort_by,omitempty"`
	TopN   int      `json:"top_n,omitempty"`
	Dedupe bool     `json:"dedupe,omitempty"`
	Notes  string   `json:"notes,omitempty"`
}

// RiskProfileRef is inherit | named — never embeds account cash.
type RiskProfileRef struct {
	Mode    string `json:"mode"` // inherit | named
	NamedID string `json:"named_id,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

// ParametersSpec holds knobs + stable hash for future Promote pin.
type ParametersSpec struct {
	Knobs      map[string]any `json:"knobs,omitempty"`
	ParamsHash string         `json:"params_hash,omitempty"`
}

// Revision is one versioned rule document.
type Revision struct {
	RevisionID     string         `json:"revision_id"`
	StrategyID     string         `json:"strategy_id"`
	Revision       string         `json:"revision"`
	SchemaVersion  string         `json:"schema_version"`
	Status         string         `json:"status"`
	NameOverride   string         `json:"name_override,omitempty"`
	RevisionNote   string         `json:"revision_note,omitempty"`
	Universe       UniverseSpec   `json:"universe"`
	Signals        SignalsSpec    `json:"signals"`
	Filters        FiltersSpec    `json:"filters"`
	Ranking        RankingSpec    `json:"ranking"`
	RiskProfileRef RiskProfileRef `json:"risk_profile_ref"`
	Parameters     ParametersSpec `json:"parameters"`
	RevisionHash   string         `json:"revision_hash,omitempty"` // full content fingerprint
	Source         string         `json:"source,omitempty"`        // manual | imported_adapter | ai_draft
	ParentRevision string         `json:"parent_revision,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	ActivatedAt    *time.Time     `json:"activated_at,omitempty"`
	RetiredAt      *time.Time     `json:"retired_at,omitempty"`
}

// RevisionSummary is a thin list row (no full content).
type RevisionSummary struct {
	RevisionID   string    `json:"revision_id"`
	Revision     string    `json:"revision"`
	Status       string    `json:"status"`
	RevisionNote string    `json:"revision_note,omitempty"`
	ParamsHash   string    `json:"params_hash,omitempty"`
	RevisionHash string    `json:"revision_hash,omitempty"`
	Source       string    `json:"source,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateDraftRequest creates a draft revision (optionally a new Definition).
type CreateDraftRequest struct {
	StrategyID     string         `json:"strategy_id,omitempty"`
	Slug           string         `json:"slug,omitempty"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	ParentRevision string         `json:"parent_revision,omitempty"`
	RevisionNote   string         `json:"revision_note,omitempty"`
	NameOverride   string         `json:"name_override,omitempty"`
	Universe       UniverseSpec   `json:"universe"`
	Signals        SignalsSpec    `json:"signals"`
	Filters        FiltersSpec    `json:"filters"`
	Ranking        RankingSpec    `json:"ranking"`
	RiskProfileRef RiskProfileRef `json:"risk_profile_ref"`
	Knobs          map[string]any `json:"knobs,omitempty"`
}

// UpdateDraftRequest patches draft revision content only.
type UpdateDraftRequest struct {
	RevisionNote   *string         `json:"revision_note,omitempty"`
	NameOverride   *string         `json:"name_override,omitempty"`
	Universe       *UniverseSpec   `json:"universe,omitempty"`
	Signals        *SignalsSpec    `json:"signals,omitempty"`
	Filters        *FiltersSpec    `json:"filters,omitempty"`
	Ranking        *RankingSpec    `json:"ranking,omitempty"`
	RiskProfileRef *RiskProfileRef `json:"risk_profile_ref,omitempty"`
	Knobs          map[string]any  `json:"knobs,omitempty"`
	ClearKnobs     bool            `json:"clear_knobs,omitempty"`
}

// WriteResult is returned by lifecycle write APIs.
type WriteResult struct {
	Definition Definition `json:"definition"`
	Revision   Revision   `json:"revision"`
	Message    string     `json:"message,omitempty"`
}

// ListItem is one row in GET /api/strategy/schemas.
type ListItem struct {
	StrategyID      string `json:"strategy_id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Status          string `json:"status"`
	CurrentRevision string `json:"current_revision,omitempty"`
	UpdatedAt       string `json:"updated_at"`
}

// ListResult is the schema list response.
type ListResult struct {
	Items   []ListItem `json:"items"`
	Message string     `json:"message,omitempty"`
}

// DetailResult is GET /api/strategy/schemas/{id}.
type DetailResult struct {
	Definition Definition        `json:"definition"`
	Revisions  []RevisionSummary `json:"revisions"`
	Current    *Revision         `json:"current,omitempty"`
}

// RevisionListResult is GET .../revisions.
type RevisionListResult struct {
	StrategyID string            `json:"strategy_id"`
	Items      []RevisionSummary `json:"items"`
}
