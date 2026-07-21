package models

import "time"

const (
	// ManualSourcePaperTradingPanel 手工交易意图来源：模拟盘执行台。
	ManualSourcePaperTradingPanel = "paper_trading_panel"

	ManualTradeOrderKindNormal     = "normal"
	ManualTradeOrderKindMarginBuy  = "margin_buy"  // 预留，本 PR 不执行
	ManualTradeOrderKindMarginSell = "margin_sell" // 预留，本 PR 不执行

	ManualTradeIntentStatusDraft     = "draft"
	ManualTradeIntentStatusConfirmed = "confirmed"
	ManualTradeIntentStatusSubmitted = "submitted"
	ManualTradeIntentStatusRejected  = "rejected"

	// ManualTradeStrategyTag 进入 Execution SubmitIntent 的固定 strategyTag。
	ManualTradeStrategyTag = "manual"
)

// ManualTradeIntent 模拟盘人工交易意图（Phase3-PR4-A）。
// Manual execution intent. Not a research signal container.
// 与 ResearchTradeIntent 隔离：无研究快照/信号字段；不负责撮合/下单。
type ManualTradeIntent struct {
	ID        uint `json:"id" gorm:"primaryKey"`
	AccountID uint `json:"accountId" gorm:"index"`

	Symbol    string  `json:"symbol" gorm:"size:16;index;not null"` // 如 sh600036
	StockName string  `json:"stockName" gorm:"size:64"`
	Side      string  `json:"side" gorm:"size:8;index"` // buy | sell
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	Reason    string  `json:"reason" gorm:"size:500"`

	ManualSource string `json:"manualSource" gorm:"size:64;index"`
	OrderKind    string `json:"orderKind" gorm:"size:32;index"` // normal | margin_buy | margin_sell

	Status string `json:"status" gorm:"size:16;index;not null"` // draft|confirmed|submitted|rejected

	ClientOrderID string `json:"clientOrderId" gorm:"size:64;index"`
	OrderID       uint   `json:"orderId" gorm:"index"`
	ErrorCode     string `json:"errorCode" gorm:"size:64"`
	ErrorMessage  string `json:"errorMessage" gorm:"size:500"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (ManualTradeIntent) TableName() string { return "manual_trade_intents" }
