package models

import "time"

const (
	// ResearchSourceSignalScanSnapshot 研究交易意图来源：信号扫描快照（研究候选池）。
	ResearchSourceSignalScanSnapshot = "signal_scan_snapshot"

	ResearchTradeIntentStatusDraft     = "draft"
	ResearchTradeIntentStatusConfirmed = "confirmed"
	ResearchTradeIntentStatusSubmitted = "submitted"
	ResearchTradeIntentStatusRejected  = "rejected"

	// ResearchTradeStrategyTag 进入 Execution SubmitIntent 的固定 strategyTag。
	ResearchTradeStrategyTag = "research_candidate"
)

// ResearchTradeIntent 研究页人工交易意图（Phase3-PR1）。
// Research-origin execution intent. Not a manual order container.
// 仅持久化确认态与溯源，不负责评分/风控/撮合/下单；不生成 TradePlan。
type ResearchTradeIntent struct {
	ID        uint `json:"id" gorm:"primaryKey"`
	AccountID uint `json:"accountId" gorm:"index"`

	Symbol    string  `json:"symbol" gorm:"size:16;index;not null"` // 如 sh600036
	StockName string  `json:"stockName" gorm:"size:64"`
	Side      string  `json:"side" gorm:"size:8;index"` // buy / sell
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`

	ResearchSource       string  `json:"researchSource" gorm:"size:64;index"`
	CandidateSnapshotID  uint    `json:"candidateSnapshotId" gorm:"index"`
	SignalScore          float64 `json:"signalScore"`
	SignalTag            string  `json:"signalTag" gorm:"size:32"`
	Reason               string  `json:"reason" gorm:"size:500"`

	Status string `json:"status" gorm:"size:16;index;not null"` // draft|confirmed|submitted|rejected

	ClientOrderID string `json:"clientOrderId" gorm:"size:64;index"`
	OrderID       uint   `json:"orderId" gorm:"index"`
	ErrorCode     string `json:"errorCode" gorm:"size:64"`
	ErrorMessage  string `json:"errorMessage" gorm:"size:500"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (ResearchTradeIntent) TableName() string { return "research_trade_intents" }
