package execution

import (
	"context"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/risk"
)

type SubmitRequest struct {
	AccountID uint    `json:"accountId"`
	Kind      string  `json:"kind"`
	StockCode string  `json:"stockCode"`
	StockName string  `json:"stockName"`
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	Reason    string  `json:"reason"`

	// 市场/组合层（可选；0/false 表示不启用对应闸门）
	MarketLevel         int     `json:"marketLevel"`
	BlockNewEntries     bool    `json:"blockNewEntries"`
	MaxExposurePct      float64 `json:"maxExposurePct"`
	MaxSingleNamePct    float64 `json:"maxSingleNamePct"`
	MaxGrossExposurePct float64 `json:"maxGrossExposurePct"`
	MaxDailyLossPct     float64 `json:"maxDailyLossPct"`
	CurrentDailyPnlPct  float64 `json:"currentDailyPnlPct"`
}

type PositionMark struct {
	StockCode string  `json:"stockCode"`
	Price     float64 `json:"price"`
}

type Snapshot struct {
	Account               data.PaperAccount               `json:"account"`
	MarginAccount         data.PaperMarginAccount         `json:"marginAccount"`
	Positions             []data.PaperPosition            `json:"positions"`
	FinanceLiabilities    []data.PaperFinanceLiability    `json:"financeLiabilities"`
	SecuritiesLiabilities []data.PaperSecuritiesLiability `json:"securitiesLiabilities"`
	Orders                []data.PaperMarginOrder         `json:"orders"`
	Ledger                []data.PaperMarginLedger        `json:"ledger"`
	RiskEvents            []data.PaperMarginRiskEvent     `json:"riskEvents"`
	Metrics               risk.Metrics                    `json:"metrics"`
}

type AccrualResult struct {
	AccountID       uint      `json:"accountId"`
	Days            float64   `json:"days"`
	FinanceInterest float64   `json:"financeInterest"`
	SecuritiesFee   float64   `json:"securitiesFee"`
	AccruedAt       time.Time `json:"accruedAt"`
}

type AccountConfig struct {
	AccountID             uint    `json:"accountId"`
	Mode                  string  `json:"mode"`
	FinanceCreditLimit    float64 `json:"financeCreditLimit"`
	SecuritiesCreditLimit float64 `json:"securitiesCreditLimit"`
	WarningRatio          float64 `json:"warningRatio"`
	CloseoutRatio         float64 `json:"closeoutRatio"`
	FinanceAnnualRate     float64 `json:"financeAnnualRate"`
	SecuritiesAnnualRate  float64 `json:"securitiesAnnualRate"`
}

type TransactionService interface {
	Snapshot(ctx context.Context, accountID uint, marks []PositionMark) (*Snapshot, error)
	Submit(ctx context.Context, req SubmitRequest) (*data.PaperMarginOrder, risk.RiskDecision, error)
	AccrueInterest(ctx context.Context, accountID uint, asOf time.Time) (*AccrualResult, error)
	ScanRisk(ctx context.Context, accountID uint, marks []PositionMark, asOf time.Time) ([]data.PaperMarginRiskEvent, error)
}
