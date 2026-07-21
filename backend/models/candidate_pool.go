package models

import "time"

const (
	CandidatePoolStatusReady  = "ready"
	CandidatePoolStatusFailed = "failed"

	CandidatePoolSourceStrategyRun = "strategy_run"
	CandidatePoolSourceFollow      = "follow"
	CandidatePoolSourceMixed       = "mixed"
)

// CandidatePool 某交易日候选股票池快照（允许多版本，历史优先）。
type CandidatePool struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	TradeDate   string    `json:"tradeDate" gorm:"size:10;index;not null"`
	GeneratedAt time.Time `json:"generatedAt"`
	Source      string    `json:"source" gorm:"size:32"`
	SourceRef   string    `json:"sourceRef" gorm:"size:128"`
	Status      string    `json:"status" gorm:"size:16;index"`
	ItemCount   int       `json:"itemCount"`
	Message     string    `json:"message" gorm:"size:500"`
	ConfigJSON  string    `json:"configJson" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	Items []CandidatePoolItem `json:"items,omitempty" gorm:"foreignKey:PoolID;references:ID"`
}

func (CandidatePool) TableName() string { return "candidate_pools" }

// CandidatePoolItem 候选池内单票。
//
// Phase3-A Decision Binding：DecisionID 为可选 QuantDecision.id 引用（可追溯）。
// 空字符串 = 未绑定。不参与 Score/Rank/选股；不作为 TradePlan/Execution 输入；
// 不接 Go producer switch。绑定应在 Rank 冻结后的旁路 enrich 完成。
type CandidatePoolItem struct {
	ID               uint    `json:"id" gorm:"primaryKey"`
	PoolID           uint    `json:"poolId" gorm:"index;not null;uniqueIndex:uidx_pool_code"`
	TradeDate        string  `json:"tradeDate" gorm:"size:10;index"`
	StockCode        string  `json:"stockCode" gorm:"size:16;index;uniqueIndex:uidx_pool_code"`
	StockName        string  `json:"stockName" gorm:"size:64"`
	Rank             int     `json:"rank"`
	Score            float64 `json:"score"` // 最终综合评分
	Reason           string  `json:"reason" gorm:"size:255"`
	StrategyName     string  `json:"strategyName" gorm:"size:120"`
	StrategyVersion  string  `json:"strategyVersion" gorm:"size:64"`
	Industry         string  `json:"industry" gorm:"size:64"`
	TagsJSON         string  `json:"tagsJson" gorm:"size:255"`
	SignalTag        string  `json:"signalTag" gorm:"size:32"`
	SignalScore      float64 `json:"signalScore"` // 信号贡献分（非最终分）
	SignalSnapshotID uint    `json:"signalSnapshotId" gorm:"index"`
	// DecisionID 可选：指向 QuantDecision.id。不参与 Score/Rank/选股。
	DecisionID string    `json:"decisionId" gorm:"column:decision_id;size:191;index"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (CandidatePoolItem) TableName() string { return "candidate_pool_items" }
