// Package portfolioinsight is the Portfolio Insight Layer: read-only explanation
// of risk / holding / shadow / rebalance observations for humans.
// Not trading advice, not auto-trade, never TradePlan / Execution / strategy mutation.
package portfolioinsight

import (
	"time"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/providershadow"
	"go-stock/backend/rebalance"
)

const (
	SchemaVersion = "portfolio_insight.v1"

	RiskLow         = "low"
	RiskModerate    = "moderate"
	RiskElevated    = "elevated"
	RiskHigh        = "high"
	RiskUnavailable = "unavailable"

	DisclaimerKey = "portfolio_insight.disclaimer.v1"
	DisclaimerZH  = "本页为组合风险与决策过程说明，不构成投资建议，不会自动买卖。"
)

// Input assembles optional read-only sources. Any source may be nil/empty.
type Input struct {
	AsOf      time.Time
	TradeDate string
	AccountID string

	Risk      *portfoliorisk.PortfolioRiskSnapshot
	Holdings  []rules.HoldingDecision
	// AllocationShadow optional; prefer FromShadowRecord helper.
	AllocationShadow *AllocationShadowView
	Rebalance        *rebalance.RebalanceSuggestion
}

// AllocationShadowView is a stable Insight DTO over H3.1 ShadowComparisonRecord.
type AllocationShadowView struct {
	Present               bool
	LegacyBuyNotional     *float64
	PortfolioBuyNotional  *float64
	BudgetBinding         string
	TightenApplied        bool
	SuggestHasPatches     bool
	GrossHeadroomBinding  bool
	SingleCapApplied      bool
	BlockedNewEntries     bool
	ShadowFingerprint     string
}

// FromShadowRecord maps a providershadow record into AllocationShadowView (nil-safe).
func FromShadowRecord(rec *providershadow.ShadowComparisonRecord) *AllocationShadowView {
	if rec == nil {
		return nil
	}
	v := &AllocationShadowView{
		Present:           true,
		ShadowFingerprint: rec.Fingerprint,
	}
	if rec.AllocationBudgetSummary != nil {
		b := rec.AllocationBudgetSummary
		v.BudgetBinding = b.Binding
		leg := b.ImpliedLegacyNotional
		v.LegacyBuyNotional = &leg
		port := b.Portfolio.AvailableCapital
		if b.Portfolio.AvailableCapital == 0 && b.CapitalVsImpliedDelta != 0 {
			// prefer capital when present; else leave portfolio from binding path
			port = b.ImpliedLegacyNotional + b.CapitalVsImpliedDelta
		}
		// Portfolio available capital is the binding budget; use as portfolio notional proxy when line sums absent.
		if b.Portfolio.AvailableCapital > 0 {
			port = b.Portfolio.AvailableCapital
		}
		v.PortfolioBuyNotional = &port
		if v.BudgetBinding == "" {
			v.BudgetBinding = b.Portfolio.Binding
		}
	}
	if rec.RiskConstraintTrace != nil {
		v.TightenApplied = rec.RiskConstraintTrace.Applied
		v.SuggestHasPatches = rec.RiskConstraintTrace.SuggestHasPatches
	} else if rec.RiskAdjustment != nil {
		v.TightenApplied = rec.RiskAdjustment.Applied
		v.SuggestHasPatches = rec.RiskAdjustment.SuggestHasPatches
	}
	if rec.RiskCutSummary != nil {
		v.GrossHeadroomBinding = rec.RiskCutSummary.GrossHeadroomBinding
		v.SingleCapApplied = rec.RiskCutSummary.SingleCapApplied
		v.BlockedNewEntries = rec.RiskCutSummary.BlockedNewEntries
		if v.BudgetBinding == "" {
			v.BudgetBinding = rec.RiskCutSummary.PortfolioBinding
		}
	}
	return v
}

// SectorConcentration is user-facing industry concentration (only when sector available).
type SectorConcentration struct {
	SectorName string   `json:"sector_name"`
	Weight     float64  `json:"weight"`
	Cap        *float64 `json:"cap,omitempty"`
	OverCap    bool     `json:"over_cap"`
	Note       string   `json:"note,omitempty"`
}

