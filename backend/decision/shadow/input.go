package shadow

import "go-stock/backend/models"

// Input 对齐 frontend assembleWatchlistDecision 的输入形（研究产物 → Decision）。
// Shadow 只消费这些只读输入，不读 TradePlan / 不下单。
type Input struct {
	Code      string
	Name      string
	Purpose   string // 默认 watchlist
	TradeDate string
	AsOf      string // RFC3339；空则测试用固定时刻

	Tag       string
	DaysAgo   int
	StatusText string
	SourceTag string
	SignalScore float64
	SellPositionPct *float64
	AddPositionPct  *float64
	RushReducePct   *float64
	SignalBar       *int
	DayKey          string

	EntryZone *models.QuantEntryZone

	Gate models.QuantGate
	// HasGate 为 false 时使用占位空 Gate（与 js 无 checklist 时 mapGate(null) 对齐）
	HasGate bool

	Size models.QuantSize
	HasSize bool

	MarketLevel int     // 1–5；0 表示未知→按 3
	ExposureCap float64 // 0 则按级别默认

	ExistingVolume int64
	ChecklistReady bool // 显式传入；也可由 Gate.Ready 推导
}

// Diff 字段级差异。
type Diff struct {
	Path  string `json:"path"`
	Slice string `json:"slice,omitempty"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
}

// Report Dual-run / shadow diff 报告。
type Report struct {
	Equal          bool              `json:"equal"`
	Summary        string            `json:"summary"`
	LeftProducer   string            `json:"leftProducer"`
	RightProducer  string            `json:"rightProducer"`
	Diffs          []Diff            `json:"diffs"`
	BySlice        map[string]SliceR `json:"bySlice"`
	Harness        string            `json:"harness"`
	Phase          string            `json:"phase"`
	BaselineID     string            `json:"baselineId,omitempty"`
	CandidateID    string            `json:"candidateId,omitempty"`
}

// SliceR 单切片对比结果。
type SliceR struct {
	Equal bool   `json:"equal"`
	Diffs []Diff `json:"diffs"`
}
