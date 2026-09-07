// Package portfoliovalidation is the Phase12 Historical Decision Validation Framework.
//
// It validates, day-by-day, what Legacy vs Portfolio decision paths would emit
// given the same historical inputs. It is NOT a backtest: no PnL, no Sharpe,
// no fill simulation, no auto-tuning, no production-chain writes.
//
// Composition: portfolioreplay.ReplayCase facts → portfoliosim (Legacy+Portfolio)
// → optional decisionshadowv2 annex → PortfolioValidationReport.
package portfoliovalidation

import (
	"time"

	"go-stock/backend/decisionshadowv2"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfolioreplay"
	"go-stock/backend/selection"
)

const (
	SchemaVersion = "portfolio_validation.h12-hdv-v1"

	// DefaultEnabled is compile-time OFF: callers must pass Enabled=true to run.
	DefaultEnabled = false

	DefaultLegacyAmount = 100_000.0
)

// Options controls framework behaviour (all observation / read-only).
type Options struct {
	// Enabled must be true to run; default false → skipped report.
	Enabled bool `json:"enabled"`
	// AttachDecisionShadowV2 runs decisionshadowv2 per day for richer sector/cash projection.
	AttachDecisionShadowV2 bool `json:"attach_decision_shadow_v2"`
	// SkipFilterCompat skips read-only PlanFilter compatibility (default runs filter).
	SkipFilterCompat bool `json:"skip_filter_compat"`
}

// DayCase is one historical decision day (frozen inputs). Not a TradePlan.
type DayCase struct {
	CaseID       string    `json:"case_id"`
	TradeDate    string    `json:"trade_date"`
	DecisionTime time.Time `json:"decision_time"`

	Snapshot    *portfoliolayer.PortfolioSnapshot      `json:"-"`
	Candidates  *selection.CandidateSelectionResult    `json:"-"`
	Constraints portfoliolayer.ConstraintSet           `json:"-"`
	Budget      *portfoliolayer.AllocationBudget       `json:"-"`

	LegacyAmountPerName float64           `json:"legacy_amount_per_name,omitempty"`
	IndustryBySymbol    map[string]string `json:"industry_by_symbol,omitempty"`

	// Optional V2 objective / ceiling when AttachDecisionShadowV2.
	Objective decisionshadowv2.PortfolioObjective `json:"objective,omitempty"`
	Ceiling   portfoliolayer.ConstraintSet        `json:"-"`

	Notes string `json:"notes,omitempty"`
}

// FromReplayCase maps an F.11 ReplayCase into a validation DayCase.
func FromReplayCase(c portfolioreplay.ReplayCase) DayCase {
	return DayCase{
		CaseID:       c.CaseID,
		TradeDate:    c.TradeDate,
		DecisionTime: c.DecisionTime,
		Snapshot:     c.Snapshot,
		Candidates:   c.Candidates,
		Constraints:  c.Constraints,
		Budget:       c.Budget,
		Ceiling:      c.Constraints,
		Notes:        c.Notes,
	}
}

// Input is the framework entry.
type Input struct {
	AsOf    time.Time `json:"as_of"`
	Options Options   `json:"options"`
	Days    []DayCase `json:"days"`
}

// AmountLine is one name's suggested buy notional (observation only).
type AmountLine struct {
	Symbol       string  `json:"symbol"`
	TargetAmount float64 `json:"target_amount"`
	Role         string  `json:"role,omitempty"` // selected | waitlist
}

// SectorWeight is one industry bucket weight (post-buy projection when available).
type SectorWeight struct {
	Industry string  `json:"industry"`
	Weight   float64 `json:"weight"`
}

// FilterReject is one potential PlanFilter skip reason (not an order reject event).
type FilterReject struct {
	Symbol   string  `json:"symbol"`
	RiskCode string  `json:"risk_code"`
	Amount   float64 `json:"amount,omitempty"`
}

// SideDecision is Legacy or Portfolio projected decision for one day.
type SideDecision struct {
	ProviderIdentity string         `json:"provider_identity"` // fixed_amount | portfolio_allocation
	NameCount        int            `json:"name_count"`
	Lines            []AmountLine   `json:"lines"`
	BuyNotionalSum   float64        `json:"buy_notional_sum"`
	CashRatio        *float64       `json:"cash_ratio,omitempty"` // projected remaining cash / equity when equity>0
	CashRatioNote    string         `json:"cash_ratio_note,omitempty"`
	SectorExposure   []SectorWeight `json:"sector_exposure,omitempty"`
	SectorAvailable  bool           `json:"sector_available"`
	SectorNote       string         `json:"sector_note,omitempty"`
	TightenApplied   bool           `json:"tighten_applied"`
	TightenNoteCount int            `json:"tighten_note_count"`
	TightenReasons   []string       `json:"tighten_reasons,omitempty"`
	FilterRejects    []FilterReject `json:"filter_rejects"`
	FilterRiskStatus string         `json:"filter_risk_status,omitempty"`
	FilterRan        bool           `json:"filter_ran"`
}

