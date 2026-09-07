package summary

import "time"

// DailyInvestmentSummaryView is the read-only daily digest (no LLM, no trade actions).
type DailyInvestmentSummaryView struct {
	TradeDate          string                 `json:"trade_date"`
	AsOf               time.Time              `json:"as_of"`
	PortfolioSummary   PortfolioSummaryBlock  `json:"portfolio_summary"`
	TradingSummary     TradingSummaryBlock    `json:"trading_summary"`
	RiskSummary        RiskSummaryBlock       `json:"risk_summary"`
	PositionAttention  []PositionAttentionItem `json:"position_attention"`
	TomorrowFocus      []string               `json:"tomorrow_focus"`
	Quality            string                 `json:"quality"` // OK | DEGRADED
	MissingInputs      []string               `json:"missing_inputs,omitempty"`
	DataSourceNote     string                 `json:"data_source_note"`
	Disclaimer         string                 `json:"disclaimer"`
}

// PortfolioSummaryBlock mirrors Dashboard numbers + short narrative.
type PortfolioSummaryBlock struct {
	Equity        float64 `json:"equity"`
	Cash          float64 `json:"cash"`
	MarketValue   float64 `json:"market_value"`
	DailyPnL      *float64 `json:"daily_pnl"`
	PositionCount int     `json:"position_count"`
	Narrative     string  `json:"narrative"`
}

// TradingSummaryBlock mirrors Day Monitor statuses + narrative.
type TradingSummaryBlock struct {
	MaterializeStatus string `json:"materialize_status"`
	ApproveStatus     string `json:"approve_status"`
	FreezeStatus      string `json:"freeze_status"`
	ExecutionStatus   string `json:"execution_status"`
	SettlementStatus  string `json:"settlement_status"`
	Session           string `json:"session,omitempty"`
	PlanID            uint   `json:"plan_id,omitempty"`
	ExecutionReason   string `json:"execution_reason,omitempty"`
	Narrative         string `json:"narrative"`
}

// RiskSummaryBlock aggregates Position Intelligence attention.
type RiskSummaryBlock struct {
	RiskLevel              string `json:"risk_level"`
	AttentionCount         int    `json:"attention_count"`
	HighestAttentionReason string `json:"highest_attention_reason,omitempty"`
	Narrative              string `json:"narrative"`
	Degraded               bool   `json:"degraded,omitempty"`
}

// PositionAttentionItem is one name the user should notice.
type PositionAttentionItem struct {
	StockCode string `json:"stock_code"`
	StockName string `json:"stock_name"`
	Reason    string `json:"reason"`
	Status    string `json:"position_status,omitempty"`
}
