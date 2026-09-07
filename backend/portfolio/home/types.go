// Package home assembles the read-only Investment Home page (Phase11-I.1 / J.5).
//
// It aggregates Dashboard / Decision / Daily Summary / Day Monitor / Daily Attention.
// It does not recompute trading, risk, or scoring business rules.
package home

import (
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/portfolio/readmodel"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"
)

const (
	QualityOK       = "OK"
	QualityDegraded = "DEGRADED"

	dataSourceNote = "Investment Home P5-D · Portfolio Snapshot (cash/equity/qty) + Daily PnL from dashboard report + Day Monitor (TradePlan); no business recompute"
	disclaimer     = "投资首页（只读聚合）。非投资建议，不生成买卖指令，不修改交易状态。"
)

// InvestmentHomeView is the daily home read model.
type InvestmentHomeView struct {
	TradeDate        string                              `json:"trade_date"`
	AsOf             time.Time                           `json:"as_of"`
	PortfolioSummary HomePortfolioSummary                `json:"portfolio_summary"`
	DecisionSummary  *decision.PortfolioDecisionSummary  `json:"decision_summary"`
	DailySummary     *summary.DailyInvestmentSummaryView `json:"daily_summary"`
	TradingStatus    HomeTradingStatus                   `json:"trading_status"`
	// DailyAttention is the J.4/J.5 primary attention center (preferred).
	DailyAttention *attention.DailyAttentionView `json:"daily_attention"`
	// PositionStates is Phase11-K unified holding state machine (preferred for new/old/lock).
	PositionStates *positionstate.Bundle `json:"position_states,omitempty"`
	// AttentionItems is deprecated (I.1 chip list). Kept for API compat; prefer daily_attention.
	AttentionItems []AttentionItem `json:"attention_items"`
	Quality        string          `json:"quality"`
	MissingInputs  []string        `json:"missing_inputs,omitempty"`
	DataSourceNote string          `json:"data_source_note"`
	Disclaimer     string          `json:"disclaimer"`
}

// HomePortfolioSummary is a thin projection of Dashboard summary (+ optional narrative).
type HomePortfolioSummary struct {
	Found         bool     `json:"found"`
	Equity        float64  `json:"equity"`
	Cash          float64  `json:"cash"`
	MarketValue   float64  `json:"market_value"`
	DailyPnL      *float64 `json:"daily_pnl"`
	PositionCount int      `json:"position_count"`
	Narrative     string   `json:"narrative,omitempty"`
}

// HomeTradingStatus is a thin projection of Day Monitor.
type HomeTradingStatus struct {
	MaterializeStatus string `json:"materialize_status"`
	ApproveStatus     string `json:"approve_status"`
	FreezeStatus      string `json:"freeze_status"`
	ExecutionStatus   string `json:"execution_status"`
	SettlementStatus  string `json:"settlement_status"`
	Session           string `json:"session,omitempty"`
	PlanID            uint   `json:"plan_id,omitempty"`
	ExecutionReason   string `json:"execution_reason,omitempty"`
	Narrative         string `json:"narrative,omitempty"`
}

// AttentionItem is deprecated — use DailyAttention items. Kept for backward compatibility.
type AttentionItem struct {
	Kind      string `json:"kind"` // DECISION|RISK|EFFICIENCY|OPPORTUNITY|TRADING|DAILY
	StockCode string `json:"stock_code,omitempty"`
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	Level     string `json:"level,omitempty"` // HOLD|WATCH|REVIEW|FAIL|INFO
}

// Inputs for pure Assemble (tests inject already-built read models).
type Inputs struct {
	TradeDate string
	AsOf      time.Time
	Dashboard *portfolio.PortfolioDashboardView
	// Snapshot is the P5-D asset source (cash/equity/qty). Nil → fall back to Dashboard numbers (Assemble tests).
	Snapshot *readmodel.View
	Decision *decision.PortfolioDecisionSummary
	Daily    *summary.DailyInvestmentSummaryView
	Monitor  *tradingdaymonitor.TradingDayMonitorView
	// Intelligence feeds Daily Attention Center (optional; missing → DEGRADED attention).
	Intelligence *intelligence.Bundle
	IntelErr     error
	// DailyAttention optional prebuilt view (tests); when nil, Assemble builds via attention.Build.
	DailyAttention *attention.DailyAttentionView
	// PositionStates optional; when nil, Assemble builds via positionstate.Service inputs.
	PositionStates *positionstate.Bundle
	// Optional trading narrative from daily summary trading block (preferred when set).
	TradingNarrative string
}
