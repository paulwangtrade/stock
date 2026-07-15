package models

import "time"

// StockStrategy 本地选股策略（自然语言 / 技术面）
type StockStrategy struct {
	ID           uint       `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	Name         string     `json:"name" gorm:"size:255;not null"`
	QueryType    string     `json:"queryType" gorm:"size:32;not null"` // eastmoney_nl | technical
	QueryText    string     `json:"queryText" gorm:"type:text"`
	QueryJSON    string     `json:"queryJson" gorm:"type:text"`
	Keyword      string     `json:"keyword" gorm:"size:255"`
	Industry     string     `json:"industry" gorm:"size:512"`
	CronExpr     string     `json:"cronExpr" gorm:"size:100"`
	Enable       bool       `json:"enable" gorm:"default:false"`
	PageSize     int        `json:"pageSize" gorm:"default:50"`
	Description  string     `json:"description" gorm:"size:500"`
	LastRunAt    *time.Time `json:"lastRunAt"`
	LastRunCount int        `json:"lastRunCount"`
	LastRunError string     `json:"lastRunError" gorm:"size:500"`
}

func (StockStrategy) TableName() string {
	return "stock_strategies"
}

// StockStrategyRun 策略单次执行记录
type StockStrategyRun struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	CreatedAt  time.Time `json:"createdAt"`
	StrategyID uint      `json:"strategyId" gorm:"index"`
	StockCount int       `json:"stockCount"`
	Message    string    `json:"message" gorm:"size:500"`
	ResultJSON string    `json:"resultJson" gorm:"type:text"`
}

func (StockStrategyRun) TableName() string {
	return "stock_strategy_runs"
}

type StockStrategyQuery struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Name      string `json:"name"`
	QueryType string `json:"queryType"`
}

type StockStrategyPageResp struct {
	Total int             `json:"total"`
	Data  []StockStrategy `json:"data"`
}

type StockStrategyRunQuery struct {
	StrategyID uint `json:"strategyId"`
	Page       int  `json:"page"`
	PageSize   int  `json:"pageSize"`
}

type StockStrategyRunPageResp struct {
	Total int                `json:"total"`
	Data  []StockStrategyRun `json:"data"`
}

// StockStrategyRunView 返回给前端的执行结果
type StockStrategyRunView struct {
	RunID      uint   `json:"runId"`
	StrategyID uint   `json:"strategyId"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
	QueryType  string `json:"queryType"`
	StockCount int    `json:"stockCount"`
	TraceInfo  string `json:"traceInfo,omitempty"`
	Columns    any    `json:"columns,omitempty"`
	DataList   any    `json:"dataList,omitempty"`
	RunAt      string `json:"runAt,omitempty"` // 执行时间（历史回放用）
}
