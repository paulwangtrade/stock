package dailypilot

import (
	"time"

	"go-stock/backend/controlledmonitor"
	"go-stock/backend/portfoliohistory"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/providershadow"
)

const (
	SchemaVersion = "daily_pilot.p13-v1"

	// DefaultEnabled is compile-time OFF.
	DefaultEnabled = false

	verdictUnset           = "unset"
	verdictContinueObserve = "continue_observe"
	verdictKill            = "kill"
	verdictReset           = "reset"
	verdictDenyNextDate    = "deny_next_date"
	verdictIncompleteData  = "incomplete_data"

	transitionUnchanged         = "unchanged"
	transitionEnabledControlled = "enabled_controlled"
	transitionKilled            = "killed"
	transitionResetToOff        = "reset_to_off"
	transitionScopeChanged      = "scope_changed"
	transitionUnknown           = "unknown"

	actionNone              = "none"
	actionConsiderKill      = "consider_kill"
	actionConsiderReset     = "consider_reset"
	actionConsiderKeepOff   = "consider_keep_off"
	actionInvestigateScope  = "investigate_scope"

	maxSymbolSample = 8
	maxReasons      = 10
)

// PolicySnapshotInput is adoption policy supplied by callers (read-only projection).
// Account/strategy lists are used for counts and hash8 only — never serialized verbatim.
type PolicySnapshotInput struct {
	Adoption      string
	KillSwitch    bool
	AccountIDs    []uint
	StrategyNames []string
	TradeDates    []string
}

// WriteChainHints optional read-only write-chain counters (no plan items / cash).
type WriteChainHints struct {
	BlockedDraftCount   int
	PortfolioDraftCount int
	LegacyDraftCount    int
	PersistAttempts     int
	PersistSuccesses    int
}

// DailyPilotInput aggregates read-only pilot observation sources for one trade date.
type DailyPilotInput struct {
	// Enabled must be true to build a filled report. Default false → skipped.
	Enabled bool

	TradeDate   string
	ReviewID    string
	GeneratedAt time.Time

	ShadowRecords     []providershadow.ShadowComparisonRecord
	Validation        *portfoliovalidation.PortfolioValidationReport
	ControlledMonitor *controlledmonitor.ControlledPilotObservationReport
	PortfolioHistory  *portfoliohistory.PortfolioHistoryView

	PolicySnapshotOpen  PolicySnapshotInput
	PolicySnapshotClose PolicySnapshotInput

	WriteChainHints *WriteChainHints
}

// ReasonCount is a histogram bucket.
type ReasonCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// PilotScope is the pilot-window projection (counts + hash8, no whitelist plaintext).
type PilotScope struct {
	AccountWhitelistCount  int      `json:"account_whitelist_count"`
	StrategyWhitelistCount int      `json:"strategy_whitelist_count"`
	DateWhitelistCount     int      `json:"date_whitelist_count"`
	AccountIDHash8         string   `json:"account_id_hash8,omitempty"`
	StrategyNameHash8      string   `json:"strategy_name_hash8,omitempty"`
	DatesListed            []string `json:"dates_listed,omitempty"`
	ScopeExpanded          bool     `json:"scope_expanded"`
	Adoption               string   `json:"adoption"`
	KillSwitch             bool     `json:"kill_switch"`
}

// SourcePresence records which inputs were present.
type SourcePresence struct {
	ShadowPresent        bool `json:"shadow_present"`
	ShadowRecordCount    int  `json:"shadow_record_count"`
	ValidationPresent    bool `json:"validation_present"`
	ValidationSkipped    bool `json:"validation_skipped"`
	MonitorPresent       bool `json:"monitor_present"`
	MonitorSkipped       bool `json:"monitor_skipped"`
	HistoryPresent       bool `json:"history_present"`
	HistoryDayMatched    bool `json:"history_day_matched"`
	PolicyOpenPresent    bool `json:"policy_open_present"`
	PolicyClosePresent   bool `json:"policy_close_present"`
	WriteChainHintsPresent bool `json:"write_chain_hints_present"`
}

