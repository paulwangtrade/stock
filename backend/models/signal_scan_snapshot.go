package models

import "time"

const (
	SignalScanScopeAll    = "all"
	SignalScanSessionMid  = "midday"
	SignalScanSessionClose = "close"
)

// SignalScanSnapshot 全市场信号扫描快照（按交易日 + 时段）
type SignalScanSnapshot struct {
	ID               uint      `json:"id" gorm:"primarykey"`
	CreatedAt        time.Time `json:"createdAt"`
	TradeDate        string    `json:"tradeDate" gorm:"size:10;index;not null"`
	Session          string    `json:"session" gorm:"size:16;index;not null"` // midday | close
	Scope            string    `json:"scope" gorm:"size:32;index;default:all"`
	StrategyID       string    `json:"strategyId" gorm:"size:80;index"`
	StrategyName     string    `json:"strategyName" gorm:"size:120"`
	SignalParamsJSON string    `json:"signalParamsJson" gorm:"type:text"`
	ScannedTotal     int       `json:"scannedTotal"`
	HitTotal         int       `json:"hitTotal"`
	Status           string    `json:"status" gorm:"size:20;default:done"` // running | done | failed
	Message          string    `json:"message" gorm:"size:500"`
	ResultJSON       string    `json:"resultJson" gorm:"type:text"`
	DurationMs       int64     `json:"durationMs"`
}

func (SignalScanSnapshot) TableName() string {
	return "signal_scan_snapshots"
}

type SignalScanSnapshotQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	TradeDate string `json:"tradeDate"`
	Session   string `json:"session"`
	StrategyID string `json:"strategyId"`
}

type SignalScanSnapshotPageResp struct {
	Total int                  `json:"total"`
	Data  []SignalScanSnapshot `json:"data"`
}

// SignalScanHit 单只股票信号结果（与前端列表字段对齐）
type SignalScanHit struct {
	SECUCODE         string `json:"SECUCODE"`
	SECURITY_CODE    string `json:"SECURITY_CODE"`
	SECURITY_NAME_ABBR string `json:"SECURITY_NAME_ABBR"`
	NEW_PRICE        string `json:"NEW_PRICE,omitempty"`
	CHANGE_RATE      string `json:"CHANGE_RATE,omitempty"`
	HIGH_PRICE       string `json:"HIGH_PRICE,omitempty"`
	LOW_PRICE        string `json:"LOW_PRICE,omitempty"`
	PRE_CLOSE_PRICE  string `json:"PRE_CLOSE_PRICE,omitempty"`
	VOLUME           string `json:"VOLUME,omitempty"`
	DEAL_AMOUNT      string `json:"DEAL_AMOUNT,omitempty"`
	TURNOVERRATE     string `json:"TURNOVERRATE,omitempty"`
	VOLUME_RATIO     string `json:"VOLUME_RATIO,omitempty"`
	INDUSTRY         string `json:"INDUSTRY,omitempty"`
	CONCEPT          string `json:"CONCEPT,omitempty"`
	MARKET           string `json:"MARKET,omitempty"`
	Tag              string `json:"tag"`
	DaysAgo          *int   `json:"recentSignalDaysAgo"`
	StatusText       string `json:"statusText"`
	SortRank         int    `json:"sortRank"`
	RSI              float64 `json:"rsi,omitempty"`
}

type SignalScanResultPayload struct {
	Items         []SignalScanHit `json:"items"`
	ScannedTotal  int             `json:"scannedTotal"`
	HitTotal      int             `json:"hitTotal"`
	TradeDate     string          `json:"tradeDate"`
	Session       string          `json:"session"`
	StrategyID    string          `json:"strategyId,omitempty"`
	StrategyName  string          `json:"strategyName,omitempty"`
	CompletedAt   string          `json:"completedAt"`
}
