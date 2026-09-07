package portfoliorisk

import "time"

const (
	SchemaVersionH01 = "portfoliorisk.h0-1"

	NoteSectorUnavailable           = "sector_model_unavailable_no_industry"
	NoteSectorIncompleteCoverage    = "sector_model_unavailable_incomplete_coverage"
	NoteThemeNotImplemented         = "theme_model_not_implemented"
	NoteCorrelationUnavailable = "correlation_model_not_implemented"
	NoteFoundFalse             = "snapshot_not_found"
	NoteEquityNonPositive      = "equity_non_positive"

	CapSourceRiskView          = "config_risk_view"
	CapSourceConstraintResolve = "constraint_resolve"
	CapSourceMerged            = "risk_view+constraint_resolve"
	MarketSourceRiskView       = "config_risk_view"
	MarketSourceConstraintRisk = "constraint_risk_layer"
	MarketSourceUnavailable    = "live_unavailable"
	RegimeUnknown              = "unknown"
)

// PortfolioRiskSnapshot is a record-only portfolio risk shape. Not a TradePlan.
type PortfolioRiskSnapshot struct {
	SchemaVersion     string    `json:"schema_version"`
	AsOf              time.Time `json:"as_of"`
	TradeDate         string    `json:"trade_date,omitempty"`
	AccountID         uint      `json:"account_id,omitempty"`
	Found             bool      `json:"found"`
	RecordOnly        bool      `json:"record_only"`
	NotATradePlan     bool      `json:"not_a_trade_plan"`
	NotPlanFilter     bool      `json:"not_plan_filter"`
	NotExecutionRisk  bool      `json:"not_execution_risk"`
	DataSourceNote    string    `json:"data_source_note"`
	InputsFingerprint string    `json:"inputs_fingerprint"`

	Exposure      ExposureBlock      `json:"exposure"`
	Concentration ConcentrationBlock `json:"concentration"`
	Sector        SectorBlock        `json:"sector"`
	Theme         ThemeBlock         `json:"theme"`
	Correlation   CorrelationBlock   `json:"correlation"`
	Market        MarketBlock        `json:"market"`

	// PositionLiquidity is an optional annotation from PositionState (T+1), not a sell signal.
	PositionLiquidity PositionLiquidityNote `json:"position_liquidity,omitempty"`
}

// PositionLiquidityNote summarizes PositionState sellability counts.
type PositionLiquidityNote struct {
	Available         bool `json:"available"`
	SellableNameCount int  `json:"sellable_name_count,omitempty"`
	LockedNameCount   int  `json:"locked_name_count,omitempty"`
	NameCount         int  `json:"name_count,omitempty"`
}

// ExposureBlock is gross / cash shape of the book.
type ExposureBlock struct {
	Available     bool     `json:"available"`
	GrossExposure *float64 `json:"gross_exposure,omitempty"`
	GrossNotional *float64 `json:"gross_notional,omitempty"`
	Equity        *float64 `json:"equity,omitempty"`
	CashRatio     *float64 `json:"cash_ratio,omitempty"`
	HeadroomVsCap *float64 `json:"headroom_vs_cap,omitempty"`
	CapGross      *float64 `json:"cap_gross,omitempty"`
	CapSource     string   `json:"cap_source,omitempty"`
	Note          string   `json:"note,omitempty"`
}

// ConcentrationBlock is name-level concentration on the current book.
type ConcentrationBlock struct {
	Available  bool     `json:"available"`
	Top1Weight *float64 `json:"top1_weight,omitempty"`
	Top5Weight *float64 `json:"top5_weight,omitempty"`
	NameCount  int      `json:"name_count,omitempty"`
	CapSingle  *float64 `json:"cap_single,omitempty"`
	CapSource  string   `json:"cap_source,omitempty"`
	Note       string   `json:"note,omitempty"`
}

// SectorWeight is one industry bucket.
type SectorWeight struct {
	Sector    string  `json:"sector"`
	Weight    float64 `json:"weight"`
	NameCount int     `json:"name_count"`
}

// SectorBlock is industry exposure. Unavailable when Snapshot has no industry data.
type SectorBlock struct {
	Available       bool           `json:"available"`
	SectorExposure  []SectorWeight `json:"sector_exposure"`
	MaxSectorWeight *float64       `json:"max_sector_weight,omitempty"` // omit when unavailable; never fake 0
	Taxonomy        string         `json:"taxonomy,omitempty"`
	Note            string         `json:"note,omitempty"`
}

// ThemeWeight is a placeholder theme bucket.
type ThemeWeight struct {
	Theme     string  `json:"theme"`
	Weight    float64 `json:"weight"`
	NameCount int     `json:"name_count"`
}

// ThemeBlock is unimplemented; available stays false.
type ThemeBlock struct {
	Available     bool          `json:"available"`
	ThemeExposure []ThemeWeight `json:"theme_exposure"`
	Note          string        `json:"note,omitempty"`
}

// CorrelationBlock is unimplemented; CorrelationRisk stays nil (not 0).
type CorrelationBlock struct {
	Available       bool     `json:"available"`
	CorrelationRisk *float64 `json:"correlation_risk"` // null placeholder; never 0-as-pass
	Window          string   `json:"window,omitempty"`
	Note            string   `json:"note,omitempty"`
}

// MarketBlock projects input MarketLevel only — not a live regime classifier.
type MarketBlock struct {
	Available       bool     `json:"available"`
	MarketRegime    string   `json:"market_regime"`
	Source          string   `json:"source,omitempty"`
	BlockNewEntries bool     `json:"block_new_entries,omitempty"`
	MarketLevel     int      `json:"market_level,omitempty"`
	PnLAvailable    bool     `json:"pnl_available"`
	DailyPnLPct     *float64 `json:"daily_pnl_pct,omitempty"`
	Note            string   `json:"note,omitempty"`
}

const dataSourceNote = "PortfolioRiskSnapshot · read-only from Snapshot+RiskView+ConstraintSet+PositionState+optional IndustryBySymbol; not plan-state filter; not Execution; not a TradePlan"
