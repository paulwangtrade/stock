package models

// ResearchSnapshotCandidate 研究页「候选池」列表项（只读，来自最新信号快照）。
type ResearchSnapshotCandidate struct {
	StockCode   string  `json:"stockCode"`
	StockName   string  `json:"stockName"`
	SignalScore float64 `json:"signalScore"`
	SignalTag   string  `json:"signalTag"`
	Direction   string  `json:"direction"` // 看多 / 看空 / 中性
	StatusText  string  `json:"statusText"`
	Price       string  `json:"price"`
	Industry    string  `json:"industry"`
}

// ResearchSnapshotCandidateList 研究页候选池响应。
type ResearchSnapshotCandidateList struct {
	SnapshotID     uint                        `json:"snapshotId"`
	SnapshotTime   string                      `json:"snapshotTime"` // RFC3339
	TradeDate      string                      `json:"tradeDate"`
	Session        string                      `json:"session"`
	StrategyName   string                      `json:"strategyName"`
	MinScore       float64                     `json:"minScore"`
	HitTotal       int                         `json:"hitTotal"`
	ItemCount      int                         `json:"itemCount"`
	Message        string                      `json:"message"`
	Items          []ResearchSnapshotCandidate `json:"items"`
}
