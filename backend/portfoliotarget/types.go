// Package portfoliotarget is the Phase12 Portfolio Target Model.
//
// CompileTargetPortfolio maps UserRiskProfile + StrategyGoal + PortfolioObjective
// (+ SectorClassification) into a TargetPortfolio intent shape.
//
// Read-only: not a TradePlan, not Execution, not Allocation write-chain,
// not auto-rebalance. Downstream consumers: Rebalance / SellAllocation / Insight.
package portfoliotarget

import "time"

const (
	SchemaVersion = "portfolio_target.v1"

	SourceObjectiveCompile     = "objective_compile"
	SourceAllocationProjection = "allocation_projection"
	SourceFixture              = "fixture"
	SourceResearch             = "research"
	SourceManual               = "manual"

	CapitalModeWeight   = "weight"
	CapitalModeNotional = "notional"

	RiskTierConservative = "conservative"
	RiskTierModerate     = "moderate"
	RiskTierAggressive   = "aggressive"
	RiskTierUnavailable  = "unavailable"

	MaterializeEqualWeight = "equal_weight"

	weightSumEpsilon = 1e-6
)

// UserRiskProfile is the user risk tier / ceiling template (intent + hard caps).
type UserRiskProfile struct {
	RiskTier              string   `json:"risk_tier"` // conservative|moderate|aggressive
	MaxGrossExposurePct   *float64 `json:"max_gross_exposure_pct,omitempty"`
	MaxSingleWeight       *float64 `json:"max_single_weight,omitempty"`
	TargetCashRatio       *float64 `json:"target_cash_ratio,omitempty"`
	BlockNewEntries       *bool    `json:"block_new_entries,omitempty"`
	MaxSectorWeight       *float64 `json:"max_sector_weight,omitempty"`
}

// StrategyGoal is strategy breadth / diversification intent.
type StrategyGoal struct {
	MinNames           *int                `json:"min_names,omitempty"`
	MaxNames           *int                `json:"max_names,omitempty"`
	Style              string              `json:"style,omitempty"`
	SectorTargetWeights map[string]float64 `json:"sector_target_weights,omitempty"` // intent only
	PreferDiversify    bool                `json:"prefer_diversify,omitempty"`
}

// PortfolioObjective merges user preference / objective knobs (H2 fixture shape).
// Must not widen Risk ceilings from UserRiskProfile.
type PortfolioObjective struct {
	TargetGrossExposurePct *float64 `json:"target_gross_exposure_pct,omitempty"`
	TargetCashRatio        *float64 `json:"target_cash_ratio,omitempty"`
	MaxSingleWeight        *float64 `json:"max_single_weight,omitempty"`
	MaxNewNames            *int     `json:"max_new_names,omitempty"`
	MinNames               *int     `json:"min_names,omitempty"`
	BlockNewEntries        *bool    `json:"block_new_entries,omitempty"`
	MaxSectorWeight        *float64 `json:"max_sector_weight,omitempty"`
}

// SectorClassification is the industry map + coverage gate for Target sector block.
// Prefer feeding from sectorprovider CoverageReport (AllowSectorConstraint).
type SectorClassification struct {
	// BySymbol maps norm(symbol) → sector label. Missing codes omit — never "unknown".
	BySymbol map[string]string `json:"by_symbol,omitempty"`
	// Available must be true to emit sector targets / position sector fields.
	// Incomplete coverage → false (fail-closed; no fake 0).
	Available bool   `json:"available"`
	Taxonomy  string `json:"taxonomy,omitempty"`
	Note      string `json:"note,omitempty"`
	// MaxSectorWeight intent; still clamped by risk profile / objective ceilings.
	MaxSectorWeight *float64 `json:"max_sector_weight,omitempty"`
}

// CompileInput is the CompileTargetPortfolio entry.
type CompileInput struct {
	AsOf      time.Time `json:"as_of"`
	TradeDate string    `json:"trade_date,omitempty"`
	AccountID string    `json:"account_id,omitempty"`

	UserRisk   UserRiskProfile      `json:"user_risk"`
	Strategy   StrategyGoal         `json:"strategy_goal"`
	Objective  PortfolioObjective   `json:"portfolio_objective"`
	Sector     SectorClassification `json:"sector_classification"`

	// Symbols to materialize as TargetPosition (candidates / keep list). Order = priority.
	Symbols []string `json:"symbols,omitempty"`
	// EquityRef for target_amount = weight × equity (weight mode). Optional.
	EquityRef float64 `json:"equity_ref,omitempty"`

	Options CompileOptions `json:"options"`
}

