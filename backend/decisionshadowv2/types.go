// Package decisionshadowv2 is H3.2 Portfolio Decision Shadow V2: a read-only
// simulation comparing Legacy fixed_amount vs Portfolio Objective→Risk→Alloc.
// Default Enabled=false. Never Draft / TradePlan / Execution / provider switch.
package decisionshadowv2

import (
	"time"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"
)

const (
	SchemaVersion = "portfolio_decision_shadow.h3-2-v2"
	DefaultEnabled = false
	DefaultLegacyAmount = 100_000.0
)

// PortfolioObjective is a shadow-only H2 fixture (not production Objective store).
// Preferences may only tighten against RiskCeilingTemplate / ConstraintSet Risk layer.
type PortfolioObjective struct {
	MaxGrossExposurePct *float64 `json:"max_gross_exposure_pct,omitempty"`
	MaxSingleWeight     *float64 `json:"max_single_weight,omitempty"`
	ReserveCashRatio    *float64 `json:"reserve_cash_ratio,omitempty"`
	MaxNewNames         *int     `json:"max_new_names,omitempty"`
	BlockNewEntries     *bool    `json:"block_new_entries,omitempty"`
	SkipAlreadyHolding  *bool    `json:"skip_already_holding,omitempty"`
}

// Options controls projection behaviour (all observation-only).
type Options struct {
	Phase                string `json:"phase"` // shadow_only
	ProjectPostBuyBook   bool   `json:"project_post_buy_book"`
	IncludeWaitlistInDiff bool  `json:"include_waitlist_in_diff"`
}

// Input is the V2 shadow entry. Enabled defaults false when omitted via Runtime.
type Input struct {
	Enabled   bool      `json:"enabled"`
	AsOf      time.Time `json:"as_of"`
	TradeDate string    `json:"trade_date"`
	AccountID string    `json:"account_id,omitempty"`

	CandidatePool []selection.Candidate          `json:"candidate_pool"`
	Snapshot      *portfoliolayer.PortfolioSnapshot `json:"portfolio_snapshot,omitempty"`
	// IndustryBySymbol optional; missing → sector_diff.available=false (no fake zeros).
	IndustryBySymbol map[string]string `json:"industry_by_symbol,omitempty"`

	Objective            PortfolioObjective           `json:"objective"`
	RiskCeilingTemplate  portfoliolayer.ConstraintSet `json:"risk_ceiling_template"`
	RiskView             *tradingconfig.RiskView      `json:"-"`
	Ledger               *portfolio.Snapshot          `json:"-"`
	// PrebuiltRisk optional; when set, skip Build for fingerprint stability in tests.
	PrebuiltRisk *portfoliorisk.PortfolioRiskSnapshot `json:"-"`

	SelectionLimitLegacy    int `json:"selection_limit_legacy"`
	SelectionLimitPortfolio int `json:"selection_limit_portfolio"` // 0 → use resolved MaxNewNames / legacy

	LegacyAmountPerName float64 `json:"legacy_amount_per_name"` // ≤0 → DefaultLegacyAmount
	LegacyAmountSource  string  `json:"legacy_amount_source"`   // constant | sizer_readonly

	AllocationOptions allocationengine.Options `json:"allocation_options"`
	Options           Options                  `json:"options"`
}

// PlanLine is one name in a plan projection (not a TradePlan line).
type PlanLine struct {
	Symbol             string  `json:"symbol"`
	TargetAmount       float64 `json:"target_amount"`
	TargetWeightDelta  float64 `json:"target_weight_delta,omitempty"`
	ImpliedPostWeight  float64 `json:"implied_post_weight,omitempty"`
	Industry           string  `json:"industry,omitempty"`
	AllocationReason   string  `json:"allocation_reason,omitempty"`
	RiskBinding        string  `json:"risk_binding,omitempty"`
	Role               string  `json:"role,omitempty"` // selected | waitlist
}

