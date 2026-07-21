package data

import (
	"time"
)

// K 线周期常量（库内统一，与东财 klt 解耦）
const (
	KLinePeriod1D  = "1d"
	KLinePeriod1W  = "1w"
	KLinePeriod1Mo = "1M" // 月线；与 1m 分钟区分
	KLinePeriod1m  = "1m"
	KLinePeriod5m  = "5m"
	KLinePeriod15m = "15m"
	KLinePeriod30m = "30m"
	KLinePeriod60m = "60m"

	KLineAdjustNone = "none"
	KLineAdjustQFQ  = "qfq"
	KLineAdjustHFQ  = "hfq"
)

// StockKLineDay 日/周/月 K 线正式表（交易所本地时间存于 bar_time）。
// 唯一键：(market, ts_code, period, adjust_type, bar_time)
type StockKLineDay struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Market         string    `gorm:"size:8;not null;uniqueIndex:uk_kline_day;index:idx_kline_day_symbol" json:"market"`
	TSCode         string    `gorm:"column:ts_code;size:32;not null;uniqueIndex:uk_kline_day;index:idx_kline_day_query" json:"tsCode"`
	Symbol         string    `gorm:"size:32;not null;index:idx_kline_day_symbol" json:"symbol"`
	Period         string    `gorm:"size:8;not null;uniqueIndex:uk_kline_day;index:idx_kline_day_query" json:"period"` // 1d|1w|1M
	AdjustType     string    `gorm:"column:adjust_type;size:8;not null;default:none;uniqueIndex:uk_kline_day;index:idx_kline_day_query" json:"adjustType"`
	BarTime        time.Time `gorm:"column:bar_time;not null;uniqueIndex:uk_kline_day;index:idx_kline_day_query" json:"barTime"`
	Open           float64   `gorm:"not null" json:"open"`
	High           float64   `gorm:"not null" json:"high"`
	Low            float64   `gorm:"not null" json:"low"`
	Close          float64   `gorm:"not null" json:"close"`
	Volume         float64   `gorm:"not null;default:0" json:"volume"`
	Amount         *float64  `json:"amount,omitempty"`
	ChangePct      *float64  `gorm:"column:change_pct" json:"changePct,omitempty"`
	ChangeVal      *float64  `gorm:"column:change_val" json:"changeVal,omitempty"`
	Amplitude      *float64  `json:"amplitude,omitempty"`
	TurnoverRate   *float64  `gorm:"column:turnover_rate" json:"turnoverRate,omitempty"`
	Source         string    `gorm:"size:32" json:"source,omitempty"`
	SourceSecID    string    `gorm:"column:source_secid;size:32" json:"sourceSecId,omitempty"`
	SourceSinaCode string    `gorm:"column:source_sina_code;size:32" json:"sourceSinaCode,omitempty"`
	FetchedAt      *time.Time `gorm:"column:fetched_at" json:"fetchedAt,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (StockKLineDay) TableName() string { return "stock_kline_day" }

// StockKLineMinute 分钟 K 线正式表（1m/5m/15m/30m/60m）。
// 唯一键：(market, ts_code, period, adjust_type, bar_time)
type StockKLineMinute struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Market         string    `gorm:"size:8;not null;uniqueIndex:uk_kline_min;index:idx_kline_min_symbol" json:"market"`
	TSCode         string    `gorm:"column:ts_code;size:32;not null;uniqueIndex:uk_kline_min;index:idx_kline_min_query" json:"tsCode"`
	Symbol         string    `gorm:"size:32;not null;index:idx_kline_min_symbol" json:"symbol"`
	Period         string    `gorm:"size:8;not null;uniqueIndex:uk_kline_min;index:idx_kline_min_query" json:"period"`
	AdjustType     string    `gorm:"column:adjust_type;size:8;not null;default:none;uniqueIndex:uk_kline_min;index:idx_kline_min_query" json:"adjustType"`
	BarTime        time.Time `gorm:"column:bar_time;not null;uniqueIndex:uk_kline_min;index:idx_kline_min_query" json:"barTime"`
	Open           float64   `gorm:"not null" json:"open"`
	High           float64   `gorm:"not null" json:"high"`
	Low            float64   `gorm:"not null" json:"low"`
	Close          float64   `gorm:"not null" json:"close"`
	Volume         float64   `gorm:"not null;default:0" json:"volume"`
	Amount         *float64  `json:"amount,omitempty"`
	ChangePct      *float64  `gorm:"column:change_pct" json:"changePct,omitempty"`
	ChangeVal      *float64  `gorm:"column:change_val" json:"changeVal,omitempty"`
	Amplitude      *float64  `json:"amplitude,omitempty"`
	TurnoverRate   *float64  `gorm:"column:turnover_rate" json:"turnoverRate,omitempty"`
	Source         string    `gorm:"size:32" json:"source,omitempty"`
	SourceSecID    string    `gorm:"column:source_secid;size:32" json:"sourceSecId,omitempty"`
	SourceSinaCode string    `gorm:"column:source_sina_code;size:32" json:"sourceSinaCode,omitempty"`
	FetchedAt      *time.Time `gorm:"column:fetched_at" json:"fetchedAt,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (StockKLineMinute) TableName() string { return "stock_kline_minute" }

// StockKLineSyncState 增量水位（日线/分钟共用）。
type StockKLineSyncState struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Market         string     `gorm:"size:8;not null;uniqueIndex:uk_kline_sync" json:"market"`
	TSCode         string     `gorm:"column:ts_code;size:32;not null;uniqueIndex:uk_kline_sync" json:"tsCode"`
	Period         string     `gorm:"size:8;not null;uniqueIndex:uk_kline_sync" json:"period"`
	AdjustType     string     `gorm:"column:adjust_type;size:8;not null;default:none;uniqueIndex:uk_kline_sync" json:"adjustType"`
	LastBarTime    time.Time  `gorm:"column:last_bar_time;not null" json:"lastBarTime"`
	LastSuccessAt  *time.Time `gorm:"column:last_success_at" json:"lastSuccessAt,omitempty"`
	LastError      string     `gorm:"column:last_error;type:text" json:"lastError,omitempty"`
	Source         string     `gorm:"size:32" json:"source,omitempty"`
	SourceSecID    string     `gorm:"column:source_secid;size:32" json:"sourceSecId,omitempty"`
	SourceSinaCode string     `gorm:"column:source_sina_code;size:32" json:"sourceSinaCode,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (StockKLineSyncState) TableName() string { return "stock_kline_sync_state" }

// MigrateStockKLineTables AutoMigrate 正式 K 线三表。
func MigrateStockKLineTables(database interface{ AutoMigrate(...interface{}) error }) error {
	if database == nil {
		return nil
	}
	return database.AutoMigrate(&StockKLineDay{}, &StockKLineMinute{}, &StockKLineSyncState{})
}

// IsDayKLinePeriod 日/周/月 → stock_kline_day
func IsDayKLinePeriod(period string) bool {
	switch period {
	case KLinePeriod1D, KLinePeriod1W, KLinePeriod1Mo:
		return true
	default:
		return false
	}
}

// IsMinuteKLinePeriod 分钟周期 → stock_kline_minute
func IsMinuteKLinePeriod(period string) bool {
	switch period {
	case KLinePeriod1m, KLinePeriod5m, KLinePeriod15m, KLinePeriod30m, KLinePeriod60m:
		return true
	default:
		return false
	}
}
