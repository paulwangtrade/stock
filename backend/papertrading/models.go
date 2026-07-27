package papertrading

import (
	"fmt"
	"time"

	"go-stock/backend/db"

	"gorm.io/gorm"
)

// Order status (minimal state machine; no continuous matching).
const (
	OrderStatusCreated   = "created"
	OrderStatusSubmitted = "submitted"
	OrderStatusFilled    = "filled"
	OrderStatusRejected  = "rejected"
)

// Reject reasons (fail-closed).
const (
	RejectLimitUpUnavailable   = "limit_up_unavailable"
	RejectLimitDownUnavailable = "limit_down_unavailable"
	RejectMissingOpenPrice     = "missing_open_price"
	RejectInvalidQuantity      = "invalid_quantity"
	RejectInsufficientCash     = "insufficient_cash"
)

// Fill reasons.
const (
	FillReasonMarketOpen = "market_open"
)

// PaperSimAccount is the isolated MVP paper account (distinct from production paper_accounts).
type PaperSimAccount struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"size:64;default:paper_sim_default" json:"name"`
	InitialCash   float64   `json:"initialCash"`
	Cash          float64   `json:"cash"`
	MarketValue   float64   `json:"marketValue"`
	Equity        float64   `json:"equity"`
	RealizedPnl   float64   `json:"realizedPnl"`
	UnrealizedPnl float64   `json:"unrealizedPnl"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (PaperSimAccount) TableName() string { return "paper_sim_accounts" }

// PaperSimPosition holds T+1 aware holdings.
type PaperSimPosition struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AccountID       uint      `gorm:"index;uniqueIndex:idx_paper_sim_pos" json:"accountId"`
	StockCode       string    `gorm:"size:16;uniqueIndex:idx_paper_sim_pos" json:"stockCode"`
	StockName       string    `gorm:"size:64" json:"stockName"`
	TotalVolume     int64     `json:"totalVolume"`
	AvailableVolume int64     `json:"availableVolume"`
	LockedVolume    int64     `json:"lockedVolume"`
	AvgCost         float64   `json:"avgCost"`
	MarkPrice       float64   `json:"markPrice"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (PaperSimPosition) TableName() string { return "paper_sim_positions" }

// PaperSimOrder is a simulated order derived from a Frozen Trade Plan item.
// Idempotency: UNIQUE(plan_id, plan_item_id, trade_date) — one order per plan item per day.
type PaperSimOrder struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AccountID    uint      `gorm:"index" json:"accountId"`
	PlanID       uint      `gorm:"uniqueIndex:idx_paper_sim_order_idem;not null" json:"planId"`
	PlanItemID   uint      `gorm:"uniqueIndex:idx_paper_sim_order_idem;not null" json:"planItemId"`
	TradeDate    string    `gorm:"size:10;uniqueIndex:idx_paper_sim_order_idem;index" json:"tradeDate"`
	StockCode    string    `gorm:"size:16;index" json:"stockCode"`
	StockName    string    `gorm:"size:64" json:"stockName"`
	Side         string    `gorm:"size:8;index" json:"side"`
	Quantity     int64     `json:"quantity"`
	OrderPrice   float64   `json:"orderPrice"` // planned/limit price (reference only; never used as fill)
	Status       string    `gorm:"size:16;index" json:"status"`
	FilledPrice  float64   `json:"filledPrice"`
	FilledVolume int64     `json:"filledVolume"`
	Fee          float64   `json:"fee"`
	RejectReason string    `gorm:"size:64" json:"rejectReason"`
	OrderTime    time.Time `json:"orderTime"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (PaperSimOrder) TableName() string { return "paper_sim_orders" }

// Run status values for paper_sim_runs ledger.
const (
	RunStatusRunning               = "running"
	RunStatusCompleted             = "completed"
	RunStatusCompletedWithRejects  = "completed_with_rejects"
	RunStatusFailed                = "failed"
	RunStatusSkippedDisabled       = "skipped_disabled"
	RunStatusSkippedNonTradingDay  = "skipped_non_trading_day"
	RunStatusSkippedNoFrozenPlan   = "skipped_no_frozen_plan"
	RunStatusSkippedAlreadyRun     = "skipped_already_run"
)

// Trigger values.
const (
	TriggerCron   = "cron"
	TriggerManual = "manual"
)

