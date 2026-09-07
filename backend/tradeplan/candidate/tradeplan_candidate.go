package candidate

import "time"

// IntentKind explanation categories projected from Action.Code (NOT trade commands).
const (
	IntentEnterHint       = "ENTER_HINT"
	IntentWaitHint        = "WAIT_HINT"
	IntentScaleInHint     = "SCALE_IN_HINT"
	IntentReduceHint      = "REDUCE_HINT"
	IntentExitPartialHint = "EXIT_PARTIAL_HINT"
	IntentWatchHint       = "WATCH_HINT"
	IntentBlockedHint     = "BLOCKED_HINT"
	IntentOtherHint       = "OTHER_HINT"
)

// Validation / error codes (frozen).
const (
	CodeDraftNotValidated         = "draft_not_validated"
	CodeCandidateExecuteForbidden = "candidate_execute_forbidden"
	CodeCandidateMutationDetected = "candidate_mutation_detected"
	CodeSourceInvalid             = "source_invalid"
	CodeAuthorityDenied           = "authority_denied"
)

// CandidateValidation last validation outcome.
type CandidateValidation struct {
	OK      bool     `json:"ok"`
	Codes   []string `json:"codes,omitempty"`
	Message string   `json:"message,omitempty"`
}

// TradePlanCandidate shadow projection — not a TradePlan, not an Order.
type TradePlanCandidate struct {
	CandidateID        string              `json:"candidateId"`
	SourceDraftID      string              `json:"sourceDraftId,omitempty"`
	SourceDecisionID   string              `json:"sourceDecisionId"`
	SourceSnapshotHash string              `json:"sourceSnapshotHash"`
	Side               string              `json:"side"`
	TargetShares       int64               `json:"targetShares"`
	EntryPriceHint     float64             `json:"entryPriceHint"`
	StopPriceHint      float64             `json:"stopPriceHint"`
	IntentKind         string              `json:"intentKind"`
	Executable         bool                `json:"executable"` // always false
	Validation         CandidateValidation `json:"validation"`
	CreatedAt          time.Time           `json:"createdAt"`
	CandidateHash      string              `json:"candidateHash"`
	StockCode          string              `json:"stockCode,omitempty"`
	TradeDate          string              `json:"tradeDate,omitempty"`
	AdapterVersion     string              `json:"adapterVersion"`
}
