package providershadow

import (
	"time"

	"go-stock/backend/decisionprovider"
)

const (
	SchemaVersionG61     = "shadow_comparison.g6-1"
	ProviderVersionStamp = "legacy@g3-1+portfolio@g4-1"
	ConstraintVersionF21 = "constraint@f2-1"
	ComparatorVersionG51 = "comparator@g5-1"
	SelectionVersionE61  = "selection@e6-1"

	TriggerAfterCloseDraft = "after_close_draft"

	PresenceAbsent = "absent"
	PresenceZero   = "zero"
	PresenceActual = "actual"

	RoleSelected = "selected"
	RoleWaitlist = "waitlist"
	RoleRejected = "rejected"
	RoleAbsent   = "absent"

	KindBothActual     = "both_actual"
	KindZeroVsActual   = "zero_vs_actual"
	KindBothZero       = "both_zero"
	KindAbsentVsZero   = "absent_vs_zero"
	KindAbsentVsActual = "absent_vs_actual"

	IncomparablePortfolioFailed = "portfolio_failed"
	IncomparableLegacyFailed    = "legacy_failed"
	IncomparableBothFailed      = "both_failed"

	ErrRuntimePanic   = "runtime_panic"
	ErrRuntimeTimeout = "runtime_timeout"
)

// DefaultEnabled is the compile-time default. Production stays false.
const DefaultEnabled = false

// ComparisonSummary is the G.12 persist-facing count block.
type ComparisonSummary struct {
	OnlyLegacyCount    int `json:"only_legacy_count"`
	OnlyPortfolioCount int `json:"only_portfolio_count"`
	CommonCount        int `json:"common_count"`
	AmountDeltaCount   int `json:"amount_delta_count"`
	RoleChangeCount    int `json:"role_change_count"`
}

// ShadowFailure records a closed Portfolio (or runtime) failure.
type ShadowFailure struct {
	PortfolioFailed bool   `json:"portfolio_failed"`
	ErrorReason     string `json:"error_reason"`
}

// ShadowComparisonRecord is an independent observation row. It is not a TradePlan.
type ShadowComparisonRecord struct {
	SchemaVersion            string                    `json:"schema_version"`
	RunID                    string                    `json:"run_id"`
	RecordedAt               time.Time                 `json:"recorded_at"`
	DecisionTime             time.Time                 `json:"decision_time"`
	TradeDate                string                    `json:"trade_date,omitempty"`
	ContextFingerprint       string                    `json:"context_fingerprint"`
	Fingerprint              string                    `json:"fingerprint"`
	ProviderVersion          string                    `json:"provider_version"`
	LegacyProviderVersion    string                    `json:"legacy_provider_version"`
	PortfolioProviderVersion string                    `json:"portfolio_provider_version"`
	ConstraintVersion        string                    `json:"constraint_version"`
	AllocationVersion        string                    `json:"allocation_version"`
	SelectionVersion         string                    `json:"selection_version"`
	ComparatorVersion        string                    `json:"comparator_version"`
	ContractVersion          string                    `json:"contract_version"`
	Comparable               bool                      `json:"comparable"`
	IncomparableReason       string                    `json:"incomparable_reason,omitempty"`
	ComparisonSummary        ComparisonSummary         `json:"comparison_summary"`
	LegacySummary            EnvelopeRef               `json:"legacy_summary"`
	PortfolioSummary         EnvelopeRef               `json:"portfolio_summary"`
	Failure                  ShadowFailure             `json:"failure"`
	FailureSummary           ShadowFailure             `json:"failure_summary"`

	// H3.1 Allocation Shadow record fields (observation only).
	RiskConstraintTrace     *RiskAdjustmentTrace      `json:"risk_constraint_trace,omitempty"`
	AllocationBudgetSummary *AllocationBudgetSummary  `json:"allocation_budget_summary,omitempty"`
	ReserveSummary          *ReserveSummary           `json:"reserve_summary,omitempty"`
	RiskCutSummary          *RiskCutSummary           `json:"risk_cut_summary,omitempty"`
	// RiskAdjustment mirrors RiskConstraintTrace for earlier callers.
	RiskAdjustment *RiskAdjustmentTrace `json:"risk_adjustment,omitempty"`

	Report *ProviderComparisonReport `json:"report,omitempty"`
}

