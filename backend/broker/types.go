package broker

import "time"

const (
	EventOrderSubmitted  = "order_submitted"
	EventFill            = "fill"
	EventPositionChanged = "position_changed"
	EventAccountChanged  = "account_changed"
)

// TradeOrder 是执行域内统一的委托模型。
type TradeOrder struct {
	ID           string    `json:"id"`
	BrokerID     string    `json:"brokerId"`
	AccountID    string    `json:"accountId"`
	StockCode    string    `json:"stockCode"`
	StockName    string    `json:"stockName"`
	Side         string    `json:"side"`
	Status       string    `json:"status"`
	Price        float64   `json:"price"`
	Volume       int64     `json:"volume"`
	FilledPrice  float64   `json:"filledPrice"`
	FilledVolume int64     `json:"filledVolume"`
	Fee          float64   `json:"fee"`
	Reason       string    `json:"reason"`
	StrategyTag  string    `json:"strategyTag"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	FilledAt     time.Time `json:"filledAt"`
}

// TradeFill 是执行域内统一的成交模型。
type TradeFill struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"orderId"`
	AccountID   string    `json:"accountId"`
	StockCode   string    `json:"stockCode"`
	StockName   string    `json:"stockName"`
	Side        string    `json:"side"`
	Price       float64   `json:"price"`
	Volume      int64     `json:"volume"`
	Fee         float64   `json:"fee"`
	StrategyTag string    `json:"strategyTag"`
	FilledAt    time.Time `json:"filledAt"`
}

// TradePosition 是执行域内统一的持仓快照。
type TradePosition struct {
	AccountID string    `json:"accountId"`
	StockCode string    `json:"stockCode"`
	StockName string    `json:"stockName"`
	Volume    int64     `json:"volume"`
	Sellable  int64     `json:"sellable"`
	AvgCost   float64   `json:"avgCost"`
	MarkPrice float64   `json:"markPrice"`
	MarkedAt  time.Time `json:"markedAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TradeAccount 是执行域内统一的账户快照。
type TradeAccount struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Cash              float64   `json:"cash"`
	InitialCash       float64   `json:"initialCash"`
	Equity            float64   `json:"equity"`
	ValuationStatus   string    `json:"valuationStatus"`
	UnpricedPositions int       `json:"unpricedPositions"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// TradingEvent 是执行域事件信封；不同事件只填充相关载荷。
type TradingEvent struct {
	Sequence   uint64         `json:"sequence"`
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	AccountID  string         `json:"accountId"`
	OrderID    string         `json:"orderId"`
	OccurredAt time.Time      `json:"occurredAt"`
	Order      *TradeOrder    `json:"order,omitempty"`
	Fill       *TradeFill     `json:"fill,omitempty"`
	Position   *TradePosition `json:"position,omitempty"`
	Account    *TradeAccount  `json:"account,omitempty"`
}