// PlanTotals aggregates buy side of a projection.
type PlanTotals struct {
	BuyNotionalSum    float64 `json:"buy_notional_sum"`
	NameCount         int     `json:"name_count"`
	ReserveCash       float64 `json:"reserve_cash,omitempty"`
	AvailableCapital  float64 `json:"available_capital,omitempty"`
	BudgetBinding     string  `json:"budget_binding,omitempty"`
}

// SectorExposure is one industry weight in a post-buy book projection.
type SectorExposure struct {
	Industry string  `json:"industry"`
	Weight   float64 `json:"weight"`
}

// BookProjection is an assumed post-buy book (observation only).
type BookProjection struct {
	PostGrossExposure  float64          `json:"post_gross_exposure"`
	PostCashRatio      float64          `json:"post_cash_ratio"`
	PostTop1Weight     float64          `json:"post_top1_weight"`
	PostSectorExposure []SectorExposure `json:"post_sector_exposure,omitempty"`
	SectorAvailable    bool             `json:"sector_available"`
}

// SelectedName is a selection-slot summary.
type SelectedName struct {
	Symbol string `json:"symbol"`
	Role   string `json:"role"`
}

// PlanProjection is legacy or portfolio plan-layer projection (not TradePlan).
type PlanProjection struct {
	ProviderIdentity string         `json:"provider_identity"`
	Selected         []SelectedName `json:"selected"`
	Lines            []PlanLine     `json:"lines"`
	Totals           PlanTotals     `json:"totals"`
	BookProjection   *BookProjection `json:"book_projection,omitempty"`
	OK               bool           `json:"ok"`
	Failure          string         `json:"failure,omitempty"`
}

// SelectionDiff compares who is bought.
type SelectionDiff struct {
	OnlyLegacy    []string `json:"only_legacy"`
	OnlyPortfolio []string `json:"only_portfolio"`
	Both          []string `json:"both"`
}

// AmountDiffRow is per-symbol amount comparison.
type AmountDiffRow struct {
	Symbol           string  `json:"symbol"`
	LegacyAmount     float64 `json:"legacy_amount"`
	PortfolioAmount  float64 `json:"portfolio_amount"`
	Delta            float64 `json:"delta"`
	State            string  `json:"state"` // absent | zero | actual
}

// WeightDiffRow is per-symbol implied post-weight comparison.
type WeightDiffRow struct {
	Symbol          string  `json:"symbol"`
	LegacyWeight    float64 `json:"legacy_weight"`
	PortfolioWeight float64 `json:"portfolio_weight"`
	Delta           float64 `json:"delta"`
}

// SectorDiff compares industry exposure when available.
type SectorDiff struct {
	Available     bool             `json:"available"`
	UnavailableReason string       `json:"unavailable_reason,omitempty"`
	Legacy        []SectorExposure `json:"legacy,omitempty"`
	Portfolio     []SectorExposure `json:"portfolio,omitempty"`
	Delta         []SectorExposure `json:"delta,omitempty"`
	Hotter        []string         `json:"hotter,omitempty"`
	Cooler        []string         `json:"cooler,omitempty"`
}

// CapitalDiff compares total capital usage.
type CapitalDiff struct {
	LegacyBuyNotional    float64 `json:"legacy_buy_notional"`
	PortfolioBuyNotional float64 `json:"portfolio_buy_notional"`
	Delta                float64 `json:"delta"`
	LegacyNameCount      int     `json:"legacy_name_count"`
	PortfolioNameCount   int     `json:"portfolio_name_count"`
}

// RiskImpactDiff explains how tighten changed portfolio buy sizes.
type RiskImpactDiff struct {
	TightenApplied       bool     `json:"tighten_applied"`
	NotionalBeforeTighten float64 `json:"notional_before_tighten"`
	NotionalAfterTighten  float64 `json:"notional_after_tighten"`
	NotionalDelta         float64 `json:"notional_delta"`
	NameCountBefore       int     `json:"name_count_before"`
	NameCountAfter        int     `json:"name_count_after"`
	Notes                 []string `json:"notes,omitempty"`
	Explain               []string `json:"explain,omitempty"`
}

