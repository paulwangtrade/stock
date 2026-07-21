package models

import "time"

const (
	TradePlanStatusDraft      = "draft"
	TradePlanStatusReady      = "ready"
	TradePlanStatusExecuting  = "executing"
	TradePlanStatusDone       = "done"
	TradePlanStatusPartial    = "partial"
	TradePlanStatusSkipped    = "skipped"
	TradePlanStatusFailed     = "failed"
	TradePlanStatusSuperseded = "superseded"

	TradePlanItemPending = "pending"
	TradePlanItemFilled  = "filled"
	TradePlanItemSkipped = "skipped"
	TradePlanItemError   = "error"

	PaperStrategyTagTradePlan = "trade_plan"
)

// TradePlan 某交易日可执行买入计划（同日可有历史版本；执行只认 status=ready）。
type TradePlan struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
	TradeDate          string     `json:"tradeDate" gorm:"size:10;index;not null"`
	PoolID             uint       `json:"poolId" gorm:"index"`
	GeneratedAt        time.Time  `json:"generatedAt"`
	Status             string     `json:"status" gorm:"size:16;index"`
	Side               string     `json:"side" gorm:"size:8"`
	AmountPerStock     float64    `json:"amountPerStock"`
	MaxNames           int        `json:"maxNames"`
	EnableExecute      bool       `json:"enableExecute"`
	Message            string     `json:"message" gorm:"size:500"`
	RiskStatus         string     `json:"riskStatus" gorm:"size:16"`
	MarketLevel        int        `json:"marketLevel"`
	RiskFilteredCount  int        `json:"riskFilteredCount"`
	RiskAcceptedCount  int        `json:"riskAcceptedCount"`
	RiskSummary        string     `json:"riskSummary" gorm:"size:500"`
	RiskSnapshotJSON   string     `json:"riskSnapshotJson" gorm:"type:text"`
	CheckedAt          *time.Time `json:"checkedAt"`
	ExecutedAt         *time.Time `json:"executedAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`

	Items []TradePlanItem `json:"items,omitempty" gorm:"foreignKey:PlanID;references:ID"`
}

func (TradePlan) TableName() string { return "trade_plans" }

// TradePlanItem 计划内单票意图与执行回写（含风控拒绝项 status=skipped）。
type TradePlanItem struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	PlanID          uint      `json:"planId" gorm:"index;not null"`
	TradeDate       string    `json:"tradeDate" gorm:"size:10;index"`
	StockCode       string    `json:"stockCode" gorm:"size:16;index"`
	StockName       string    `json:"stockName" gorm:"size:64"`
	Side            string    `json:"side" gorm:"size:8"`
	Priority        int       `json:"priority"`
	TargetAmount    float64   `json:"targetAmount"`
	TargetVolume    int64     `json:"targetVolume"`
	LimitPrice      float64   `json:"limitPrice"`
	Score           float64   `json:"score"`
	Reason          string    `json:"reason" gorm:"size:255"`
	StrategyName    string    `json:"strategyName" gorm:"size:120"`
	StrategyVersion string    `json:"strategyVersion" gorm:"size:64"`
	Status          string    `json:"status" gorm:"size:16;index"`
	RiskCode        string    `json:"riskCode" gorm:"size:64"`
	RiskMessage     string    `json:"riskMessage" gorm:"size:500"`
	Error           string    `json:"error" gorm:"size:500"`
	OrderID         uint      `json:"orderId" gorm:"index"`
	FillID          uint      `json:"fillId" gorm:"index"`
	FilledPrice     float64   `json:"filledPrice"`
	FilledVolume    int64     `json:"filledVolume"`
	FilledFee       float64   `json:"filledFee"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (TradePlanItem) TableName() string { return "trade_plan_items" }
