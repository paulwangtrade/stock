// Package attention builds the read-only Daily Attention Center (Phase11-J.4).
//
// Normalizes Decision / Intelligence / Daily Summary / Monitor into HOLD|WATCH|REVIEW.
// Never emits BUY/SELL/AUTO_ACTION. Does not write DB or touch trading paths.
package attention

import (
	"time"

	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"
)

const (
	ActionHold   = "HOLD"
	ActionWatch  = "WATCH"
	ActionReview = "REVIEW"

	QualityOK       = "OK"
	QualityDegraded = "DEGRADED"

	TypePortfolio   = "PORTFOLIO"
	TypeRisk        = "RISK"
	TypeEfficiency  = "EFFICIENCY"
	TypeOpportunity = "OPPORTUNITY"
	TypePosition    = "POSITION"
	TypeTrading     = "TRADING"
	TypeTomorrow    = "TOMORROW"

	SourceDecision      = "DECISION"
	SourceIntelligence  = "INTELLIGENCE"
	SourceDailySummary  = "DAILY_SUMMARY"
	SourcePretrade      = "PRETRADE"
	SourceMonitor       = "MONITOR"

	SeverityInfo   = "INFO"
	SeverityLow    = "LOW"
	SeverityMedium = "MEDIUM"
	SeverityHigh   = "HIGH"

	dataSourceNote = "Daily Attention Center J.4 · Decision + Intelligence + Daily Summary + Monitor; Score/Snapshot via Decision; read-only"
	disclaimer     = "每日关注中心（只读）。仅提示 HOLD/WATCH/REVIEW，不生成买卖或自动调仓指令。"
)

// DailyAttention is one normalized attention row.
type DailyAttention struct {
	ID               string `json:"id,omitempty"`
	ItemType         string `json:"item_type"`
	StockCode        string `json:"stock_code,omitempty"`
	StockName        string `json:"stock_name,omitempty"`
	Priority         int    `json:"priority"`
	Title            string `json:"title"`
	Reason           string `json:"reason"`
	Source           string `json:"source"`
	Severity         string `json:"severity"`
	SuggestedAction  string `json:"suggested_action"` // HOLD|WATCH|REVIEW only
	RelatedPlanID    uint   `json:"related_plan_id,omitempty"`
	AsOfTradeDate    string `json:"as_of_trade_date,omitempty"`
}

// DailyAttentionView is the API envelope payload.
type DailyAttentionView struct {
	TradeDate     string           `json:"trade_date"`
	AsOf          time.Time        `json:"as_of"`
	OverallAction string           `json:"overall_action"` // HOLD|WATCH|REVIEW
	Headline      string           `json:"headline,omitempty"`
	Items         []DailyAttention `json:"items"`
	Counts        AttentionCounts  `json:"counts"`
	Quality       string           `json:"quality"`
	MissingInputs []string         `json:"missing_inputs,omitempty"`
	DataSourceNote string          `json:"data_source_note"`
	Disclaimer    string           `json:"disclaimer"`
}

// AttentionCounts summarizes action distribution.
type AttentionCounts struct {
	Review int `json:"review"`
	Watch  int `json:"watch"`
	Hold   int `json:"hold"`
	Total  int `json:"total"`
}

// Inputs for pure Build (tests inject already-built read models).
type Inputs struct {
	TradeDate    string
	AsOf         time.Time
	Decision     *decision.PortfolioDecisionSummary
	Intelligence *intelligence.Bundle
	IntelErr     error
	Daily        *summary.DailyInvestmentSummaryView
	Monitor      *tradingdaymonitor.TradingDayMonitorView
	// DecisionErr / DailyErr mark load failures → DEGRADED.
	DecisionErr error
	DailyErr    error
}