// CompileOptions controls materialization (still record-only).
type CompileOptions struct {
	// MaterializeMethod empty → equal_weight.
	MaterializeMethod string `json:"materialize_method,omitempty"`
	// ScaleToFit scales name weights down if Σ weights + cash > 1.
	ScaleToFit bool `json:"scale_to_fit"`
	Source     string `json:"source,omitempty"` // default objective_compile
}

// TargetPosition is one materialized name intent.
type TargetPosition struct {
	Symbol             string  `json:"symbol,omitempty"`
	TargetWeight       float64 `json:"target_weight"`
	TargetAmount       float64 `json:"target_amount,omitempty"`
	TargetSectorWeight float64 `json:"target_sector_weight,omitempty"`
	SectorName         string  `json:"sector_name,omitempty"`
	Priority           int     `json:"priority,omitempty"`
	Reason             string  `json:"reason,omitempty"`
	Source             string  `json:"source,omitempty"`
}

// SectorWeightTarget is one industry target weight.
type SectorWeightTarget struct {
	SectorName   string  `json:"sector_name"`
	TargetWeight float64 `json:"target_weight"`
}

// TargetPortfolio is the standard target-book contract.
type TargetPortfolio struct {
	SchemaVersion string    `json:"schema_version"`
	AsOfIntent    time.Time `json:"as_of_intent"`
	TradeDate     string    `json:"trade_date,omitempty"`
	AccountID     string    `json:"account_id,omitempty"`
	Fingerprint   string    `json:"fingerprint"`
	Source        string    `json:"source"`

	// Convenience fields (user-facing / Insight / Rebalance).
	TargetCashRatio     float64              `json:"target_cash_ratio"`
	TargetGrossExposure float64              `json:"target_gross_exposure"`
	TargetPositions     []TargetPosition     `json:"target_positions"`
	TargetSectorWeights []SectorWeightTarget `json:"target_sector_weights,omitempty"`
	MaxSingleWeight     float64              `json:"max_single_weight"`

	// Structured blocks (design §3).
	Capital   CapitalTarget   `json:"capital"`
	Breadth   BreadthTarget   `json:"breadth"`
	NamePolicy NamePolicy     `json:"name_policy"`
	Sector    SectorTarget    `json:"sector"`
	RiskLevel RiskLevelTarget `json:"risk_level"`

	MaterializeMethod string   `json:"materialize_method,omitempty"`
	UnresolvedNotes   []string `json:"unresolved_notes,omitempty"`

	RecordOnly       bool `json:"record_only"`
	NotATradePlan    bool `json:"not_a_trade_plan"`
	NotExecution     bool `json:"not_execution"`
	NotAutoRebalance bool `json:"not_auto_rebalance"`
	NotAllocWrite    bool `json:"not_allocation_write_chain"`
}

// CapitalTarget is total-book capital intent.
type CapitalTarget struct {
	EquityExposureTarget float64 `json:"equity_exposure_target"`
	CashTargetWeight     float64 `json:"cash_target_weight"`
	CashTargetNotional   float64 `json:"cash_target_notional,omitempty"`
	Mode                 string  `json:"mode"`
}

// BreadthTarget is name-count intent.
type BreadthTarget struct {
	MinNames int    `json:"min_names"`
	MaxNames int    `json:"max_names"`
	Note     string `json:"note,omitempty"`
}

// NamePolicy is single-name weight policy.
type NamePolicy struct {
	DefaultMaxWeight float64 `json:"default_max_weight"`
	DefaultMinWeight float64 `json:"default_min_weight,omitempty"`
}

// SectorTarget is industry intent (fail-closed when Available=false).
type SectorTarget struct {
	Available       bool                 `json:"available"`
	Taxonomy        string               `json:"taxonomy,omitempty"`
	MaxSectorWeight *float64             `json:"max_sector_weight,omitempty"` // omit when unavailable; never fake 0
	Targets         []SectorWeightTarget `json:"targets,omitempty"`
	CoverageNote    string               `json:"coverage_note,omitempty"`
}

// RiskLevelTarget is explanatory risk tier + clamped gross intent.
type RiskLevelTarget struct {
	TargetTier          string   `json:"target_tier"`
	MaxGrossExposurePct *float64 `json:"max_gross_exposure_pct,omitempty"`
	BlockNewEntries     *bool    `json:"block_new_entries,omitempty"`
	Source              string   `json:"source,omitempty"`
}