// AmountDiffRow compares per-symbol amounts.
type AmountDiffRow struct {
	Symbol          string  `json:"symbol"`
	LegacyAmount    float64 `json:"legacy_amount"`
	PortfolioAmount float64 `json:"portfolio_amount"`
	Delta           float64 `json:"delta"`
}

// DayDifference is the required comparison block for one day.
type DayDifference struct {
	NameCountLegacy    int             `json:"name_count_legacy"`
	NameCountPortfolio int             `json:"name_count_portfolio"`
	NameCountDelta     int             `json:"name_count_delta"`

	AmountDiffs []AmountDiffRow `json:"amount_diffs"`

	SectorAvailable      bool           `json:"sector_available"`
	SectorUnavailableNote string        `json:"sector_unavailable_note,omitempty"`
	SectorLegacy         []SectorWeight `json:"sector_legacy,omitempty"`
	SectorPortfolio      []SectorWeight `json:"sector_portfolio,omitempty"`

	CashRatioLegacy    *float64 `json:"cash_ratio_legacy,omitempty"`
	CashRatioPortfolio *float64 `json:"cash_ratio_portfolio,omitempty"`
	CashRatioDelta     *float64 `json:"cash_ratio_delta,omitempty"`

	RiskTightenCountLegacy    int `json:"risk_tighten_count_legacy"`
	RiskTightenCountPortfolio int `json:"risk_tighten_count_portfolio"`
	RiskTightenCountDelta     int `json:"risk_tighten_count_delta"`

	FilterRejectReasonsLegacy    map[string]int `json:"filter_reject_reasons_legacy"`
	FilterRejectReasonsPortfolio map[string]int `json:"filter_reject_reasons_portfolio"`
}

// DayValidation is one historical day's dual projection.
type DayValidation struct {
	CaseID    string         `json:"case_id"`
	TradeDate string         `json:"trade_date"`
	OK        bool           `json:"ok"`
	Error     string         `json:"error,omitempty"`

	LegacyDecision    SideDecision   `json:"legacy_decision"`
	PortfolioDecision SideDecision   `json:"portfolio_decision"`
	Difference        DayDifference  `json:"difference"`

	// Optional V2 annex fingerprint (when enabled).
	DecisionShadowV2Attached bool   `json:"decision_shadow_v2_attached"`
	DecisionShadowV2FP       string `json:"decision_shadow_v2_fp,omitempty"`
}

// AggregateSummary rolls up multi-day validation (decision behaviour only).
type AggregateSummary struct {
	DayCount                   int            `json:"day_count"`
	OKCount                    int            `json:"ok_count"`
	NameCountDeltaMean         float64        `json:"name_count_delta_mean"`
	NotionalDeltaMean          float64        `json:"notional_delta_mean"` // portfolio − legacy
	TightenCountPortfolioTotal int            `json:"tighten_count_portfolio_total"`
	FilterRejectReasonTotals   map[string]int `json:"filter_reject_reason_totals"` // portfolio side
	SectorComparableDays       int            `json:"sector_comparable_days"`
}

// PortfolioValidationReport is the framework output.
type PortfolioValidationReport struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	Enabled       bool      `json:"enabled"`
	Skipped       bool      `json:"skipped"`
	SkipReason    string    `json:"skip_reason,omitempty"`

	RecordOnly         bool `json:"record_only"`
	NotABacktest       bool `json:"not_a_backtest"`
	NotPnL             bool `json:"not_pnl"`
	NotSharpe          bool `json:"not_sharpe"`
	NotAutoTune        bool `json:"not_auto_tune"`
	NotATradePlan      bool `json:"not_a_trade_plan"`
	NotExecution       bool `json:"not_execution"`
	NotProductionWrite bool `json:"not_production_write"`
	ReadOnly           bool `json:"read_only"`

	Days     []DayValidation   `json:"days"`
	Summary  AggregateSummary  `json:"summary"`
	Notes    []string          `json:"notes,omitempty"`
}
