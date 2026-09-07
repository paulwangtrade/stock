package decisionprovider

import (
	"fmt"
	"time"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"
)

const (
	ProviderFixedAmount = "fixed_amount"
	ContractG21         = "decision_provider.g2-1"

	AllocReasonFixedAmount = "fixed_amount"

	ProviderPortfolioAllocation = "portfolio_allocation"
	AllocVersionF1Equal         = "allocation@f1-v1-equal"

	ErrCodeContextInvalid    = "ERR_CONTEXT_INVALID"
	ErrCodeMissingSymbol     = "ERR_MISSING_SYMBOL"
	ErrCodeZeroAmount        = "ERR_ZERO_AMOUNT"
	ErrCodeInvalidAllocation = "ERR_INVALID_ALLOCATION"
)

// DecisionVersion is the G.2 algorithm identity stamp.
type DecisionVersion struct {
	Contract   string
	Selection  string
	Constraint string
	Allocation string
	Provider   string
}

// DecisionContext is the G.2 Decide input. Legacy only reads Selection (and
// UniformAmount when the FilterPool compatibility path injects a scalar).
type DecisionContext struct {
	Selection    *selection.CandidateSelectionResult
	DecisionTime time.Time
	Version      DecisionVersion
	// UniformAmount, when >0, is the FilterPool/Builder scalar and skips Sizer.
	// Zero means LegacyDecisionProvider calls FixedAmountSizer.Propose.
	UniformAmount float64

	Snapshot    *portfoliolayer.PortfolioSnapshot
	Constraints portfoliolayer.ConstraintSet
	// Budget nil → Portfolio ComputeBudget; non-nil → AllocateWithBudget (all-zero object ≠ nil).
	Budget *portfoliolayer.AllocationBudget
}

// RejectedLine is excluded from Filter scan (G.2/G.4). Not a DecisionLine.
type RejectedLine struct {
	Symbol string
	Reason string
	Rank   int
}

// DecisionError is a closed-fail Decide outcome (G.2).
type DecisionError struct {
	Code    string
	Message string
	Symbol  string
}

func (e *DecisionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Symbol != "" {
		return fmt.Sprintf("%s: %s symbol=%s", e.Code, e.Message, e.Symbol)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// LineMeta is envelope metadata. Not a second amount source.
type LineMeta struct {
	Rank             int
	Score            float64
	StockName        string
	Industry         string
	InAllocationSet  bool
	AllocationReason string
	SourceProvider   string
	CandidateReason  string
}

// DecisionLine is the G.2 row contract.
type DecisionLine struct {
	Symbol       string
	TargetAmount float64
	Reason       string
	Metadata     LineMeta
}

// EnvelopeMeta is batch identity, not a TradePlan header replacement.
type EnvelopeMeta struct {
	UniformAmount float64
	SnapshotFound bool
	Budget        portfoliolayer.AllocationBudget
	Notes         []string
}

// DecisionEnvelope is the G.2 Decide output. It is not a TradePlan.
type DecisionEnvelope struct {
	OK             bool
	Error          *DecisionError
	Provider       string
	DecisionTime   time.Time
	Version        DecisionVersion
	SelectionLimit int
	Lines          []DecisionLine
	Rejected       []RejectedLine
	Metadata       EnvelopeMeta
}

// DecisionProvider produces a DecisionEnvelope. G.10 Legacy; G.11 Portfolio.
// G.12 Shadow may call Portfolio as bypass. G.8 Controlled may call Portfolio on
// write-chain Draft when ControlledProviderPolicy whitelist matches (default OFF).
type DecisionProvider interface {
	Name() string
	Decide(DecisionContext) (*DecisionEnvelope, error)
}
