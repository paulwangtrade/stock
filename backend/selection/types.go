// Package selection is the Candidate Selection Layer (Phase12-E.1 / E.2).
//
// Pure functions only: no DB, API, TradePlan, PlanFilter, Sizer, Execution, or Broker.
// Not wired into BuildCandidatePool / BuildDraftTradePlanFromCandidatePool.
package selection

// DefaultMaxSelectedNames matches strategy defaultMaxPlanNames (5). Used when MaxSelectedNames <= 0.
const DefaultMaxSelectedNames = 5

const (
	ReasonRankTop          = "rank_top"
	ReasonOverNameLimit    = "over_name_limit"
	ReasonInvalidCandidate = "invalid_candidate"
	ReasonAlreadyHolding   = "already_holding"
	ReasonDuplicateSymbol  = "duplicate_symbol"
	ReasonCashLimit        = "cash_limit"
	// Reserved for later phases (not applied in E.2).
	ReasonSectorLimit = "sector_limit"
	ReasonRiskLimit   = "risk_limit"
)

// Candidate is a pool row the selector can see. It is not a TradePlan item.
type Candidate struct {
	StockCode string
	StockName string
	Industry  string
	Rank      int
	Score     float64
	Reason    string
}

// ExistingPosition is an injected holding. Selector does not load Snapshot/API.
type ExistingPosition struct {
	StockCode string
}

// PortfolioSnapshot is reserved for sector/risk context (E.2 does not read it).
type PortfolioSnapshot struct {
	Cash   float64
	Equity float64
}

// SelectionContext drives Select. Injected facts only — no DB or Portfolio API.
type SelectionContext struct {
	MaxSelectedNames       int
	ExistingPositions      []ExistingPosition
	PortfolioSnapshot      *PortfolioSnapshot // reserved (sector_limit / risk_limit)
	AvailableCash          float64
	EstimatedAmountPerName float64 // 0 → cash prefilter off
}

// CandidateDecision is one selection ruling. Not a QuantDecision and not executable.
type CandidateDecision struct {
	Candidate       Candidate
	Selected        bool
	Rank            int
	SelectionReason string
	SkippedReason   string `json:"skip_reason,omitempty"`
	SkipReason      string `json:"-"` // alias of SkippedReason
	SelectionRank   int    // 1..K when selected; 0 when skipped
}

// SelectedCandidates is the adapter output for a future TradePlan consumer.
type SelectedCandidates struct {
	Selected []CandidateDecision
	Skipped  []CandidateDecision
}
