// Package portfoliohistory stores read-only daily observation summaries for the
// Portfolio Decision Dashboard history strip.
//
// Each day may keep compact projections of:
//   - PortfolioValidationReport (day slice / aggregate hints)
//   - PortfolioRiskSnapshot
//   - Allocation Shadow (from ShadowComparisonRecord / Insight view)
//
// This is NOT a backtest, NOT PnL / Sharpe analysis, and NOT auto-optimisation.
// It never writes TradePlan, never calls Execution, and never switches Provider.
//
// DefaultEnabled is false: RecordDay is a no-op unless Enabled=true.
package portfoliohistory

import "time"

const (
	SchemaVersion = "portfolio_history.p13-v1"
	DaySchema     = "portfolio_history_day.p13-v1"

	// DefaultEnabled is compile-time OFF for persistence.
	DefaultEnabled = false

	DisclaimerKey = "portfolio_history.disclaimer.v1"
	DisclaimerZH  = "本页为组合决策观察历史摘要，不构成投资建议，不是回测或收益分析，不会自动买卖或切换 Provider。"
)

// ReasonCount is a compact histogram bucket.
type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// ValidationDaySummary is a persist-safe slice of Validation (decision behaviour only).
type ValidationDaySummary struct {
	Present bool `json:"present"`
	Skipped bool `json:"skipped,omitempty"`
	OK      bool `json:"ok,omitempty"`

	CaseID    string `json:"case_id,omitempty"`
	TradeDate string `json:"trade_date,omitempty"`

	NameCountLegacy    int `json:"name_count_legacy,omitempty"`
	NameCountPortfolio int `json:"name_count_portfolio,omitempty"`
	NameCountDelta     int `json:"name_count_delta,omitempty"`

	LegacyBuyNotional    float64 `json:"legacy_buy_notional,omitempty"`
	PortfolioBuyNotional float64 `json:"portfolio_buy_notional,omitempty"`
	NotionalDelta        float64 `json:"notional_delta,omitempty"` // portfolio − legacy

	TightenCountLegacy    int `json:"tighten_count_legacy,omitempty"`
	TightenCountPortfolio int `json:"tighten_count_portfolio,omitempty"`

	FilterRejectTop []ReasonCount `json:"filter_reject_top,omitempty"` // portfolio side, top N
	SectorAvailable bool          `json:"sector_available,omitempty"`

	SkipReason string `json:"skip_reason,omitempty"`
	Note       string `json:"note,omitempty"`
}

// RiskSnapshotSummary is a compact book-shape card (no position list, no PnL).
type RiskSnapshotSummary struct {
	Present bool `json:"present"`
	Found   bool `json:"found,omitempty"`

	GrossExposure *float64 `json:"gross_exposure,omitempty"`
	CashRatio     *float64 `json:"cash_ratio,omitempty"`
	HeadroomVsCap *float64 `json:"headroom_vs_cap,omitempty"`
	Top1Weight    *float64 `json:"top1_weight,omitempty"`
	NameCount     int      `json:"name_count,omitempty"`

	SectorAvailable   bool     `json:"sector_available,omitempty"`
	MaxSectorWeight   *float64 `json:"max_sector_weight,omitempty"`
	InputsFingerprint string   `json:"inputs_fingerprint,omitempty"`
	Note              string   `json:"note,omitempty"`
}

// AllocationShadowSummary is a compact Legacy vs Portfolio allocation observation.
type AllocationShadowSummary struct {
	Present bool `json:"present"`

	Comparable         bool `json:"comparable,omitempty"`
	LegacyLineCount    int  `json:"legacy_line_count,omitempty"`
	PortfolioLineCount int  `json:"portfolio_line_count,omitempty"`
	OnlyLegacyCount    int  `json:"only_legacy_count,omitempty"`
	OnlyPortfolioCount int  `json:"only_portfolio_count,omitempty"`
	CommonCount        int  `json:"common_count,omitempty"`

	LegacyBuyNotional    *float64 `json:"legacy_buy_notional,omitempty"`
	PortfolioBuyNotional *float64 `json:"portfolio_buy_notional,omitempty"`
	BudgetBinding        string   `json:"budget_binding,omitempty"`

	TightenApplied       bool `json:"tighten_applied,omitempty"`
	SuggestHasPatches    bool `json:"suggest_has_patches,omitempty"`
	GrossHeadroomBinding bool `json:"gross_headroom_binding,omitempty"`
	SingleCapApplied     bool `json:"single_cap_applied,omitempty"`
	BlockedNewEntries    bool `json:"blocked_new_entries,omitempty"`

	ShadowFingerprint string `json:"shadow_fingerprint,omitempty"`
	Note              string `json:"note,omitempty"`
}

// DailyRecord is one trade-date history row (summaries only).
type DailyRecord struct {
	SchemaVersion string    `json:"schema_version"`
	TradeDate     string    `json:"trade_date"`
	RecordedAt    time.Time `json:"recorded_at"`
	AccountHint   string    `json:"account_hint,omitempty"` // hash8 or empty; never raw secrets

	Validation       ValidationDaySummary    `json:"validation"`
	Risk             RiskSnapshotSummary     `json:"risk"`
	AllocationShadow AllocationShadowSummary `json:"allocation_shadow"`

	DataGaps []string `json:"data_gaps,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

// HistoryRollup is multi-day observation counts (not returns).
type HistoryRollup struct {
	DayCount              int `json:"day_count"`
	ValidationPresentDays int `json:"validation_present_days"`
	ValidationOKDays      int `json:"validation_ok_days"`
	RiskFoundDays         int `json:"risk_found_days"`
	ShadowPresentDays     int `json:"shadow_present_days"`
	ShadowComparableDays  int `json:"shadow_comparable_days"`
}

// PortfolioHistoryView is the Dashboard-facing read model.
type PortfolioHistoryView struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`

	RecordOnly        bool `json:"record_only"`
	ReadOnly          bool `json:"read_only"`
	NotABacktest      bool `json:"not_a_backtest"`
	NotPnL            bool `json:"not_pnl"`
	NotSharpe         bool `json:"not_sharpe"`
	NotAutoTune       bool `json:"not_auto_tune"`
	NotATradePlan     bool `json:"not_a_trade_plan"`
	NotExecution      bool `json:"not_execution"`
	NotProviderSwitch bool `json:"not_provider_switch"`
	AnalysisToolOnly  bool `json:"analysis_tool_only"`

	Disclaimer    string `json:"disclaimer"`
	DisclaimerKey string `json:"disclaimer_key"`

	Days   []DailyRecord `json:"days"`
	Rollup HistoryRollup `json:"rollup"`

	DataGaps       []string `json:"data_gaps,omitempty"`
	DataSourceNote string   `json:"data_source_note"`
}