// MaxPosition is top-1 weight observation.
type MaxPosition struct {
	Symbol string  `json:"symbol,omitempty"`
	Weight float64 `json:"weight"`
	Note   string  `json:"note,omitempty"`
}

// PortfolioSummary is the current-book card.
type PortfolioSummary struct {
	NameCount            *int                 `json:"name_count,omitempty"`
	NameCountNote        string               `json:"name_count_note,omitempty"`
	SectorConcentration  *SectorConcentration `json:"sector_concentration,omitempty"`
	SectorUnavailableNote string              `json:"sector_unavailable_note,omitempty"`
	MaxPosition          *MaxPosition         `json:"max_position,omitempty"`
	MaxPositionNote      string               `json:"max_position_note,omitempty"`
	CashRatio            *float64             `json:"cash_ratio,omitempty"`
	CashRatioNote        string               `json:"cash_ratio_note,omitempty"`
	RiskLevel            string               `json:"risk_level"`
	RiskLevelLabel       string               `json:"risk_level_label"`
	RiskLevelReasons     []string             `json:"risk_level_reasons"`
	NarrativeHints       []string             `json:"narrative_hints"`
}

// WhyReduce explains REDUCE/EXIT observation for one symbol.
type WhyReduce struct {
	Symbol           string   `json:"symbol"`
	Headline         string   `json:"headline"`
	ActionObserved   string   `json:"action_observed"`
	PlainReasons     []string `json:"plain_reasons"`
	EvidenceBullets  []string `json:"evidence_bullets,omitempty"`
	ExecutableNote   string   `json:"executable_note,omitempty"`
	Disclaimer       string   `json:"disclaimer"`
	SourceReasonCodes []string `json:"source_reason_codes,omitempty"`
}

// BuyLimitFactor is one why-buy-limited factor.
type BuyLimitFactor struct {
	Code      string `json:"code"`
	PlainText string `json:"plain_text"`
	Available bool   `json:"available"`
}

// ShadowContrast is optional Legacy vs Portfolio notional note.
type ShadowContrast struct {
	LegacyNotionalSum    *float64 `json:"legacy_notional_sum,omitempty"`
	PortfolioNotionalSum *float64 `json:"portfolio_notional_sum,omitempty"`
	Binding              string   `json:"binding,omitempty"`
	Note                 string   `json:"note"`
}

// WhyBuyLimited explains new-buy observation constraints.
type WhyBuyLimited struct {
	Headline       string           `json:"headline"`
	Factors        []BuyLimitFactor `json:"factors"`
	ShadowContrast *ShadowContrast  `json:"shadow_contrast,omitempty"`
	Disclaimer     string           `json:"disclaimer"`
}

// PortfolioInsight is the user-facing explanation view (alias PortfolioInsightView).
type PortfolioInsight struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	TradeDate     string    `json:"trade_date,omitempty"`
	AccountID     string    `json:"account_id,omitempty"`

	RecordOnly       bool `json:"record_only"`
	NotTradingAdvice bool `json:"not_trading_advice"`
	NotAutoTrade     bool `json:"not_auto_trade"`
	NotATradePlan    bool `json:"not_a_trade_plan"`
	NotExecution     bool `json:"not_execution"`
	NotBuyChainWrite bool `json:"not_buy_chain_write"`
	NotStrategyWrite bool `json:"not_strategy_write"`

	Disclaimer    string `json:"disclaimer"`
	DisclaimerKey string `json:"disclaimer_key"`

	PortfolioSummary PortfolioSummary `json:"portfolio_summary"`
	WhyReduce        []WhyReduce      `json:"why_reduce"`
	WhyBuyLimited    *WhyBuyLimited   `json:"why_buy_limited,omitempty"`

	DataGaps          []string `json:"data_gaps"`
	SourcesFingerprint string  `json:"sources_fingerprint"`
}
