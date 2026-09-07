// Package readmodel is the Phase14-P5-B GET-time Portfolio Snapshot envelope.
//
// It projects paper_sim_* + PositionState (+ optional Quote display).
// It does not write DB, create accounts, or touch TradePlan / Gateway / fill / Unlock.
package readmodel

import (
	"time"

	"go-stock/backend/portfolio/positionstate"
)

const (
	dataSourceNote = "Portfolio Read Model · paper_sim_accounts.cash + paper_sim_positions (persisted mark); PositionState nested; Quote overlay is display-only"
	disclaimer     = "Paper 模拟账户只读快照。不代表真实交易账户。不生成买卖单。"

	QuoteSourceLive         = "live"
	QuoteSourceOpenFallback = "open_fallback"
	QuoteSourcePersisted    = "persisted"

	// PriceFreshness is display-only (Phase17.2); never written to ledger.
	PriceFreshnessFresh   = "FRESH"
	PriceFreshnessStale   = "STALE"
	PriceFreshnessUnknown = "UNKNOWN"
)

// defaultQuoteStaleAfter: live quote older than this → STALE (display gate only).
const defaultQuoteStaleAfter = 5 * time.Minute

// View is GET /api/portfolio/snapshot payload (snapshot object).
type View struct {
	Found          bool           `json:"found"`
	Cash           float64        `json:"cash"`
	Equity         float64        `json:"equity"`
	MarketValue    float64        `json:"market_value"`
	Positions      []PositionView `json:"positions"`
	UpdatedAt      time.Time      `json:"updated_at"`
	AsOf           time.Time      `json:"as_of,omitempty"`
	TradeDate      string         `json:"trade_date,omitempty"`
	AccountID      uint           `json:"account_id,omitempty"`
	AccountName    string         `json:"account_name,omitempty"`
	PositionCount  int            `json:"position_count,omitempty"`
	DataSourceNote string         `json:"data_source_note,omitempty"`
	Disclaimer     string         `json:"disclaimer,omitempty"`
}

// PositionView is one net holding. Accounting fields use persisted mark; display_* are optional overlay.
type PositionView struct {
	StockCode          string                          `json:"stock_code"`
	StockName          string                          `json:"stock_name"`
	TotalQty           int64                           `json:"total_qty"`
	AvailableQty       int64                           `json:"available_qty"`
	LockedQty          int64                           `json:"locked_qty"`
	AvgCost            float64                         `json:"avg_cost"`
	MarkPrice          float64                         `json:"mark_price"`
	MarketValue        float64                         `json:"market_value"`
	PnL                float64                         `json:"pnl"`
	PnLPercent         *float64                        `json:"pnl_percent"`
	PositionState      positionstate.PositionStateView `json:"position_state"`
	DisplayPrice       *float64                        `json:"display_price,omitempty"`
	DisplayQuoteSource string                          `json:"display_quote_source,omitempty"`
	DisplayMarketValue *float64                        `json:"display_market_value,omitempty"`
	DisplayPnL         *float64                        `json:"display_pnl,omitempty"`
	DisplayPnLPercent  *float64                        `json:"display_pnl_percent,omitempty"`
	// QuotePreClose is Quote.PreClose when IncludeDisplay fetched a quote (Phase16.27-P0).
	QuotePreClose *float64 `json:"quote_pre_close,omitempty"`
	// TodayPnL = (display_price - quote_pre_close) × total_qty when both prices > 0; else omitted/null.
	// Observation-only; never equals account daily_pnl and must not rewrite mark/equity.
	TodayPnL *float64 `json:"today_pnl,omitempty"`

	// Phase17.2 truth-layer display fields (never mutate mark_price / equity).
	// QuotePrice is set only when a live/open quote exists — nil when overlay falls back to mark.
	QuotePrice *float64 `json:"quote_price,omitempty"`
	// QuoteTimestamp is quote FetchedAt (or parsed upstream Date+Time); display freshness only.
	QuoteTimestamp *time.Time `json:"quote_timestamp,omitempty"`
	// PriceFreshness: FRESH | STALE | UNKNOWN for the quote overlay (not ledger mark).
	PriceFreshness string `json:"price_freshness,omitempty"`
}
