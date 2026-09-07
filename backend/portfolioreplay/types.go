package portfolioreplay

import (
	"time"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliosim"
	"go-stock/backend/selection"
)

const (
	SchemaReplayCaseV1 = "replay_case.v1"

	SourceFileFixture    = "file_fixture"
	SourceManualScenario = "manual_scenario"
	SourceDBSnapshot     = "db_snapshot" // allowed on disk only; runner still does not query DB
)

const asOfMismatchTolerance = time.Second

// EngineVersions pins algorithm identity (F.10).
type EngineVersions struct {
	SelectionVersion  string `json:"selection_version"`
	ConstraintVersion string `json:"constraint_version"`
	AllocationVersion string `json:"allocation_version"`
	SimulationVersion string `json:"simulation_version"`
}

// ReplayCase is a frozen decision input. It is not a TradePlan.
type ReplayCase struct {
	SchemaVersion string         `json:"schema_version"`
	CaseID        string         `json:"case_id"`
	DecisionTime  time.Time      `json:"decision_time"`
	TradeDate     string         `json:"trade_date,omitempty"`
	Source        string         `json:"source"`
	SourceRef     string         `json:"source_ref,omitempty"`
	Versions      EngineVersions `json:"versions"`
	Snapshot      *portfoliolayer.PortfolioSnapshot
	Candidates    *selection.CandidateSelectionResult
	Constraints   portfoliolayer.ConstraintSet
	Budget        *portfoliolayer.AllocationBudget
	Filter        portfoliosim.FilterCompatRequest
	Notes         string
}

// CountBin is one histogram bucket.
type CountBin struct {
	Value int `json:"value"`
	Count int `json:"count"`
}

// AllocationDistribution summarizes per-case allocation-set notionals.
type AllocationDistribution struct {
	Values []float64 `json:"values"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
	Mean   float64   `json:"mean"`
}

// PotentialFilterSummary aggregates F.8 potential Filter outcomes.
type PotentialFilterSummary struct {
	CalledCount   int            `json:"called_count"`
	NotRunCount   int            `json:"not_run_count"`
	PendingTotal  int            `json:"pending_total"`
	SkippedTotal  int            `json:"skipped_total"`
	RiskStatus    map[string]int `json:"risk_status"`
	RejectReasons map[string]int `json:"reject_reasons"`
}

// ReplayError is a per-case load or run failure.
type ReplayError struct {
	CaseID string `json:"case_id"`
	Reason string `json:"reason"`
}

// PortfolioReplayReport is the F.9 aggregation over Legacy + Portfolio simulations.
// Decision-behavior only: not a backtest, no returns, no parameter search.
type PortfolioReplayReport struct {
	RecordOnly    bool `json:"record_only"`
	NotATradePlan bool `json:"not_a_trade_plan"`
	NotABacktest  bool `json:"not_a_backtest"`

	CaseCount                 int                    `json:"case_count"`
	SuccessCount              int                    `json:"success_count"`
	DecisionCount             int                    `json:"decision_count"`
	DeterministicCheck        bool                   `json:"deterministic_check"`
	SelectedCountDistribution []CountBin             `json:"selected_count_distribution"`
	AllocationDistribution    AllocationDistribution `json:"allocation_distribution"`
	PotentialFilterSummary    PotentialFilterSummary `json:"potential_filter_summary"`

	// DecisionBehavior compares Legacy Simulation vs Portfolio Simulation.
	DecisionBehavior DecisionBehaviorStats `json:"decision_behavior"`
	Rows             []ReplayRow           `json:"rows,omitempty"`

	Errors []ReplayError `json:"errors,omitempty"`
}

// ReplayReport is retained as an alias for existing callers.
type ReplayReport = PortfolioReplayReport

