// Package portfolioevaluation is the Phase13 Portfolio Historical Evaluation Framework.
//
// Batch ReplayCase evaluation: Legacy FixedAmount vs Portfolio Decision.
// Decision-behavior analysis only — not a PnL / return / Sharpe backtest.
//
// DefaultEnabled=false. Read-only. Never creates TradePlan or calls Execution.
package portfolioevaluation

import (
	"time"

	"go-stock/backend/portfolioreplay"
	"go-stock/backend/portfoliovalidation"
)

const (
	SchemaVersion  = "portfolio_historical_evaluation.p13-v1"
	DefaultEnabled = false

	DefaultLegacyAmount = portfoliovalidation.DefaultLegacyAmount
)

// Options controls the framework (all observation / read-only).
type Options struct {
	// Enabled must be true to run; default false → skipped report.
	Enabled bool `json:"enabled"`
	// AttachDecisionShadowV2 enriches sector/cash/risk explain via H3.2 (default false).
	AttachDecisionShadowV2 bool `json:"attach_decision_shadow_v2"`
	// AttachInsight builds portfolioinsight annex from PortfolioRisk when available (default false).
	AttachInsight bool `json:"attach_insight"`
	// SkipFilterCompat skips read-only PlanFilter compatibility (default runs filter).
	SkipFilterCompat bool `json:"skip_filter_compat"`
	// IncludeDayRows keeps per-day detail in the report (default true).
	IncludeDayRows bool `json:"include_day_rows"`
}

// DefaultOptions is production-safe: framework OFF.
func DefaultOptions() Options {
	return Options{
		Enabled:                DefaultEnabled,
		AttachDecisionShadowV2: false,
		AttachInsight:          false,
		IncludeDayRows:         true,
	}
}

// Input is the Evaluate entry.
type Input struct {
	AsOf    time.Time                 `json:"as_of"`
	Options Options                   `json:"options"`
	Cases   []portfolioreplay.ReplayCase `json:"-"`
	// Days optional alternate entry (already mapped); Cases preferred for batch Replay.
	Days []portfoliovalidation.DayCase `json:"-"`
}

// ScalarStats is a simple distribution over days (not returns).
type ScalarStats struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
	Sum   float64 `json:"sum"`
}

// CountBin is a histogram bucket.
type CountBin struct {
	Value int `json:"value"`
	Count int `json:"count"`
}

// SideDayMetrics is one side (Legacy or Portfolio) for one day.
type SideDayMetrics struct {
	ProviderIdentity string `json:"provider_identity"`

	SelectedCount  int     `json:"selected_count"`
	WaitlistCount  int     `json:"waitlist_count"`
	AllocationCount int    `json:"allocation_count"` // names in allocation set
	BuyNotional    float64 `json:"buy_notional"`

	GrossExposure float64  `json:"gross_exposure"`
	CashRatio     *float64 `json:"cash_ratio,omitempty"`
	Top1Weight    float64  `json:"top1_weight"`
	Top5Weight    float64  `json:"top5_weight"`

	SectorAvailable bool               `json:"sector_available"`
	SectorExposure  []SectorWeightRow `json:"sector_exposure,omitempty"`
	MaxSectorWeight float64            `json:"max_sector_weight,omitempty"`

	TightenApplied bool     `json:"tighten_applied"`
	TightenCount   int      `json:"tighten_count"`
	TightenReasons []string `json:"tighten_reasons,omitempty"`

	BudgetBinding string `json:"budget_binding,omitempty"`
	BudgetCut     bool   `json:"budget_cut"` // binding implies capital/gross/cash cut

	FilterRejectReasons map[string]int `json:"filter_reject_reasons"`

	AllocationReasons map[string]int `json:"allocation_reasons"`
	SelectionReasons  map[string]int `json:"selection_reasons"`
	RiskReasons       map[string]int `json:"risk_reasons"`
}

// SectorWeightRow is one industry bucket.
type SectorWeightRow struct {
	Industry string  `json:"industry"`
	Weight   float64 `json:"weight"`
}

// DayEvaluation is one ReplayCase dual-path summary.
type DayEvaluation struct {
	CaseID    string `json:"case_id"`
	TradeDate string `json:"trade_date"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`

	Legacy    SideDayMetrics `json:"legacy"`
	Portfolio SideDayMetrics `json:"portfolio"`

	DecisionShadowV2Attached bool   `json:"decision_shadow_v2_attached"`
	InsightAttached          bool   `json:"insight_attached"`
	InsightRiskLevel         string `json:"insight_risk_level,omitempty"`
	InsightDisclaimer        string `json:"insight_disclaimer,omitempty"`
}