// PaperSimRun is the per-(trade_date, plan_id) execution ledger for observability + soft idempotency.
type PaperSimRun struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	ExecutionID   string     `gorm:"size:64;uniqueIndex" json:"executionId"`
	TradeDate     string     `gorm:"size:10;index" json:"tradeDate"`
	PlanID        uint       `gorm:"index" json:"planId"`
	Trigger       string     `gorm:"size:16" json:"trigger"`
	Actor         string     `gorm:"size:64" json:"actor"`
	Status        string     `gorm:"size:32;index" json:"status"`
	Message       string     `gorm:"size:500" json:"message"`
	OrdersTotal   int        `json:"ordersTotal"`
	FilledCount   int        `json:"filledCount"`
	RejectCount   int        `json:"rejectCount"`
	SkippedAlready int       `json:"skippedAlready"`
	AccountID     uint       `json:"accountId"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

func (PaperSimRun) TableName() string { return "paper_sim_runs" }

// PaperSimFill records a simulated fill (1:1 with a fully-filled order in MVP).
type PaperSimFill struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AccountID  uint      `gorm:"index" json:"accountId"`
	OrderID    uint      `gorm:"index" json:"orderId"`
	PlanID     uint      `gorm:"index" json:"planId"`
	PlanItemID uint      `gorm:"index" json:"planItemId"`
	StockCode  string    `gorm:"size:16;index" json:"stockCode"`
	StockName  string    `gorm:"size:64" json:"stockName"`
	Side       string    `gorm:"size:8" json:"side"`
	Price      float64   `json:"price"`
	Volume     int64     `json:"volume"`
	Fee        float64   `json:"fee"`
	FillReason string    `gorm:"size:32" json:"fillReason"`
	FilledAt   time.Time `gorm:"index" json:"filledAt"`
}

func (PaperSimFill) TableName() string { return "paper_sim_fills" }

// PaperSimDailyReport is an immutable end-of-day account snapshot (UNIQUE account_id + report_date).
type PaperSimDailyReport struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	AccountID             uint      `gorm:"uniqueIndex:idx_paper_sim_daily_rpt;not null" json:"accountId"`
	ReportDate            string    `gorm:"size:10;uniqueIndex:idx_paper_sim_daily_rpt;index;not null" json:"reportDate"`
	Cash                  float64   `json:"cash"`
	MarketValue           float64   `json:"marketValue"`
	Equity                float64   `json:"equity"`
	FloatingPnl           float64   `json:"floatingPnl"`
	PlanCount             int       `json:"planCount"`
	OrderCount            int       `json:"orderCount"`
	FilledCount           int       `json:"filledCount"`
	RejectedCount         int       `json:"rejectedCount"`
	Turnover              float64   `json:"turnover"`
	MaxSinglePositionPct  float64   `json:"maxSinglePositionPct"`
	MaxGrossExposurePct   float64   `json:"maxGrossExposurePct"`
	MaxSingleStockCode    string    `gorm:"size:16" json:"maxSingleStockCode"`
	PositionCount         int       `json:"positionCount"`
	PlanID                uint      `json:"planId"`
	RunStatus             string    `gorm:"size:32" json:"runStatus"`
	Source                string    `gorm:"size:32" json:"source"`
	CreatedAt             time.Time `json:"createdAt"`
}

func (PaperSimDailyReport) TableName() string { return "paper_sim_daily_reports" }

// PaperSimDailyPosition is an immutable end-of-day position snapshot.
type PaperSimDailyPosition struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AccountID       uint      `gorm:"uniqueIndex:idx_paper_sim_daily_pos;not null" json:"accountId"`
	ReportDate      string    `gorm:"size:10;uniqueIndex:idx_paper_sim_daily_pos;index;not null" json:"reportDate"`
	Symbol          string    `gorm:"size:16;uniqueIndex:idx_paper_sim_daily_pos;not null" json:"symbol"`
	Volume          int64     `json:"volume"`
	AvailableVolume int64     `json:"availableVolume"`
	CostPrice       float64   `json:"costPrice"`
	ClosePrice      float64   `json:"closePrice"`
	MarketValue     float64   `json:"marketValue"`
	FloatingPnl     float64   `json:"floatingPnl"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (PaperSimDailyPosition) TableName() string { return "paper_sim_daily_positions" }

// EnsureSchema creates the isolated paper_sim_* tables. It is additive: it never
// touches trade_plans / trade_plan_items / production paper_* tables.
func EnsureSchema(gdb *gorm.DB) error {
	if gdb == nil {
		gdb = db.Dao
	}
	if gdb == nil {
		return fmt.Errorf("papertrading: db not initialized")
	}
	return gdb.AutoMigrate(
		&PaperSimAccount{},
		&PaperSimPosition{},
		&PaperSimOrder{},
		&PaperSimFill{},
		&PaperSimRun{},
		&PaperSimDailyReport{},
		&PaperSimDailyPosition{},
	)
}
