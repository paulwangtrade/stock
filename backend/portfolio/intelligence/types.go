package intelligence

import (
	"time"

	"go-stock/backend/portfolio/positionstate"
)

// Position status — observation labels only (not trade actions).
const (
	StatusNewPosition   = "NEW_POSITION"
	StatusHoldingProfit = "HOLDING_PROFIT"
	StatusHoldingLoss   = "HOLDING_LOSS"
	StatusWaiting       = "WAITING"
	StatusNeedReview    = "NEED_REVIEW"
	StatusNoPosition    = "NO_POSITION"
)

// Strategy status — best-effort; UNKNOWN when no signal pipeline wired.
const (
	StrategyActiveSignal  = "ACTIVE_SIGNAL"
	StrategyNoSignal      = "NO_SIGNAL"
	StrategySignalPending = "SIGNAL_PENDING"
	StrategyUnknown       = "UNKNOWN"
)

// Risk level for a single name (observation).
const (
	RiskLow     = "LOW"
	RiskMedium  = "MEDIUM"
	RiskHigh    = "HIGH"
	RiskUnknown = "UNKNOWN"
)

// PositionIntelligenceView answers: status / why no action / what to watch (read-only).
type PositionIntelligenceView struct {
	StockCode       string  `json:"stock_code"`
	StockName       string  `json:"stock_name"`
	PositionStatus  string  `json:"position_status"`
	CurrentPosition int64   `json:"current_position"`
	Cost            float64 `json:"cost"`
	MarketPrice     float64 `json:"market_price"`
	PnL             float64 `json:"pnl"`
	RiskLevel       string  `json:"risk_level"`
	StrategyStatus  string  `json:"strategy_status"`
	AttentionReason string  `json:"attention_reason"`
	QuoteSource     string  `json:"quote_source,omitempty"`
	AvailableVolume int64   `json:"available_volume,omitempty"`
	LockedVolume    int64   `json:"locked_volume,omitempty"`
	// PositionState layer (Phase11-K) — single source for new/old/lock.
	PositionState string `json:"position_state,omitempty"`
	IsNewPosition bool   `json:"is_new_position"`
	CanSell       bool   `json:"can_sell"`
	HoldingDays   int    `json:"holding_days,omitempty"`
}

// Bundle is the API envelope payload.
type Bundle struct {
	AsOf           time.Time                  `json:"as_of"`
	Found          bool                       `json:"found"`
	Positions      []PositionIntelligenceView `json:"positions"`
	DataSourceNote string                     `json:"data_source_note"`
	Disclaimer     string                     `json:"disclaimer"`
}

// Re-export lot type for Options wiring.
type LotRecord = positionstate.LotRecord
