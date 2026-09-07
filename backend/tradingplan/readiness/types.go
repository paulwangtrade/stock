// Package readiness evaluates whether a TradePlan could execute against the
// current paper_sim account — read-only observation; no orders or gateway calls.
package readiness

// Status values for overall execution readiness.
const (
	StatusReady    = "READY"
	StatusWarning  = "WARNING"
	StatusBlocked  = "BLOCKED"
)

// Concentration levels after simulated full execution of plan BUY items.
const (
	ConcentrationNormal   = "NORMAL"
	ConcentrationElevated = "ELEVATED"
	ConcentrationHigh     = "HIGH"
)

const (
	IssueCashInsufficient = "CASH_INSUFFICIENT"
	IssueAlreadyHolding   = "ALREADY_HOLDING"
	IssueHighConcentration = "HIGH_CONCENTRATION"
)

// PlanItem is a minimal trade-plan line for evaluation (decoupled from models).
type PlanItem struct {
	StockCode    string
	StockName    string
	Side         string
	TargetAmount float64
	TargetVolume int64
	LimitPrice   float64
	RefPrice     float64
	OpenRefPrice float64
	Status       string
	IntentStatus string
}

// AccountSnapshot is paper_sim account cash/equity for readiness checks.
type AccountSnapshot struct {
	Cash   float64
	Equity float64
}

// PositionSnapshot is an existing holding used for conflict detection.
type PositionSnapshot struct {
	StockCode       string
	StockName       string
	TotalVolume     int64
	AvailableVolume int64
	AvgCost         float64
	MarkPrice       float64
}

// PositionConflict describes a BUY target that already has a position.
type PositionConflict struct {
	StockCode   string `json:"stock_code"`
	StockName   string `json:"stock_name"`
	Side        string `json:"side"`
	Quantity    int64  `json:"quantity"`
	Message     string `json:"message"`
}

// ReadinessIssue is one observation finding (warn/block context for UI).
type ReadinessIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // WARN | BLOCK
	Message  string `json:"message"`
}

// PositionWeightProjection is simulated post-execution weight for one BUY line.
type PositionWeightProjection struct {
	StockCode      string  `json:"stock_code"`
	StockName      string  `json:"stock_name"`
	RequiredCash   float64 `json:"required_cash"`
	ProjectedWeight float64 `json:"projected_weight"`
	WeightPct      float64 `json:"weight_pct"`
}

// ExecutionReadiness is the E.5 observation report.
type ExecutionReadiness struct {
	PlanID            int                        `json:"plan_id"`
	AvailableCash     float64                    `json:"available_cash"`
	RequiredCash      float64                    `json:"required_cash"`
	CashEnough        bool                       `json:"cash_enough"`
	TotalEquity       float64                    `json:"total_equity"`
	ExistingPositions []PositionConflict         `json:"conflicts"`
	Issues            []ReadinessIssue           `json:"issues"`
	AfterExecution    []PositionWeightProjection `json:"after_execution"`
	Concentration     string                     `json:"concentration"`
	Status            string                     `json:"status"`
}
