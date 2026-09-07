// Package research — Phase13 Research Candidate Pool (MVP-1 read + MVP-2 annotation store).
// Naming isolation: ResearchCandidate ≠ models.CandidatePool (Trade Candidate).
// No formal DB table; no Promote / TradePlan / Broker in this package for MVP-2.
package research

import "time"

// Research status values (A6 MVP-2). Isolated from TradePlan / Trade Candidate statuses.
const (
	StatusNew       = "new"
	StatusWatching  = "watching"
	StatusReviewed  = "reviewed"
	StatusDiscarded = "discarded"
)

// SourceSignalSnapshot is the primary list source.
const SourceSignalSnapshot = "signal_snapshot"

// Candidate is the Research Candidate Pool boundary DTO (A2/A3 + A6 annotation fields).
// Explain full body lives on ResearchExplain; Candidate only carries thin refs (A7/A8).
type Candidate struct {
	ID               string     `json:"id"`
	TradeDate        string     `json:"trade_date"`
	StockCode        string     `json:"stock_code"`
	StockName        string     `json:"stock_name"`
	Status           string     `json:"status"`
	Source           string     `json:"source"`
	SourceRef        string     `json:"source_ref,omitempty"`
	SignalTag        string     `json:"signal_tag,omitempty"`
	SignalScore      *float64   `json:"signal_score,omitempty"`
	SignalSnapshotID *uint      `json:"signal_snapshot_id,omitempty"`
	Score            *float64   `json:"score,omitempty"`
	Rank             *int       `json:"rank,omitempty"`
	Tags             []string   `json:"tags,omitempty"`
	Note             string     `json:"note,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	PromotedPoolID   *uint      `json:"promoted_pool_id,omitempty"`
	PromotedAt       *time.Time `json:"promoted_at,omitempty"`
	// Thin Explain mount (A8) — no evidence/risk_note inflation.
	ExplainRef     *string `json:"explain_ref,omitempty"`
	ExplainSummary string  `json:"explain_summary,omitempty"`
	// Display-only helpers (not required by A3 contract; omitempty for UI shell).
	Direction string `json:"direction,omitempty"`
	Price     string `json:"price,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// ListResult is GET /api/research/candidates response body.
type ListResult struct {
	TradeDate  string      `json:"trade_date"`
	Items      []Candidate `json:"items"`
	AsOf       string      `json:"as_of"`
	Message    string      `json:"message,omitempty"`
	Threshold  float64     `json:"threshold,omitempty"`
	SnapshotID uint        `json:"snapshot_id,omitempty"`
}

// ExplanationShell is the thin Detail projection of ResearchExplain.
type ExplanationShell struct {
	Available     bool   `json:"available"`
	Summary       string `json:"summary"`
	MissingReason string `json:"missing_reason,omitempty"`
	ExplainRef    string `json:"explain_ref,omitempty"`
}

// LinksShell holds optional cross-boundary refs (null until Promote MVP).
type LinksShell struct {
	TradePoolID *uint `json:"trade_pool_id"`
	TradePlanID *uint `json:"trade_plan_id"`
}

// DetailResult is GET /api/research/candidates/{id} response body.
type DetailResult struct {
	Candidate   Candidate        `json:"candidate"`
	Explanation ExplanationShell `json:"explanation"`
	Links       LinksShell       `json:"links"`
}

// ListQuery filters for List.
type ListQuery struct {
	TradeDate string
	Status    string // comma-separated optional
	Source    string
	MinScore  float64 // <=0 → config default
}

// UpdatePatch is PATCH body for research annotation (status / note / tags).
// Nil pointer = field not provided (leave unchanged).
type UpdatePatch struct {
	Status *string  `json:"status"`
	Note   *string  `json:"note"`
	Tags   *[]string `json:"tags"`
}