// DecisionStability aggregates selection / allocation / waitlist / amount distributions.
type DecisionStability struct {
	DailySelectedCountLegacy    []CountBin  `json:"daily_selected_count_legacy"`
	DailySelectedCountPortfolio []CountBin  `json:"daily_selected_count_portfolio"`
	SelectedCountLegacyMean     float64     `json:"selected_count_legacy_mean"`
	SelectedCountPortfolioMean  float64     `json:"selected_count_portfolio_mean"`
	SelectedCountDeltaMean      float64     `json:"selected_count_delta_mean"` // port − legacy

	AllocationCountLegacyMean    float64 `json:"allocation_count_legacy_mean"`
	AllocationCountPortfolioMean float64 `json:"allocation_count_portfolio_mean"`

	AmountDistributionLegacy    ScalarStats `json:"amount_distribution_legacy"`
	AmountDistributionPortfolio ScalarStats `json:"amount_distribution_portfolio"`
	AmountDelta                 ScalarStats `json:"amount_delta"` // port − legacy

	WaitlistLegacyMean    float64     `json:"waitlist_legacy_mean"`
	WaitlistPortfolioMean float64     `json:"waitlist_portfolio_mean"`
	WaitlistDeltaMean     float64     `json:"waitlist_delta_mean"`
	WaitlistDeltaBins     []CountBin  `json:"waitlist_delta_bins"`
}

// RiskBehavior aggregates exposure / cash / concentration / sector.
type RiskBehavior struct {
	GrossExposureLegacyMean    float64     `json:"gross_exposure_legacy_mean"`
	GrossExposurePortfolioMean float64     `json:"gross_exposure_portfolio_mean"`
	GrossExposureDelta         ScalarStats `json:"gross_exposure_delta"`

	CashRatioLegacyMean    float64     `json:"cash_ratio_legacy_mean"`
	CashRatioPortfolioMean float64     `json:"cash_ratio_portfolio_mean"`
	CashRatioDelta         ScalarStats `json:"cash_ratio_delta"`
	CashRatioComparableDays int        `json:"cash_ratio_comparable_days"`

	Top1LegacyMean    float64     `json:"top1_legacy_mean"`
	Top1PortfolioMean float64     `json:"top1_portfolio_mean"`
	Top1Delta         ScalarStats `json:"top1_delta"`

	Top5LegacyMean    float64     `json:"top5_legacy_mean"`
	Top5PortfolioMean float64     `json:"top5_portfolio_mean"`
	Top5Delta         ScalarStats `json:"top5_delta"`

	SectorComparableDays       int         `json:"sector_comparable_days"`
	MaxSectorLegacyMean        float64     `json:"max_sector_legacy_mean,omitempty"`
	MaxSectorPortfolioMean     float64     `json:"max_sector_portfolio_mean,omitempty"`
	MaxSectorDelta             ScalarStats `json:"max_sector_delta,omitempty"`
	SectorUnavailableDayCount  int         `json:"sector_unavailable_day_count"`
}

// ConstraintBehavior aggregates tighten / budget cut / filter rejects.
type ConstraintBehavior struct {
	RiskTightenDaysPortfolio   int            `json:"risk_tighten_days_portfolio"`
	RiskTightenCountPortfolio  int            `json:"risk_tighten_count_portfolio"`
	RiskTightenCountLegacy     int            `json:"risk_tighten_count_legacy"`
	BudgetCutDaysPortfolio     int            `json:"budget_cut_days_portfolio"`
	BudgetCutDaysLegacy        int            `json:"budget_cut_days_legacy"`
	BudgetBindingTotals        map[string]int `json:"budget_binding_totals"`
	FilterRejectReasonsLegacy  map[string]int `json:"filter_reject_reasons_legacy"`
	FilterRejectReasonsPortfolio map[string]int `json:"filter_reject_reasons_portfolio"`
}

// Explainability aggregates reason histograms (decision narrative, not advice).
type Explainability struct {
	AllocationReasonTotals map[string]int `json:"allocation_reason_totals"`
	RiskReasonTotals       map[string]int `json:"risk_reason_totals"`
	SelectionReasonTotals  map[string]int `json:"selection_reason_totals"`
	InsightRiskLevelCounts map[string]int `json:"insight_risk_level_counts,omitempty"`
	Notes                  []string       `json:"notes,omitempty"`
}

// PortfolioHistoricalEvaluationReport is the framework output.
type PortfolioHistoricalEvaluationReport struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	Enabled       bool      `json:"enabled"`
	Skipped       bool      `json:"skipped"`
	SkipReason    string    `json:"skip_reason,omitempty"`

	RecordOnly         bool `json:"record_only"`
	ReadOnly           bool `json:"read_only"`
	NotABacktest       bool `json:"not_a_backtest"`
	NotPnL             bool `json:"not_pnl"`
	NotReturn          bool `json:"not_return"`
	NotSharpe          bool `json:"not_sharpe"`
	NotAutoTune        bool `json:"not_auto_tune"`
	NotATradePlan      bool `json:"not_a_trade_plan"`
	NotExecution       bool `json:"not_execution"`
	NotProductionWrite bool `json:"not_production_write"`

	CaseCount    int `json:"case_count"`
	SuccessCount int `json:"success_count"`
	ErrorCount   int `json:"error_count"`

	DecisionStability DecisionStability  `json:"decision_stability"`
	RiskBehavior      RiskBehavior       `json:"risk_behavior"`
	ConstraintBehavior ConstraintBehavior `json:"constraint_behavior"`
	Explainability    Explainability     `json:"explainability"`

	Days   []DayEvaluation `json:"days,omitempty"`
	Errors []EvalError     `json:"errors,omitempty"`
	Notes  []string        `json:"notes,omitempty"`
}

// EvalError is a per-case failure.
type EvalError struct {
	CaseID string `json:"case_id"`
	Reason string `json:"reason"`
}
