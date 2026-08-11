// Package allocation computes a read-only buy budget from PortfolioSnapshot (Phase10-D.5).
//
// Pure functions only: no DB, no writes, no TradePlan / Sizer / Execution wiring.
// Planned sell proceeds are never added to available capital.
package allocation

import "go-stock/backend/portfolio"

// Reason codes for CapitalAllocationResult.Reason (not sell / not sizing method).
const (
	ReasonOK            = "OK"
	ReasonNoAccount     = "NO_ACCOUNT"
	ReasonBlocked       = "BLOCKED"
	ReasonCash          = "CASH"
	ReasonGross         = "GROSS"
	ReasonNegativeInput = "NEGATIVE_INPUT"
)

// DefaultMaxGrossExposurePct matches plan-risk MVP (0.85). Used only when policy pct is unset.
const DefaultMaxGrossExposurePct = 0.85

// AllocationPolicy is the risk/budget cap for this round (not a sizer).
// MaxSingleNamePct / MaxNames are stored for future Equal/Score/Risk sizers; v1 total formula ignores them.
type AllocationPolicy struct {
	MaxGrossExposurePct float64 `json:"max_gross_exposure_pct"`
	MaxSingleNamePct    float64 `json:"max_single_name_pct,omitempty"`
	MaxNames            int     `json:"max_names,omitempty"`
	BlockNewEntries     bool    `json:"block_new_entries,omitempty"`
}

// CapitalAllocationRequest is the pure-function input. Callers inject Snapshot; this package does not load it.
type CapitalAllocationRequest struct {
	Snapshot   *portfolio.Snapshot `json:"snapshot"`
	Policy     AllocationPolicy    `json:"policy"`
	PendingBuy float64             `json:"pending_buy,omitempty"` // submitted buy notional; default 0
}

// CapitalAllocationResult is the round budget. Not per-name sizes.
type CapitalAllocationResult struct {
	AvailableCapital float64 `json:"available_capital"`
	CashAvailable    float64 `json:"cash_available"`
	ExposureLimit    float64 `json:"exposure_limit"`
	GrossHeadroom    float64 `json:"gross_headroom"`
	Reason           string  `json:"reason"`
	Binding          string  `json:"binding,omitempty"`
	PolicyGrossPct   float64 `json:"policy_gross_pct"`
}

// Allocator is the future extension point (Equal/Score/Risk consume Result, not this package).
type Allocator interface {
	Budget(req CapitalAllocationRequest) CapitalAllocationResult
}

type pureAllocator struct{}

// NewAllocator returns the v1 pure budget calculator.
func NewAllocator() Allocator {
	return pureAllocator{}
}

func (pureAllocator) Budget(req CapitalAllocationRequest) CapitalAllocationResult {
	return Budget(req)
}
