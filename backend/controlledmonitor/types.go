// Package controlledmonitor is the Controlled Pilot Observation Monitor (Phase13).
//
// It assembles a read-only ControlledPilotObservationReport from injected
// ShadowComparisonRecord, Provider metadata, Portfolio allocation summaries,
// and PlanFilter-shaped outcomes. It never writes TradePlan, never calls
// Execution, and never switches DecisionProvider / controlledadoption.
//
// DefaultEnabled is false. Callers must pass Enabled=true to produce a filled report.
package controlledmonitor

import "time"

const (
	SchemaVersion = "controlled_pilot_observation.p13-v1"

	// DefaultEnabled is compile-time OFF: Observe skips unless Input.Enabled=true.
	DefaultEnabled = false

	ProviderFixedAmount         = "fixed_amount"
	ProviderPortfolioAllocation = "portfolio_allocation"

	OutcomeOK             = "ok"
	OutcomeSkipped        = "skipped"
	OutcomePortfolioFail  = "portfolio_failed"
	OutcomeLegacyFail     = "legacy_failed"
	OutcomeBothFail       = "both_failed"
	OutcomeIncomparable   = "incomparable"
	OutcomePartialInputs  = "partial_inputs"
)

// ProviderMetadata is the G.13-style stamp projection (observation only).
type ProviderMetadata struct {
	ProviderMode      string `json:"provider_mode,omitempty"`
	DecisionProvider  string `json:"decision_provider,omitempty"`
	DecisionVersion   string `json:"decision_version,omitempty"`
	AllocationVersion string `json:"allocation_version,omitempty"`
}

// AllocationSide is a persist-safe Portfolio/Legacy allocation summary.
// Callers map from AllocationEngine / envelope; this package does not run Allocate.
type AllocationSide struct {
	Present          bool    `json:"present"`
	OK               bool    `json:"ok"`
	Method           string  `json:"method,omitempty"`
	SelectedCount    int     `json:"selected_count"`
	WaitlistCount    int     `json:"waitlist_count"`
	SumNotional      float64 `json:"sum_notional"`
	AvailableCapital float64 `json:"available_capital,omitempty"`
	ReserveCash      float64 `json:"reserve_cash,omitempty"`
	Binding          string  `json:"binding,omitempty"`
	ErrorCode        string  `json:"error_code,omitempty"`
}

// FilterSide is a PlanFilter-shaped outcome. Not a TradePlan item list.
type FilterSide struct {
	Present       bool           `json:"present"`
	AcceptedCount int            `json:"accepted_count"`
	RejectedCount int            `json:"rejected_count"`
	RiskStatus    string         `json:"risk_status,omitempty"`
	RejectReasons map[string]int `json:"reject_reasons,omitempty"`
}

// AllocationDiff is Legacy vs Portfolio allocation shape.
type AllocationDiff struct {
	Present              bool    `json:"present"`
	LegacySelectedCount  int     `json:"legacy_selected_count"`
	PortfolioSelectedCount int   `json:"portfolio_selected_count"`
	SelectedCountDelta   int     `json:"selected_count_delta"` // portfolio − legacy
	LegacyWaitlistCount  int     `json:"legacy_waitlist_count"`
	PortfolioWaitlistCount int   `json:"portfolio_waitlist_count"`
	LegacyMethod         string  `json:"legacy_method,omitempty"`
	PortfolioMethod      string  `json:"portfolio_method,omitempty"`
	LegacyBinding        string  `json:"legacy_binding,omitempty"`
	PortfolioBinding     string  `json:"portfolio_binding,omitempty"`
	CapitalDelta         float64 `json:"capital_delta,omitempty"` // portfolio capital − legacy implied/capital
}

// AmountDiffBlock is signed notional comparison (observation counts, not a score).
type AmountDiffBlock struct {
	Present           bool    `json:"present"`
	LegacySum         float64 `json:"legacy_sum"`
	PortfolioSum      float64 `json:"portfolio_sum"`
	Delta             float64 `json:"delta"` // portfolio − legacy
	AbsDelta          float64 `json:"abs_delta"`
	SampleCount       int     `json:"sample_count,omitempty"` // per-symbol both_actual samples when from Shadow
	AmountDeltaCount  int     `json:"amount_delta_count,omitempty"`
}