// NameCountBlock compares Legacy vs Portfolio name counts.
type NameCountBlock struct {
	AvgLegacy    float64 `json:"avg_legacy,omitempty"`
	AvgPortfolio float64 `json:"avg_portfolio,omitempty"`
	DeltaMean    float64 `json:"delta_mean,omitempty"`
}

// SymbolDiffBlock is set cardinality (optional samples for internal pilot use).
type SymbolDiffBlock struct {
	OnlyLegacyCount    int      `json:"only_legacy_count"`
	OnlyPortfolioCount int      `json:"only_portfolio_count"`
	CommonCount        int      `json:"common_count"`
	OnlyLegacySample   []string `json:"only_legacy_sample,omitempty"`
	OnlyPortfolioSample []string `json:"only_portfolio_sample,omitempty"`
}

// AmountBlock aggregates signed amount deltas (omit zeros when no samples).
type AmountBlock struct {
	AmountDeltaCount int     `json:"amount_delta_count"`
	AvgSignedDelta   float64 `json:"avg_signed_delta,omitempty"`
	AvgAbsDelta      float64 `json:"avg_abs_delta,omitempty"`
	SampleCount      int     `json:"sample_count"`
}

// ValidationOverlayBlock optional validation day overlay.
type ValidationOverlayBlock struct {
	Used           bool    `json:"used"`
	NameCountDelta int     `json:"name_count_delta,omitempty"`
	NotionalDelta  float64 `json:"notional_delta,omitempty"`
	Skipped        bool    `json:"skipped"`
	DayUnmatched   bool    `json:"day_unmatched,omitempty"`
}

// LegacyVsPortfolioBlock is §3.1 Legacy vs Portfolio diff summary.
type LegacyVsPortfolioBlock struct {
	ComparableCount      int                    `json:"comparable_count"`
	IncomparableCount    int                    `json:"incomparable_count"`
	IncomparableReasons  []ReasonCount          `json:"incomparable_reasons,omitempty"`
	NameCount            NameCountBlock         `json:"name_count"`
	SymbolDiff           SymbolDiffBlock        `json:"symbol_diff"`
	Amount               AmountBlock            `json:"amount"`
	RoleChangeCount      int                    `json:"role_change_count"`
	ValidationOverlay    ValidationOverlayBlock `json:"validation_overlay"`
	MonitorOverlay       MonitorOverlayBlock    `json:"monitor_overlay"`
}

// MonitorOverlayBlock folds ControlledMonitor diffs when present.
type MonitorOverlayBlock struct {
	Present            bool   `json:"present"`
	OutcomeCode        string `json:"outcome_code,omitempty"`
	ProviderUsed       string `json:"provider_used,omitempty"`
	SymbolCountDelta   int    `json:"symbol_count_delta,omitempty"`
	AcceptedDelta      int    `json:"accepted_delta,omitempty"`
	RejectedDelta      int    `json:"rejected_delta,omitempty"`
}

// AllocationBlock is §3.2 allocation observation.
type AllocationBlock struct {
	RecordsWithAllocSummary int           `json:"records_with_alloc_summary"`
	ComparableAllocCount    int           `json:"comparable_alloc_count"`
	BindingHistogram        []ReasonCount `json:"binding_histogram,omitempty"`
	Reserve                 ReserveBlock  `json:"reserve"`
	Budget                  BudgetBlock   `json:"budget"`
	Waitlist                WaitlistBlock `json:"waitlist"`
	MinOrderZeroCountSum    int           `json:"min_order_zero_count_sum"`
	ChangeReasons           []ReasonCount `json:"change_reasons,omitempty"`
	HistoryShape            HistoryShapeBlock `json:"history_shape,omitempty"`
}

