package portfoliosim

import (
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"
)

const (
	FilterStatusNotRun = "not_run"
	FilterStatusCalled = "called"

	DefaultScanLimit = 30
)

// FilterCompatRequest controls the read-only PlanFilter compatibility check.
// SkipCompatibilityCheck=false (zero value) means the check runs.
type FilterCompatRequest struct {
	SkipCompatibilityCheck bool
	ScanLimit              int
	MaxNames               int
}

// Input is the F.7/F.8 controlled-simulation request. All facts are injected.
type Input struct {
	Snapshot            *portfoliolayer.PortfolioSnapshot
	Candidates          *selection.CandidateSelectionResult
	RankedFallback      []selection.Candidate
	SelectionCtx        selection.SelectionContext
	Constraints         portfoliolayer.ConstraintSet
	Budget              *portfoliolayer.AllocationBudget
	Filter              FilterCompatRequest
	AttachShadow        bool
	LegacyAmountPerName float64
	// Risk is optional read-only RiskView for PortfolioRiskSnapshot (H0.2).
	Risk *tradingconfig.RiskView
	// DecisionTime stamps the PortfolioDecisionProvider envelope; zero → Snapshot.AsOf / UTC now.
	DecisionTime time.Time
	// SkipRiskTighten skips SuggestTighten/ApplyTightenOnly (Risk snapshot still built when possible).
	SkipRiskTighten bool
}

// SimulatedFilterLine is one potential PlanFilter row. Not a trade_plan_item.
type SimulatedFilterLine struct {
	StockCode       string  `json:"stock_code"`
	TargetAmount    float64 `json:"target_amount"`
	Status          string  `json:"status"` // pending | skipped
	RiskCode        string  `json:"risk_code,omitempty"`
	InAllocationSet bool    `json:"in_allocation_set"`
}

// SimulatedFilterResult is a potential Filter outcome at simulation time.
type SimulatedFilterResult struct {
	Ran           bool                  `json:"ran"`
	Status        string                `json:"status"` // not_run | called
	RiskStatus    string                `json:"risk_status,omitempty"`
	Pending       []SimulatedFilterLine `json:"pending"`
	Skipped       []SimulatedFilterLine `json:"skipped"`
	AcceptedCount int                   `json:"accepted_count"`
	ScanLimit     int                   `json:"scan_limit"`
	MaxNames      int                   `json:"max_names"`
}

// SimulatedDecisionMetrics is absolute sandbox stats (not F.6 vs-legacy deltas).
type SimulatedDecisionMetrics struct {
	RankedCount           int     `json:"ranked_count"`
	SelectedCount         int     `json:"selected_count"`
	WaitlistCount         int     `json:"waitlist_count"`
	RejectedCount         int     `json:"rejected_count"`
	AllocationSetNotional float64 `json:"allocation_set_notional"`
	ReserveCash           float64 `json:"reserve_cash"`
	AvailableCapital      float64 `json:"available_capital"`
	PotentialPendingCount int     `json:"potential_pending_count"`
	PotentialSkippedCount int     `json:"potential_skipped_count"`
	QuantityAlwaysZero    bool    `json:"quantity_always_zero"`
}

// ShadowAnnex optionally attaches F.5/F.6 comparison. It is not the primary result.
type ShadowAnnex struct {
	Report     *portfoliolayer.PortfolioShadowReport  `json:"report"`
	Evaluation *portfoliolayer.ShadowEvaluationReport `json:"evaluation"`
}

// RiskConstraintTrace records H0.2 SuggestTighten → ApplyTightenOnly inside the sandbox.
// Preference-only; never writes RiskView / Execution.
type RiskConstraintTrace struct {
	RecordOnly        bool                         `json:"record_only"`
	NotRiskViewWrite  bool                         `json:"not_risk_view_write"`
	NotExecutionWrite bool                         `json:"not_execution_write"`
	Applied           bool                         `json:"applied"`
	HasPatches        bool                         `json:"has_patches"`
	InputsFingerprint string                       `json:"inputs_fingerprint,omitempty"`
	Notes             []portfoliorisk.TightenNote  `json:"notes"`
}

// LegacyCompareSummary is a fixed_amount projection for sandbox comparison (not a write-chain run).
type LegacyCompareSummary struct {
	Method           string  `json:"method"`
	AmountPerName    float64 `json:"amount_per_name"`
	SelectedCount    int     `json:"selected_count"`
	SumAllocationSet float64 `json:"sum_allocation_set"`
}

// SimulatedPortfolioDecisionResult is an in-memory sandbox decision. Not a TradePlan.
type SimulatedPortfolioDecisionResult struct {
	RecordOnly          bool                               `json:"record_only"`
	NotATradePlan       bool                               `json:"not_a_trade_plan"`
	SnapshotAsOf        time.Time                          `json:"snapshot_as_of"`
	ResolvedConstraints portfoliolayer.ResolvedConstraints `json:"resolved_constraints"`

	Selection *selection.CandidateSelectionResult `json:"selection"`
	Selected  []portfoliolayer.PortfolioPick      `json:"selected"`
	Waitlist  []portfoliolayer.PortfolioPick      `json:"waitlist"`
	Rejected  []portfoliolayer.PortfolioReject    `json:"rejected"`
	ScanList  []selection.Candidate               `json:"scan_list"`

	Budget     portfoliolayer.AllocationBudget  `json:"budget"`
	Allocation *portfoliolayer.AllocationResult `json:"allocation"`

	// H0.2 / H.3 sandbox attachments.
	PortfolioRiskSnapshot *portfoliorisk.PortfolioRiskSnapshot `json:"portfolio_risk_snapshot,omitempty"`
	RiskConstraintTrace   *RiskConstraintTrace                 `json:"risk_constraint_trace,omitempty"`
	DecisionEnvelope      *decisionprovider.DecisionEnvelope   `json:"decision_envelope,omitempty"`
	LegacyCompare         *LegacyCompareSummary                `json:"legacy_compare,omitempty"`

	PotentialFilter SimulatedFilterResult    `json:"potential_filter"`
	Evaluation      SimulatedDecisionMetrics `json:"evaluation"`
	ShadowAnnex     *ShadowAnnex             `json:"shadow_annex,omitempty"`
	Notes           []string                 `json:"notes,omitempty"`
}
