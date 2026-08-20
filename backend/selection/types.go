// Package selection is the Candidate Selection Layer (Phase12-E.1).
//
// Pure functions only: no DB, API, TradePlan, PlanFilter, Sizer, Execution, or Broker.
// Not wired into BuildCandidatePool / BuildDraftTradePlanFromCandidatePool.
package selection

// DefaultMaxSelectedNames matches strategy defaultMaxPlanNames (5). Used when MaxSelectedNames <= 0.
const DefaultMaxSelectedNames = 5

const (
	ReasonRankTop        = "rank_top"
	ReasonOverNameLimit  = "over_name_limit"
	ReasonInvalidCandidate = "invalid_candidate"
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

// ExistingPosition is reserved for skip_held (not applied in E.1).
type ExistingPosition struct {
	StockCode string
}

// PortfolioSnapshot is reserved for capital/exposure context (not read in E.1).
type PortfolioSnapshot struct {
	Cash   float64
	Equity float64
}

// SelectionContext drives Select. E.1 only honours MaxSelectedNames.
type SelectionContext struct {
	MaxSelectedNames  int
	ExistingPositions []ExistingPosition // reserved
	PortfolioSnapshot *PortfolioSnapshot // reserved
	AvailableCash     float64            // reserved
}

// CandidateDecision is one selection ruling. Not a QuantDecision and not executable.
type CandidateDecision struct {
	Candidate        Candidate
	Selected         bool
	Rank             int
	SelectionReason  string
	SkippedReason    string
	SelectionRank    int // 1..K when selected; 0 when skipped
}

// SelectedCandidates is the adapter output for a future TradePlan consumer.
type SelectedCandidates struct {
	Selected []CandidateDecision
	Skipped  []CandidateDecision
}