// EnvelopeRef is a persist-safe envelope summary. Not a TradePlan header.
type EnvelopeRef struct {
	OK        bool   `json:"ok"`
	Provider  string `json:"provider"`
	LineCount int    `json:"line_count"`
	ErrorCode string `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

// SymbolDiff is L\P, P\L, L∩P. Empty slices are written, not omitted.
type SymbolDiff struct {
	OnlyLegacy    []string `json:"only_legacy"`
	OnlyPortfolio []string `json:"only_portfolio"`
	Common        []string `json:"common"`
}

// AmountView is a three-state amount. Absent must not be filled with 0.
type AmountView struct {
	Presence string   `json:"presence"`
	Value    *float64 `json:"value,omitempty"`
}

// AmountDiff compares two AmountViews. Delta is set only for zero|actual vs zero|actual.
type AmountDiff struct {
	Symbol    string     `json:"symbol"`
	Legacy    AmountView `json:"legacy"`
	Portfolio AmountView `json:"portfolio"`
	Delta     *float64   `json:"delta,omitempty"`
	Kind      string     `json:"kind"`
}

// RoleDiff records a role mismatch. pair is observational, not a recommendation.
type RoleDiff struct {
	Symbol        string `json:"symbol"`
	LegacyRole    string `json:"legacy_role"`
	PortfolioRole string `json:"portfolio_role"`
	Pair          string `json:"pair"`
}

// ReasonDiff compares decision vs research layers independently.
type ReasonDiff struct {
	Symbol        string `json:"symbol"`
	Layer         string `json:"layer"`
	LegacyText    string `json:"legacy_text"`
	PortfolioText string `json:"portfolio_text"`
}

// ComparisonMetrics are counts, not a quality score.
type ComparisonMetrics struct {
	OnlyLegacyCount         int    `json:"only_legacy_count"`
	OnlyPortfolioCount      int    `json:"only_portfolio_count"`
	CommonCount             int    `json:"common_count"`
	AmountDiffCount         int    `json:"amount_diff_count"`
	AmountEqualCount        int    `json:"amount_equal_count"`
	RoleDiffCount           int    `json:"role_diff_count"`
	ReasonDecisionDiffCount int    `json:"reason_decision_diff_count"`
	ReasonResearchDiffCount int    `json:"reason_research_diff_count"`
	SkippedBecause          string `json:"skipped_because,omitempty"`
}

// ProviderComparisonReport is the G.5 + H3.1 sidecar report.
// Candidate diff → Symbols; amount diff → Amounts; allocation diff → Allocation;
// risk adjustment diff → RiskAdjustment. No winner / provider switch.
type ProviderComparisonReport struct {
	RecordOnly         bool                 `json:"record_only"`
	NotATradePlan      bool                 `json:"not_a_trade_plan"`
	NotAProviderSwitch bool                 `json:"not_a_provider_switch"`
	DecisionTime       time.Time            `json:"decision_time"`
	ChainProvider      string               `json:"chain_provider"`
	Legacy             EnvelopeRef          `json:"legacy"`
	Portfolio          EnvelopeRef          `json:"portfolio"`
	Comparable         bool                 `json:"comparable"`
	IncomparableReason string               `json:"incomparable_reason,omitempty"`
	Symbols            SymbolDiff           `json:"symbols"`             // candidate diff
	Amounts            []AmountDiff         `json:"amounts"`             // amount diff
	Roles              []RoleDiff           `json:"roles"`
	Reasons            []ReasonDiff         `json:"reasons"`
	Metrics            ComparisonMetrics    `json:"metrics"`
	Allocation         *AllocationCompare   `json:"allocation,omitempty"` // allocation diff
	RiskAdjustment     *RiskAdjustmentTrace `json:"risk_adjustment,omitempty"`
}

func envelopeRef(env *decisionprovider.DecisionEnvelope) EnvelopeRef {
	if env == nil {
		return EnvelopeRef{}
	}
	ref := EnvelopeRef{
		OK:        env.OK,
		Provider:  env.Provider,
		LineCount: len(env.Lines),
	}
	if env.Error != nil {
		ref.ErrorCode = env.Error.Code
		ref.ErrorMsg = env.Error.Message
	}
	return ref
}

func emptySymbolDiff() SymbolDiff {
	return SymbolDiff{
		OnlyLegacy:    []string{},
		OnlyPortfolio: []string{},
		Common:        []string{},
	}
}
