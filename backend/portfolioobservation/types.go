// Package portfolioobservation is the Phase13 unified Portfolio Observation view.
//
// Assembles read-only sources into PortfolioObservationView for GET consumers
// (Dashboard / analysis UI). Never creates TradePlan/Order, never calls Execution,
// never switches DecisionProvider / Controlled Adoption.
package portfolioobservation

import (
	"time"

	"go-stock/backend/portfolioinsight"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/sectorcoverage"
	"go-stock/backend/sellsuggestion"
)

const (
	SchemaVersion = "portfolio_observation.p13-v1"

	DisclaimerKey = "portfolio_observation.disclaimer.v1"
	DisclaimerZH  = "本页为组合观察与决策过程说明，不构成投资建议，不会自动买卖，不会生成或执行 TradePlan，不会切换交易 Provider。"

	dataSourceNote = "portfolioobservation · Assemble only; GET-shaped; not TradePlan; not Execution; not provider switch"
)

// Input injects optional read-only reports. Any source may be nil.
type Input struct {
	AsOf      time.Time
	TradeDate string
	AccountID string

	Risk       *portfoliorisk.PortfolioRiskSnapshot
	Insight    *portfolioinsight.PortfolioInsight
	Validation *portfoliovalidation.PortfolioValidationReport
	Sell       *sellsuggestion.Report
	SectorCov  *sectorcoverage.SectorCoverageReport

	Warnings []string
}

// CurrentPortfolio is the book card (exposure / concentration / sector / cash).
type CurrentPortfolio struct {
	Available bool `json:"available"`

	Exposure      ExposureView      `json:"exposure"`
	Concentration ConcentrationView `json:"concentration"`
	Sector        SectorView        `json:"sector"`
	Cash          CashView          `json:"cash"`

	RiskLevel      string   `json:"risk_level,omitempty"`
	RiskLevelLabel string   `json:"risk_level_label,omitempty"`
	RiskReasons    []string `json:"risk_reasons,omitempty"`
	NameCount      *int     `json:"name_count,omitempty"`
	Note           string   `json:"note,omitempty"`
}

// ExposureView is gross exposure shape.
type ExposureView struct {
	Available     bool     `json:"available"`
	GrossExposure *float64 `json:"gross_exposure,omitempty"`
	GrossNotional *float64 `json:"gross_notional,omitempty"`
	Equity        *float64 `json:"equity,omitempty"`
	HeadroomVsCap *float64 `json:"headroom_vs_cap,omitempty"`
	CapGross      *float64 `json:"cap_gross,omitempty"`
	Note          string   `json:"note,omitempty"`
}

// ConcentrationView is name concentration.
type ConcentrationView struct {
	Available  bool     `json:"available"`
	Top1Weight *float64 `json:"top1_weight,omitempty"`
	Top5Weight *float64 `json:"top5_weight,omitempty"`
	NameCount  int      `json:"name_count,omitempty"`
	CapSingle  *float64 `json:"cap_single,omitempty"`
	Note       string   `json:"note,omitempty"`
}

// SectorBucket is one industry weight.
type SectorBucket struct {
	Sector    string  `json:"sector"`
	Weight    float64 `json:"weight"`
	NameCount int     `json:"name_count,omitempty"`
}

// SectorView is industry exposure (fail-closed when unavailable).
type SectorView struct {
	Available           bool           `json:"available"`
	Buckets             []SectorBucket `json:"buckets,omitempty"`
	MaxSectorWeight     *float64       `json:"max_sector_weight,omitempty"`
	AllowSectorConstraint *bool        `json:"allow_sector_constraint,omitempty"`
	CoverageNote        string         `json:"coverage_note,omitempty"`
	Note                string         `json:"note,omitempty"`
}

