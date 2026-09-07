package allocationengine

// Package allocationengine is the Phase12-H.3 Allocation Engine: pure "how much money".
// Standalone package — not wired into the production buy write chain.
// Does not call PlanFilter, persist TradePlan, reference Execution, or modify FixedAmountSizer.

const (
	SchemaVersion = "allocationengine.h3-v1"

	MethodEqualWeight   = "equal_weight"
	WaitlistCopyUniform = "copy_uniform"

	ReasonEqualSplit         = "equal_split"
	ReasonCappedSingleWeight = "capped_single_weight"
	ReasonBelowMinOrder      = "below_min_order"
	ReasonWaitlistUniform    = "waitlist_uniform"
	ReasonNoAllocationSet    = "no_allocation_set"
	ReasonNoAccount          = "no_account"

	BindingOK        = "ok"
	BindingCash      = "cash"
	BindingGross     = "gross"
	BindingBlocked   = "blocked"
	BindingNoAccount = "no_account"
)

// SelectionResult is the F.1 "who" input (selected + waitlist). Rejected names are omitted.
type SelectionResult struct {
	Selected []string
	Waitlist []string
}

// PortfolioSnapshot is a read-only book projection. Engine does not query DB.
type PortfolioSnapshot struct {
	Found        bool
	Equity       float64
	Cash         float64
	ReservedCash float64
	Exposure     float64
}

// AllocationBudget is the round buy budget (reserve reflected in AvailableCash / Capital).
type AllocationBudget struct {
	AvailableCash    float64
	ReserveCash      float64
	RiskBudget       float64
	AvailableCapital float64
	Binding          string
	PolicyGrossPct   float64
}

// ResolvedConstraints are effective generation-time caps (F.2 resolve result shape).
type ResolvedConstraints struct {
	MaxGrossExposurePct float64
	MaxSingleWeight     float64
	ReserveCashRatio    float64
	MinOrderAmount      float64
	BlockNewEntries     bool
}

// Options controls split behaviour. v1 method is equal_weight only.
type Options struct {
	Method             string // empty → equal_weight
	WaitlistAmountMode string // empty → copy_uniform
	ApplyMinOrder      *bool  // nil → true
	// BudgetNilMeansCompute: when Budget==nil, ComputeBudget from Snapshot+Resolved.
	// If false and Budget==nil, capital is treated as 0.
}

// EngineInput is SelectionResult + PortfolioSnapshot + AllocationBudget + ResolvedConstraints.
type EngineInput struct {
	Selection SelectionResult
	Snapshot  *PortfolioSnapshot
	// Budget nil → ComputeBudget(Snapshot, Resolved). Non-nil → use injected budget as-is.
	Budget   *AllocationBudget
	Resolved ResolvedConstraints
	Options  Options
}

// NameAllocation is one name envelope. TargetQuantity is always 0 in v1.
type NameAllocation struct {
	StockCode        string  `json:"stock_code"`
	TargetAmount     float64 `json:"target_amount"`
	TargetQuantity   int64   `json:"target_quantity"`
	AllocationReason string  `json:"allocation_reason"`
	InAllocationSet  bool    `json:"in_allocation_set"`
}

// AllocationResult is the H.3 engine output.
type AllocationResult struct {
	SchemaVersion string           `json:"schema_version"`
	Method        string           `json:"method"`
	UniformAmount float64          `json:"uniform_amount"`
	Budget        AllocationBudget `json:"budget"`
	Items         []NameAllocation `json:"items"`     // selected ∥ waitlist scan order
	Allocated     []NameAllocation `json:"allocated"` // in_allocation_set only
	Waitlist      []NameAllocation `json:"waitlist"`  // waitlist rows (kept, not promoted)
}

// --- Backward-compatible aliases (used by portfoliolayer shadow bridge) ---

type Selection = SelectionResult
type Snapshot = PortfolioSnapshot
type Budget = AllocationBudget
type Policy = ResolvedConstraints
type Result = AllocationResult

// Input is the legacy Run() shape (budget always present).
type Input struct {
	Selection SelectionResult
	Snapshot  *PortfolioSnapshot
	Budget    AllocationBudget
	Policy    ResolvedConstraints
	Options   Options
}
