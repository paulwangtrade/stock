package portfolio

import "time"

const (
	dashboardDataSourceNote = "Portfolio Dashboard Read Model · read-only paper_sim_accounts/positions/fills + daily_reports; persisted mark; no quote overlay; no writes"
	dashboardDisclaimer     = "Paper 模拟账户观察视图。不代表真实交易账户。不生成买卖单。"

	RiskLevelUnknown = "UNKNOWN"
	RiskLevelLow     = "LOW"
	RiskLevelMedium  = "MEDIUM"
	RiskLevelHigh    = "HIGH"

	DailyPnLBasisReportDelta = "daily_report_delta"
	DailyPnLBasisUnavailable = "unavailable"
)

// PortfolioDashboardView is the Phase11-B.1 unified read model for a future Dashboard.
type PortfolioDashboardView struct {
	TradeDate      string            `json:"trade_date"`
	AsOf           time.Time         `json:"as_of"`
	Found          bool              `json:"found"`
	Summary        PortfolioSummary  `json:"summary"`
	Positions      []PositionView    `json:"positions"`
	Risk           RiskView          `json:"risk"`
	Trades         TradeHistoryView  `json:"trades"`
	DataSourceNote string            `json:"data_source_note"`
	Disclaimer     string            `json:"disclaimer"`
}

// PortfolioSummary is account-level totals (persisted mark basis).
type PortfolioSummary struct {
	Equity        float64  `json:"equity"`
	Cash          float64  `json:"cash"`
	MarketValue   float64  `json:"market_value"`
	PositionCount int      `json:"position_count"`
	DailyPnL      *float64 `json:"daily_pnl"` // nil when prior settlement snapshot missing
	DailyPnLBasis string   `json:"daily_pnl_basis,omitempty"`
}

// PositionView is one net holding row for Dashboard.
type PositionView struct {
	StockCode     string  `json:"stock_code"`
	StockName     string  `json:"stock_name"`
	Quantity      int64   `json:"quantity"`
	AvgCost       float64 `json:"avg_cost"`
	MarketPrice   float64 `json:"market_price"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

// RiskView is concentration-style risk (observation labels only).
type RiskView struct {
	MaxPositionRatio      float64  `json:"max_position_ratio"`
	IndustryConcentration *float64 `json:"industry_concentration,omitempty"` // nil: not available in paper_sim
	RiskLevel             string   `json:"risk_level"`
}

// TradeHistoryView holds today's fills (read-only).
type TradeHistoryView struct {
	TradeDate string          `json:"trade_date"`
	Fills     []TradeFillView `json:"fills"`
}

// TradeFillView is one fill row for today.
type TradeFillView struct {
	FillID     uint      `json:"fill_id"`
	OrderID    uint      `json:"order_id"`
	PlanID     uint      `json:"plan_id,omitempty"`
	StockCode  string    `json:"stock_code"`
	StockName  string    `json:"stock_name"`
	Side       string    `json:"side"`
	Price      float64   `json:"price"`
	Volume     int64     `json:"volume"`
	Fee        float64   `json:"fee"`
	FillReason string    `json:"fill_reason,omitempty"`
	FilledAt   time.Time `json:"filled_at"`
}