type ReserveBlock struct {
	AvgLegacy     float64 `json:"avg_legacy,omitempty"`
	AvgPortfolio  float64 `json:"avg_portfolio,omitempty"`
	DeltaMean     float64 `json:"delta_mean,omitempty"`
	SampleCount   int     `json:"sample_count"`
}

type BudgetBlock struct {
	LegacyHasBudgetCount       int     `json:"legacy_has_budget_count"`
	CapitalVsImpliedDeltaMean  float64 `json:"capital_vs_implied_delta_mean,omitempty"`
	SampleCount                int     `json:"sample_count"`
}

type WaitlistBlock struct {
	AvgLegacyCount    float64 `json:"avg_legacy_count,omitempty"`
	AvgPortfolioCount float64 `json:"avg_portfolio_count,omitempty"`
	SampleCount       int     `json:"sample_count"`
}

type HistoryShapeBlock struct {
	Available          bool     `json:"available"`
	GrossHeadroomVsCap *float64 `json:"gross_headroom_vs_cap,omitempty"`
	Top1Weight         *float64 `json:"top1_weight,omitempty"`
	CashRatio          *float64 `json:"cash_ratio,omitempty"`
	NameCount          int      `json:"name_count,omitempty"`
}

// ShadowTightenBlock shadow risk tighten trace.
type ShadowTightenBlock struct {
	Present               bool     `json:"present"`
	AppliedCount          int      `json:"applied_count"`
	SuggestHasPatchesCount int     `json:"suggest_has_patches_count"`
	EffectiveMaxNewNames  *int     `json:"effective_max_new_names,omitempty"`
	SkipAlreadyHolding    *bool    `json:"skip_already_holding,omitempty"`
	NoteCodes             []string `json:"note_codes,omitempty"`
}

type ShadowCutsBlock struct {
	SingleCapAppliedCount      int  `json:"single_cap_applied_count"`
	GrossHeadroomBindingCount  int  `json:"gross_headroom_binding_count"`
	BlockedNewEntriesCount     int  `json:"blocked_new_entries_count"`
	MinOrderZeroCountSum       int  `json:"min_order_zero_count_sum"`
	LegacyIgnoresCaps          bool `json:"legacy_ignores_caps"`
}

type ObservationRiskBlock struct {
	Present               bool     `json:"present"`
	RiskLevel             string   `json:"risk_level,omitempty"`
	AllowSectorConstraint bool     `json:"allow_sector_constraint,omitempty"`
	ExplainCodes          []string `json:"explain_codes,omitempty"`
}

type ValidationRiskBlock struct {
	Present                   bool          `json:"present"`
	Skipped                   bool          `json:"skipped"`
	TightenCountLegacy        int           `json:"tighten_count_legacy,omitempty"`
	TightenCountPortfolio     int           `json:"tighten_count_portfolio,omitempty"`
	FilterRejectReasonsPortfolio []ReasonCount `json:"filter_reject_reasons_portfolio,omitempty"`
}

// RiskConstraintBlock is §3.3 risk constraint layers.
type RiskConstraintBlock struct {
	ShadowTighten ShadowTightenBlock  `json:"shadow_tighten"`
	ShadowCuts    ShadowCutsBlock     `json:"shadow_cuts"`
	HistoryRisk   ObservationRiskBlock `json:"history_risk"`
	Validation    ValidationRiskBlock `json:"validation"`
}

// ShadowFailureBlock shadow-side failures.
type ShadowFailureBlock struct {
	PortfolioFailedCount int           `json:"portfolio_failed_count"`
	LegacyFailedCount    int           `json:"legacy_failed_count"`
	BothFailedCount      int           `json:"both_failed_count"`
	ErrorReasonHistogram []ReasonCount `json:"error_reason_histogram,omitempty"`
}