// SymbolCountDiff is name-set cardinality (and optional set counts from Shadow).
type SymbolCountDiff struct {
	Present            bool `json:"present"`
	LegacyCount        int  `json:"legacy_count"`
	PortfolioCount     int  `json:"portfolio_count"`
	Delta              int  `json:"delta"` // portfolio − legacy
	OnlyLegacyCount    int  `json:"only_legacy_count,omitempty"`
	OnlyPortfolioCount int  `json:"only_portfolio_count,omitempty"`
	CommonCount        int  `json:"common_count,omitempty"`
}

// FilterRejectDiff compares accept/reject shape.
type FilterRejectDiff struct {
	Present              bool           `json:"present"`
	LegacyAccepted       int            `json:"legacy_accepted"`
	PortfolioAccepted    int            `json:"portfolio_accepted"`
	AcceptedDelta        int            `json:"accepted_delta"`
	LegacyRejected       int            `json:"legacy_rejected"`
	PortfolioRejected    int            `json:"portfolio_rejected"`
	RejectedDelta        int            `json:"rejected_delta"`
	LegacyRiskStatus     string         `json:"legacy_risk_status,omitempty"`
	PortfolioRiskStatus  string         `json:"portfolio_risk_status,omitempty"`
	PortfolioRejectReasons map[string]int `json:"portfolio_reject_reasons,omitempty"`
}

// CashUsageDiff compares buy notional / reserve usage (not account cash balance dump).
type CashUsageDiff struct {
	Present              bool    `json:"present"`
	LegacyBuyNotional    float64 `json:"legacy_buy_notional"`
	PortfolioBuyNotional float64 `json:"portfolio_buy_notional"`
	BuyNotionalDelta     float64 `json:"buy_notional_delta"`
	LegacyReserve        float64 `json:"legacy_reserve"`
	PortfolioReserve     float64 `json:"portfolio_reserve"`
	ReserveDelta         float64 `json:"reserve_delta"`
}

// Outcome is success/failure of the observed pilot decision path (not Execution).
type Outcome struct {
	Success bool   `json:"success"`
	Failure bool   `json:"failure"`
	Code    string `json:"code"` // Outcome* constants
	Detail  string `json:"detail,omitempty"`
}

// ControlledPilotObservationReport is the monitor output ticket.
type ControlledPilotObservationReport struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	TradeDate     string    `json:"trade_date,omitempty"`

	Enabled    bool   `json:"enabled"`
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skip_reason,omitempty"`

	RecordOnly         bool `json:"record_only"`
	ReadOnly           bool `json:"read_only"`
	NotATradePlan      bool `json:"not_a_trade_plan"`
	NotExecution       bool `json:"not_execution"`
	NotProviderSwitch  bool `json:"not_provider_switch"`
	NotAutoExpandScope bool `json:"not_auto_expand_scope"`

	ProviderUsed     string           `json:"provider_used"` // fixed_amount | portfolio_allocation | unknown | ""
	ProviderMetadata ProviderMetadata `json:"provider_metadata"`

	Success bool    `json:"success"`
	Failure bool    `json:"failure"`
	Outcome Outcome `json:"outcome"`

	AllocationDiff   AllocationDiff   `json:"allocation_diff"`
	AmountDiff       AmountDiffBlock  `json:"amount_diff"`
	SymbolCountDiff  SymbolCountDiff  `json:"symbol_count_diff"`
	FilterRejectDiff FilterRejectDiff `json:"filter_reject_diff"`
	CashUsageDiff    CashUsageDiff    `json:"cash_usage_diff"`

	Sources        SourceFlags `json:"sources"`
	DataGaps       []string    `json:"data_gaps,omitempty"`
	Notes          []string    `json:"notes,omitempty"`
	DataSourceNote string      `json:"data_source_note"`
}

// SourceFlags records which inputs were present.
type SourceFlags struct {
	ShadowPresent     bool `json:"shadow_present"`
	MetadataPresent   bool `json:"metadata_present"`
	LegacyAllocPresent bool `json:"legacy_alloc_present"`
	PortfolioAllocPresent bool `json:"portfolio_alloc_present"`
	LegacyFilterPresent   bool `json:"legacy_filter_present"`
	PortfolioFilterPresent bool `json:"portfolio_filter_present"`
}