// CashView is cash ratio / residual.
type CashView struct {
	Available bool     `json:"available"`
	CashRatio *float64 `json:"cash_ratio,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// ExplainFactor is one plain-language observation factor.
type ExplainFactor struct {
	Code      string `json:"code"`
	PlainText string `json:"plain_text"`
	Available bool   `json:"available"`
	Source    string `json:"source,omitempty"` // insight | validation | sell_suggestion | risk | sector_coverage
}

// ReduceSuggestionRow is one suggest-only sell row for explainability.
type ReduceSuggestionRow struct {
	Symbol         string  `json:"symbol"`
	Action         string  `json:"action"`
	Reason         string  `json:"reason"`
	SuggestSellQty int64   `json:"suggest_sell_qty"`
	TargetWeight   float64 `json:"target_weight"`
	RiskReason     string  `json:"risk_reason,omitempty"`
}

// DecisionExplain is the narrative block.
type DecisionExplain struct {
	// 为什么买少
	WhyBuyLess []ExplainFactor `json:"why_buy_less"`
	// 为什么限制仓位
	WhyPositionLimited []ExplainFactor `json:"why_position_limited"`
	// 为什么建议减少
	WhySuggestReduce []ExplainFactor       `json:"why_suggest_reduce"`
	ReduceRows       []ReduceSuggestionRow `json:"reduce_rows,omitempty"`
}

// ValidationBrief is a compact Legacy vs Portfolio summary (optional).
type ValidationBrief struct {
	Present              bool    `json:"present"`
	Skipped              bool    `json:"skipped,omitempty"`
	DayCount             int     `json:"day_count,omitempty"`
	OKCount              int     `json:"ok_count,omitempty"`
	NameCountDeltaMean   float64 `json:"name_count_delta_mean,omitempty"`
	NotionalDeltaMean    float64 `json:"notional_delta_mean,omitempty"`
	TightenCountPortfolio int    `json:"tighten_count_portfolio,omitempty"`
	SectorComparableDays int     `json:"sector_comparable_days,omitempty"`
	Note                 string  `json:"note,omitempty"`
}

// SectorCoverageBrief summarizes coverage audit.
type SectorCoverageBrief struct {
	Present               bool    `json:"present"`
	HoldingsCoverage      float64 `json:"holdings_coverage,omitempty"`
	PoolCoverage          float64 `json:"pool_coverage,omitempty"`
	UnknownCount          int     `json:"unknown_count,omitempty"`
	AllowSectorConstraint bool    `json:"allow_sector_constraint"`
	MissingCount          int     `json:"missing_count,omitempty"`
	Note                  string  `json:"note,omitempty"`
}

// SourceFlags records which inputs were present.
type SourceFlags struct {
	RiskPresent       bool `json:"risk_present"`
	InsightPresent    bool `json:"insight_present"`
	ValidationPresent bool `json:"validation_present"`
	SellPresent       bool `json:"sell_present"`
	SectorCovPresent  bool `json:"sector_coverage_present"`
}

// PortfolioObservationView is the unified GET payload.
type PortfolioObservationView struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	TradeDate     string    `json:"trade_date,omitempty"`
	AccountID     string    `json:"account_id,omitempty"`

	RecordOnly         bool `json:"record_only"`
	ReadOnly           bool `json:"read_only"`
	NotTradingAdvice   bool `json:"not_trading_advice"`
	NotAutoTrade       bool `json:"not_auto_trade"`
	NotATradePlan      bool `json:"not_a_trade_plan"`
	NotOrder           bool `json:"not_order"`
	NotExecution       bool `json:"not_execution"`
	NotProviderSwitch  bool `json:"not_provider_switch"`
	AnalysisToolOnly   bool `json:"analysis_tool_only"`

	Disclaimer    string `json:"disclaimer"`
	DisclaimerKey string `json:"disclaimer_key"`

	CurrentPortfolio CurrentPortfolio `json:"current_portfolio"`
	DecisionExplain  DecisionExplain  `json:"decision_explain"`
	Validation       ValidationBrief  `json:"validation"`
	SectorCoverage   SectorCoverageBrief `json:"sector_coverage"`

	Sources        SourceFlags `json:"sources"`
	DataGaps       []string    `json:"data_gaps"`
	Warnings       []string    `json:"warnings,omitempty"`
	DataSourceNote string      `json:"data_source_note"`
}
