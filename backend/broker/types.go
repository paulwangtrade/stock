package broker

import "time"

const (
	EventOrderSubmitted  = "order_submitted"
	EventOrderFilled     = "order_filled"
	EventOrderRejected   = "order_rejected"
	EventFill            = "fill" // 兼容旧成交事件；成交时与 order_filled 双发
	EventPositionChanged = "position_changed"
	EventAccountChanged  = "account_changed"
)

// TradeOrder 是执行域内统一的委托模型。
// Phase2-C：Status 为 OMS 态（Paper：pending|filled|rejected|cancelled，无 accepted）；
// 身份字段与 data.PaperOrder 对齐；券商细态只放 BrokerStatus。
type TradeOrder struct {
	ID        string  `json:"id"`
	BrokerID  string  `json:"brokerId"` // 遗留人工适配器字段；与 BrokerOrderID 不同
	AccountID string  `json:"accountId"`
	StockCode string  `json:"stockCode"`
	Symbol    string  `json:"symbol"` // 与 StockCode 同值，统一生命周期 payload
	StockName string  `json:"stockName"`
	Side      string  `json:"side"`
	Status    string  `json:"status"` // OMS status（非 broker 细态）
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	// Real：累计成交均价 (average fill price)；Paper：全成成交价。
	FilledPrice float64 `json:"filledPrice"`
	// Real：累计成交数量 (cumulative filled quantity)；Paper：全成数量。
	FilledVolume int64 `json:"filledVolume"`
	// Real：剩余未成交数量 (= Volume - FilledVolume)；Paper 可忽略。
	LeavesQuantity   int64     `json:"leavesQuantity"`
	Fee              float64   `json:"fee"`
	Reason           string    `json:"reason"`
	StrategyTag      string    `json:"strategyTag"`
	RejectCode       string    `json:"rejectCode"`
	RejectReason     string    `json:"rejectReason"`
	FillAttemptCount int       `json:"fillAttemptCount"`
	ClientOrderID    string    `json:"clientOrderId"`
	ExecBackend      string    `json:"execBackend"`
	BrokerOrderID    string    `json:"brokerOrderId"`   // 券商委托号；Paper 必须空
	ExternalOrderID  string    `json:"externalOrderId"` // 外部单号；Paper 必须空
	BrokerStatus     string    `json:"brokerStatus"`    // Paper=paper；Real=working|partially_filled|filled|...
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	FilledAt         time.Time `json:"filledAt"`
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
