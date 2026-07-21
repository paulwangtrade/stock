package risk

import "encoding/json"

// 计划风控状态（写入 TradePlan.RiskStatus）。
const (
	PlanRiskStatusBypassed = "bypassed" // Enable=false，与 Phase1 一致
	PlanRiskStatusPassed   = "passed"   // 有接受且无拒绝
	PlanRiskStatusPartial  = "partial"  // 部分接受部分拒绝
	PlanRiskStatusBlocked  = "blocked"  // 零接受
)

// PlanCandidate 进入计划风控的候选（由 strategy 从 CandidatePool 映射，不依赖 models）。
type PlanCandidate struct {
	StockCode       string
	StockName       string
	Rank            int
	Score           float64
	Reason          string
	StrategyName    string
	StrategyVersion string
	TargetAmount    float64
}

// PlanContext 计划态风控快照（调用方填充；本包不访问 DB）。
type PlanContext struct {
	Enabled bool // false 时 PlanFilter 旁路，行为同 Phase1 TopN

	MarketLevel     int
	BlockNewEntries bool

	Cash             float64
	EquityBase       float64
	LongMarketValue  float64
	ShortMarketValue float64
	// NameMarketValue 已有单票市值（code → value）
	NameMarketValue map[string]float64

	MaxGrossExposurePct float64
	MaxSingleNamePct    float64
	MaxDailyLossPct     float64
	CurrentDailyPnlPct  float64

	AmountPerStock float64
	MaxNames       int
	ScanLimit      int // 最多扫描候选数；0 表示用 MaxNames（旁路）或宽松上限
}

// PlanFilterItem 单票过滤结果（接受 pending / 拒绝 skipped）。
type PlanFilterItem struct {
	Candidate   PlanCandidate
	Allowed     bool
	Status      string // pending | skipped
	RiskCode    ReasonCode
	RiskMessage string
	Priority    int
}

// PlanFilterResult 供 BuildTradePlan 落库，不含风控规则。
type PlanFilterResult struct {
	Items           []PlanFilterItem
	AcceptedCount   int
	FilteredCount   int
	MarketLevel     int
	RiskStatus      string
	RiskSummary     string
	RiskSnapshotJSON string
	Equity          float64
	GrossExposure   float64
}

// PlanSnapshot 写入 RiskSnapshotJSON 的可序列化摘要。
type PlanSnapshot struct {
	Enabled             bool    `json:"enabled"`
	MarketLevel         int     `json:"marketLevel"`
	BlockNewEntries     bool    `json:"blockNewEntries"`
	Cash                float64 `json:"cash"`
	Equity              float64 `json:"equity"`
	LongMarketValue     float64 `json:"longMarketValue"`
	GrossExposure       float64 `json:"grossExposure"`
	MaxGrossExposurePct float64 `json:"maxGrossExposurePct"`
	MaxSingleNamePct    float64 `json:"maxSingleNamePct"`
	MaxDailyLossPct     float64 `json:"maxDailyLossPct"`
	CurrentDailyPnlPct  float64 `json:"currentDailyPnlPct"`
	AmountPerStock      float64 `json:"amountPerStock"`
	MaxNames            int     `json:"maxNames"`
	Accepted            int     `json:"accepted"`
	Rejected            int     `json:"rejected"`
}

func (r *PlanFilterResult) marshalSnapshot(snap PlanSnapshot) {
	b, err := json.Marshal(snap)
	if err != nil {
		r.RiskSnapshotJSON = "{}"
		return
	}
	r.RiskSnapshotJSON = string(b)
}