type WriteChainFailureBlock struct {
	Present             bool    `json:"present"`
	BlockedDraftCount   int     `json:"blocked_draft_count,omitempty"`
	PortfolioDraftCount int     `json:"portfolio_draft_count,omitempty"`
	LegacyDraftCount    int     `json:"legacy_draft_count,omitempty"`
	PersistSuccessRate  float64 `json:"persist_success_rate,omitempty"`
}

// FailureBlock is §3.4 failure summary.
type FailureBlock struct {
	Shadow                 ShadowFailureBlock     `json:"shadow"`
	WriteChain             WriteChainFailureBlock `json:"write_chain"`
	ValidationDaysNotOK    int                    `json:"validation_days_not_ok"`
	HumanWatchCodes        []string               `json:"human_watch_codes,omitempty"`
}

type PolicyPoint struct {
	Adoption                 string `json:"adoption"`
	KillSwitch               bool   `json:"kill_switch"`
	AccountWhitelistCount    int    `json:"account_whitelist_count"`
	StrategyWhitelistCount   int    `json:"strategy_whitelist_count"`
	DateWhitelistCount       int    `json:"date_whitelist_count"`
}

type ScopeDeltaBlock struct {
	AccountCountDelta  int  `json:"account_count_delta"`
	StrategyCountDelta int  `json:"strategy_count_delta"`
	DateCountDelta     int  `json:"date_count_delta"`
	Expanded           bool `json:"expanded"`
}

// RollbackBlock is §3.5 rollback / scope observation.
type RollbackBlock struct {
	Open                     PolicyPoint   `json:"open"`
	Close                    PolicyPoint   `json:"close"`
	Transition               string        `json:"transition"`
	KillSwitchNow            bool          `json:"kill_switch_now"`
	AdoptionNow              string        `json:"adoption_now"`
	StillInPilotWindow       bool          `json:"still_in_pilot_window"`
	ScopeDelta               ScopeDeltaBlock `json:"scope_delta"`
	RecommendedHumanAction   string        `json:"recommended_human_action"`
	FrozenPlansUntouched     bool          `json:"frozen_plans_untouched"`
}

// HumanVerdictBlock is filled by humans after export; generator keeps unset.
type HumanVerdictBlock struct {
	Status            string `json:"status"`
	NoteSafe          string `json:"note_safe,omitempty"`
	NextDateApproved  bool   `json:"next_date_approved"`
}

// DailyPilotReport is the daily Controlled Pilot observation ticket.
type DailyPilotReport struct {
	SchemaVersion string    `json:"schema_version"`
	TradeDate     string    `json:"trade_date"`
	GeneratedAt   time.Time `json:"generated_at"`
	ReviewID      string    `json:"review_id,omitempty"`

	Enabled    bool   `json:"enabled"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`

	RecordOnly          bool `json:"record_only"`
	ReadOnly            bool `json:"read_only"`
	NotAutoTune         bool `json:"not_auto_tune"`
	NotAutoExpandScope  bool `json:"not_auto_expand_scope"`
	NotATradePlan       bool `json:"not_a_trade_plan"`
	NotExecution        bool `json:"not_execution"`
	NotProviderSwitch   bool `json:"not_provider_switch"`
	NotABacktest        bool `json:"not_a_backtest"`

	Scope           PilotScope       `json:"scope"`
	Sources         SourcePresence   `json:"sources"`
	DataGaps        []string         `json:"data_gaps,omitempty"`
	KnownGaps       []string         `json:"known_gaps,omitempty"`

	LegacyVsPortfolio LegacyVsPortfolioBlock `json:"legacy_vs_portfolio"`
	Allocation        AllocationBlock        `json:"allocation"`
	RiskConstraint    RiskConstraintBlock    `json:"risk_constraint"`
	Failure           FailureBlock           `json:"failure"`
	Rollback          RollbackBlock          `json:"rollback"`

	HumanVerdict HumanVerdictBlock `json:"human_verdict"`
	ExportNote   string            `json:"export_note"`
}
