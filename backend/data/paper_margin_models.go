package data

import "time"

const (
	PaperAccountModeCash   = "cash"
	PaperAccountModeMargin = "margin"

	PaperMarginOrderNormalBuy  = "normal_buy"
	PaperMarginOrderNormalSell = "normal_sell"
	PaperMarginOrderMarginBuy  = "margin_buy"
	PaperMarginOrderSellRepay  = "sell_repay"
	PaperMarginOrderShortSell  = "short_sell"
	PaperMarginOrderBuyReturn  = "buy_return"
)

// PaperMarginAccount 是普通模拟账户的两融扩展。阈值仅用于本地模拟，不代表监管或券商真实标准。
type PaperMarginAccount struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	AccountID             uint       `gorm:"uniqueIndex;not null" json:"accountId"`
	Mode                  string     `gorm:"size:16;not null;default:margin" json:"mode"`
	FinanceCreditLimit    float64    `gorm:"not null;default:0" json:"financeCreditLimit"`
	SecuritiesCreditLimit float64    `gorm:"not null;default:0" json:"securitiesCreditLimit"`
	WarningRatio          float64    `gorm:"not null;default:1.5" json:"warningRatio"`
	CloseoutRatio         float64    `gorm:"not null;default:1.3" json:"closeoutRatio"`
	FinanceAnnualRate     float64    `gorm:"not null;default:0.08" json:"financeAnnualRate"`
	SecuritiesAnnualRate  float64    `gorm:"not null;default:0.10" json:"securitiesAnnualRate"`
	LastAccruedAt         *time.Time `json:"lastAccruedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

func (PaperMarginAccount) TableName() string { return "paper_margin_accounts" }

type PaperFinanceLiability struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AccountID       uint      `gorm:"uniqueIndex:idx_paper_finance_debt;not null" json:"accountId"`
	StockCode       string    `gorm:"size:16;uniqueIndex:idx_paper_finance_debt;not null" json:"stockCode"`
	Principal       float64   `gorm:"not null;default:0" json:"principal"`
	AccruedInterest float64   `gorm:"not null;default:0" json:"accruedInterest"`
	Quantity        int64     `gorm:"not null;default:0" json:"quantity"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (PaperFinanceLiability) TableName() string { return "paper_finance_liabilities" }

type PaperSecuritiesLiability struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AccountID  uint      `gorm:"uniqueIndex:idx_paper_securities_debt;not null" json:"accountId"`
	StockCode  string    `gorm:"size:16;uniqueIndex:idx_paper_securities_debt;not null" json:"stockCode"`
	StockName  string    `gorm:"size:64" json:"stockName"`
	Quantity   int64     `gorm:"not null;default:0" json:"quantity"`
	AvgPrice   float64   `gorm:"not null;default:0" json:"avgPrice"`
	AccruedFee float64   `gorm:"not null;default:0" json:"accruedFee"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (PaperSecuritiesLiability) TableName() string { return "paper_securities_liabilities" }

// PaperBorrowPool 是模拟券源及担保品参数，不接入真实券商。
type PaperBorrowPool struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	StockCode             string    `gorm:"size:16;uniqueIndex;not null" json:"stockCode"`
	StockName             string    `gorm:"size:64" json:"stockName"`
	AvailableQuantity     int64     `gorm:"not null;default:0" json:"availableQuantity"`
	CollateralRate        float64   `gorm:"not null;default:0.7" json:"collateralRate"`
	FinanceMarginRatio    float64   `gorm:"not null;default:0.5" json:"financeMarginRatio"`
	SecuritiesMarginRatio float64   `gorm:"not null;default:0.5" json:"securitiesMarginRatio"`
	Enabled               bool      `gorm:"not null;default:true" json:"enabled"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

func (PaperBorrowPool) TableName() string { return "paper_borrow_pools" }

type PaperMarginOrder struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	AccountID  uint       `gorm:"index;not null" json:"accountId"`
	Kind       string     `gorm:"size:24;index;not null" json:"kind"`
	StockCode  string     `gorm:"size:16;index;not null" json:"stockCode"`
	StockName  string     `gorm:"size:64" json:"stockName"`
	Status     string     `gorm:"size:16;index;not null" json:"status"`
	Price      float64    `gorm:"not null" json:"price"`
	Volume     int64      `gorm:"not null" json:"volume"`
	Fee        float64    `gorm:"not null;default:0" json:"fee"`
	Reason     string     `gorm:"type:text" json:"reason"`
	ReasonCode string     `gorm:"size:48" json:"reasonCode"`
	FilledAt   *time.Time `json:"filledAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func (PaperMarginOrder) TableName() string { return "paper_margin_orders" }

type PaperMarginLedger struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AccountID   uint      `gorm:"index;not null" json:"accountId"`
	OrderID     uint      `gorm:"index" json:"orderId"`
	Type        string    `gorm:"size:32;index;not null" json:"type"`
	StockCode   string    `gorm:"size:16;index" json:"stockCode"`
	CashDelta   float64   `gorm:"not null;default:0" json:"cashDelta"`
	DebtDelta   float64   `gorm:"not null;default:0" json:"debtDelta"`
	Quantity    int64     `gorm:"not null;default:0" json:"quantity"`
	Description string    `gorm:"type:text" json:"description"`
	OccurredAt  time.Time `gorm:"index;not null" json:"occurredAt"`
}

func (PaperMarginLedger) TableName() string { return "paper_margin_ledger" }

type PaperMarginRiskEvent struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	AccountID        uint      `gorm:"index;not null" json:"accountId"`
	Level            string    `gorm:"size:16;index;not null" json:"level"`
	ReasonCode       string    `gorm:"size:48;index;not null" json:"reasonCode"`
	Message          string    `gorm:"type:text" json:"message"`
	MaintenanceRatio float64   `json:"maintenanceRatio"`
	Resolved         bool      `gorm:"not null;default:false" json:"resolved"`
	OccurredAt       time.Time `gorm:"index;not null" json:"occurredAt"`
}

func (PaperMarginRiskEvent) TableName() string { return "paper_margin_risk_events" }

func PaperMarginModels() []any {
	return []any{
		&PaperMarginAccount{},
		&PaperFinanceLiability{},
		&PaperSecuritiesLiability{},
		&PaperBorrowPool{},
		&PaperMarginOrder{},
		&PaperMarginLedger{},
		&PaperMarginRiskEvent{},
	}
}