// DiffSummary is a compact difference header.
type DiffSummary struct {
	LegacyBuyNotional     float64  `json:"legacy_buy_notional"`
	PortfolioBuyNotional  float64  `json:"portfolio_buy_notional"`
	NotionalDelta         float64  `json:"notional_delta"`
	NamesAdded            []string `json:"names_added,omitempty"`
	NamesDropped          []string `json:"names_dropped,omitempty"`
	NamesAmountChanged    []string `json:"names_amount_changed,omitempty"`
	MaxSingleWeightDelta  float64  `json:"max_single_weight_delta,omitempty"`
	SectorsHotter         []string `json:"sectors_hotter,omitempty"`
	SectorsCooler         []string `json:"sectors_cooler,omitempty"`
}

// DecisionShadowDifference is the required difference block.
type DecisionShadowDifference struct {
	SelectionDiff SelectionDiff   `json:"selection_diff"`
	AmountDiff    []AmountDiffRow `json:"amount_diff"`
	WeightDiff    []WeightDiffRow `json:"weight_diff"`
	SectorDiff    SectorDiff      `json:"sector_diff"`
	CapitalDiff   CapitalDiff     `json:"capital_diff"`
	RiskImpactDiff RiskImpactDiff `json:"risk_impact_diff"`
	Summary       DiffSummary     `json:"summary"`
}

// ChainTrace records Portfolio-side stages (memory only).
type ChainTrace struct {
	SelectionSummary          string   `json:"selection_summary,omitempty"`
	ObjectiveCompileSummary   string   `json:"objective_compile_summary,omitempty"`
	ResolvedBaseSummary       string   `json:"resolved_base_summary,omitempty"`
	RiskSnapshotAvailable     bool     `json:"risk_snapshot_available"`
	RiskFound                 bool     `json:"risk_found"`
	TightenApplied            bool     `json:"tighten_applied"`
	TightenNotes              []string `json:"tighten_notes,omitempty"`
	ResolvedEffectiveSummary  string   `json:"resolved_effective_summary,omitempty"`
	AllocationBudgetSummary   string   `json:"allocation_budget_summary,omitempty"`
	AllocationMethod          string   `json:"allocation_method,omitempty"`
	AllocationChangeReasons   []string `json:"allocation_change_reasons,omitempty"`
}

// Failure is a non-fatal single-side failure.
type Failure struct {
	Side    string `json:"side"` // legacy | portfolio | system
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Report is PortfolioShadowV2Report / PortfolioDecisionShadowV2Report.
type Report struct {
	SchemaVersion string `json:"schema_version"`
	Enabled       bool   `json:"enabled"`
	Skipped       bool   `json:"skipped"`
	SkipReason    string `json:"skip_reason,omitempty"`
	AsOf          time.Time `json:"as_of"`
	TradeDate     string `json:"trade_date"`
	AccountID     string `json:"account_id,omitempty"`
	InputsFingerprint string `json:"inputs_fingerprint"`

	RecordOnly         bool `json:"record_only"`
	NotADraft          bool `json:"not_a_draft"`
	NotATradePlan      bool `json:"not_a_trade_plan"`
	NotExecution       bool `json:"not_execution"`
	NotProviderSwitch  bool `json:"not_a_provider_switch"`
	NotBuyChainWrite   bool `json:"not_buy_chain_write"`

	LegacyPlanProjection    PlanProjection           `json:"legacy_projection"`
	PortfolioPlanProjection PlanProjection           `json:"portfolio_projection"`
	Difference              DecisionShadowDifference `json:"difference"`

	ChainTrace ChainTrace `json:"chain_trace"`
	Failures   []Failure  `json:"failures,omitempty"`
	Notes      []string   `json:"notes,omitempty"`
}
