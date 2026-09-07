// Package selection is the Candidate Selection Layer (Phase12-E.1 / E.6).
//
// Pure functions only: no DB, API, TradePlan, PlanFilter, Sizer, Execution, or Broker.
// Not wired into BuildCandidatePool / BuildDraftTradePlanFromCandidatePool.
package selection

// DefaultMaxSelectedNames matches strategy defaultMaxPlanNames (5). Used when MaxSelectedNames <= 0.
const DefaultMaxSelectedNames = 5

// E.6 policy freeze: ranking + dedup only. Do not flip these in this package
// without a dedicated slice — wiring PlanFilter must stay compatible (E.4/E.5).
const (
	SkipAlreadyHolding = false
	ApplyCashLimit     = false
	ApplyRiskLimit     = false
	DeduplicateSymbols = true
)

const (
	ReasonRankTop          = "rank_top"
	ReasonOverNameLimit    = "over_name_limit"
	ReasonInvalidCandidate = "invalid_candidate"
	ReasonAlreadyHolding   = "already_holding" // reserved; E.6 does not emit
	ReasonDuplicateSymbol  = "duplicate_symbol"
	ReasonCashLimit        = "cash_limit" // reserved; E.6 does not emit
	// Reserved for later phases (not applied).
	ReasonSectorLimit = "sector_limit"
	ReasonRiskLimit   = "risk_limit"
)

// Candidate is a pool row the selector can see. It is not a TradePlan item.
type Candidate struct {
	StockCode string  `json:"stock_code"`
	StockName string  `json:"stock_name"`
	Industry  string  `json:"industry"`
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
}

// ExistingPosition is an injected holding. Selector does not load Snapshot/API.
// E.6 ignores this field during Select.
type ExistingPosition struct {
	StockCode string
}

// PortfolioSnapshot is reserved for sector/risk context (E.6 does not read it).
type PortfolioSnapshot struct {
	Cash   float64
	Equity float64
}

// SelectionContext drives Select. Injected facts only — no DB or Portfolio API.
// E.6 only honors MaxSelectedNames. Holdings / cash / snapshot are ignored.
type SelectionContext struct {
	MaxSelectedNames       int
	ExistingPositions      []ExistingPosition
	PortfolioSnapshot      *PortfolioSnapshot // reserved (sector_limit / risk_limit)
	AvailableCash          float64
	EstimatedAmountPerName float64 // reserved; E.6 does not apply cash_limit
}

// CandidateDecision is one selection ruling. Not a QuantDecision and not executable.
// Selected=true means in-basket (selection_limit), not PlanFilter/trade eligibility.
type CandidateDecision struct {
	Candidate       Candidate `json:"candidate"`
	Rank            int       `json:"rank"`
	Selected        bool      `json:"selected"`
	SelectionReason string    `json:"selection_reason"`
	SkippedReason   string    `json:"skip_reason,omitempty"`
	SkipReason      string    `json:"-"`                        // alias of SkippedReason
	SelectionRank   int       `json:"selection_rank,omitempty"` // 1..K when selected; 0 otherwise
}

// CandidateSelectionResult is the E.6 authority output.
// PlanFilter (future) must scan RankedCandidates, not PrimaryPicks.
type CandidateSelectionResult struct {
	RankedCandidates   []Candidate         `json:"ranked_candidates"`
	SelectionLimit     int                 `json:"selection_limit"`
	CandidateDecisions []CandidateDecision `json:"candidate_decisions"`
}

// SelectedCandidates is the E.1/E.2 adapter view (not the Filter input).
// Selected ≈ in-basket picks; Skipped ≈ invalid + duplicate + waitlist.
type SelectedCandidates struct {
	Selected []CandidateDecision
	Skipped  []CandidateDecision
}

// RankedForPlanFilter is the scan sequence a future PlanFilter should consume.
func (r *CandidateSelectionResult) RankedForPlanFilter() []Candidate {
	if r == nil {
		return nil
	}
	return r.RankedCandidates
}

// PrimaryPicks returns in-basket decisions (selected=true). Not trade-qualified.
func (r *CandidateSelectionResult) PrimaryPicks() []CandidateDecision {
	if r == nil {
		return nil
	}
	out := make([]CandidateDecision, 0, r.SelectionLimit)
	for _, d := range r.CandidateDecisions {
		if d.Selected {
			out = append(out, d)
		}
	}
	return out
}

// Waitlist returns ranked candidates outside the basket (over_name_limit).
func (r *CandidateSelectionResult) Waitlist() []Candidate {
	if r == nil {
		return nil
	}
	if r.SelectionLimit >= len(r.RankedCandidates) {
		return []Candidate{}
	}
	return r.RankedCandidates[r.SelectionLimit:]
}

// ToSelectedCandidates maps the E.6 result onto the E.1/E.2 two-list adapter.
// Waitlist stays in RankedCandidates; it appears under Skipped here only.
func (r *CandidateSelectionResult) ToSelectedCandidates() *SelectedCandidates {
	out := &SelectedCandidates{
		Selected: []CandidateDecision{},
		Skipped:  []CandidateDecision{},
	}
	if r == nil {
		return out
	}
	for _, d := range r.CandidateDecisions {
		if d.Selected {
			out.Selected = append(out.Selected, d)
			continue
		}
		out.Skipped = append(out.Skipped, d)
	}
	return out
}
