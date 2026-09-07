package models

import "time"

// QuantDecision Phase0 契约冻结：统一决策快照（不落库、不接生产写路径）。
// 消费方：Watchlist 投影、Candidate/Risk/TradePlan 未来复用。
// 非职责：TradePlan 生命周期、Execution、PaperBroker。

const QuantDecisionSchemaVersion = 1

// Purpose：决策用途（同一引擎，不同消费者）。
const (
	QuantPurposeResearch  = "research"  // 研究/全市场扫描
	QuantPurposeWatchlist = "watchlist" // 自选卡片
	QuantPurposePlanBuild = "plan_build" // 构建 TradePlan
)

// Action.Code：对内唯一动作语义；UI 文案由 Code 投影，禁止前端二次判定。
const (
	QuantActionEnter        = "ENTER"
	QuantActionWaitPullback = "WAIT_PULLBACK"
	QuantActionWatch        = "WATCH"
	QuantActionScaleIn      = "SCALE_IN"
	QuantActionReduce       = "REDUCE"
	QuantActionExitPartial  = "EXIT_PARTIAL"
	QuantActionHold         = "HOLD"
	QuantActionBlocked      = "BLOCKED"
)

// Signal.TagKind
const (
	QuantTagKindNone    = "none"
	QuantTagKindEntry   = "entry"
	QuantTagKindExit    = "exit"
	QuantTagKindScaleIn = "scale_in"
	QuantTagKindIce     = "ice"
	QuantTagKindRush    = "rush_reduce"
)

// EntryZone.Mode / DeferMode（对齐 frontend buyPriceRange.js）
const (
	QuantZoneModeInZone  = "inZone"
	QuantZoneModeAbove   = "above"
	QuantZoneModeBelow   = "below"
	QuantZoneModeNear    = "near"
	QuantZoneModeUnknown = "unknown"

	QuantDeferSame             = "same"
	QuantDeferWait             = "wait"
	QuantDeferTodayOrTomorrow  = "todayOrTomorrow"
	QuantDeferDefer            = "defer"
)

// Blocker.Layer
const (
	QuantBlockerGate   = "gate"
	QuantBlockerRisk   = "risk"
	QuantBlockerSize   = "size"
	QuantBlockerRegime = "regime"
)

// Size.BindingConstraint
const (
	QuantBindRiskBudget  = "risk_budget"
	QuantBindPositionCap = "position_cap"
	QuantBindExposureCap = "exposure_cap"
	QuantBindMinLot      = "min_lot"
	QuantBindRegime      = "regime"
)

// HoldingBias.Action
const (
	QuantBiasAdd        = "add"
	QuantBiasReduce     = "reduce"
	QuantBiasHold       = "hold"
	QuantBiasLiquidate  = "liquidate"
)

// Producer
const (
	QuantProducerGoEngine = "go_engine"
	QuantProducerJSLegacy = "js_legacy"
)

// QuantDecision 单标的决策快照（Phase0：纯契约，无 TableName / 无 AutoMigrate）。
type QuantDecision struct {
	ID        string    `json:"id,omitempty"`
	AsOf      time.Time `json:"asOf"`
	TradeDate string    `json:"tradeDate"`
	Purpose   string    `json:"purpose"` // research | watchlist | plan_build

	Instrument  QuantInstrument    `json:"instrument"`
	Regime      QuantRegime        `json:"regime"`
	Signal      QuantSignal        `json:"signal"`
	EntryZone   *QuantEntryZone    `json:"entryZone,omitempty"`
	Gate        QuantGate          `json:"gate"`
	Risk        QuantRiskSlice     `json:"risk"`
	Size        QuantSize          `json:"size"`
	HoldingBias *QuantHoldingBias  `json:"holdingBias,omitempty"`
	Action      QuantAction        `json:"action"`
	Blockers    []QuantBlocker     `json:"blockers,omitempty"`
	Meta        QuantDecisionMeta  `json:"meta"`
}

// QuantInstrument 标的标识。
type QuantInstrument struct {
	StockCode string `json:"stockCode"`
	StockName string `json:"stockName,omitempty"`
	Industry  string `json:"industry,omitempty"`
}

// QuantRegime 市场级别（1=最防守 … 5=最进攻），与 PlanContext.MarketLevel / tradingLevelRules 对齐。
type QuantRegime struct {
	Level       int     `json:"level"`
	Key         string  `json:"key,omitempty"`  // level1…level5
	Name        string  `json:"name,omitempty"` // 空仓防守…
	Source      string  `json:"source,omitempty"`
	ExposureCap float64 `json:"exposureCap"` // 生效总敞口上限 0–1
}

// QuantSignal 研究信号（对齐 icePointSignals / CandidatePoolItem.SignalTag）。
type QuantSignal struct {
	Tag        string   `json:"tag,omitempty"`     // 强/趋/转/突/弹/买/冰/减/止/冲/加
	TagKind    string   `json:"tagKind"`           // entry|exit|scale_in|ice|rush_reduce|none
	DaysAgo    int      `json:"daysAgo"`
	Score      float64  `json:"score"`             // 0–100 research score
	RatioPct   *float64 `json:"ratioPct,omitempty"` // 减/止/加/冲 建议比例（0–1 或 百分数由生产者约定，见契约文档）
	SourceTag  string   `json:"sourceTag,omitempty"`
	Summary    string   `json:"summary,omitempty"`  // statusText
	BarIndex   *int     `json:"barIndex,omitempty"`
	ConfirmBar *int     `json:"confirmBar,omitempty"`
	DayKey     string   `json:"dayKey,omitempty"`
}

// QuantEntryZone 买入参考区间（对齐 buyPriceRange.js；非委托限价）。
type QuantEntryZone struct {
	Low          float64 `json:"low"`
	High         float64 `json:"high"`
	InstantPrice float64 `json:"instantPrice"`
	Mode         string  `json:"mode"`      // inZone|above|below|near|unknown
	DeferMode    string  `json:"deferMode"` // same|wait|todayOrTomorrow|defer
	DaysAgo      int     `json:"daysAgo"`
	Text         string  `json:"text,omitempty"`
	Note         string  `json:"note,omitempty"`
}

// QuantGateItem Pre-trade checklist 单项（对齐 buyChecklist.js）。
type QuantGateItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
	Detail   string `json:"detail,omitempty"`
}

// QuantGate 研究门控（≠ Risk PlanFilter）。
type QuantGate struct {
	Score          float64         `json:"score"` // 0–1 通过项占比
	Ready          bool            `json:"ready"`
	RequiredPassed bool            `json:"requiredPassed"`
	ReadyThreshold float64         `json:"readyThreshold"`
	Items          []QuantGateItem `json:"items,omitempty"`
}

// QuantRiskSlice 组合风控切片（对齐 risk.ReasonCode / PlanFilterItem）。
type QuantRiskSlice struct {
	Passed  bool   `json:"passed"`
	Code    string `json:"code"` // APPROVED | MARKET_LEVEL_BLOCKED | …
	Message string `json:"message,omitempty"`
}

// QuantSize 仓位规模（对齐 buyPositionSizing.js；可映射 TradePlanItem.Target*）。
type QuantSize struct {
	OK                bool    `json:"ok"`
	EntryPrice        float64 `json:"entryPrice,omitempty"`
	StopPrice         float64 `json:"stopPrice,omitempty"`
	RiskPerShare      float64 `json:"riskPerShare,omitempty"`
	Confidence        float64 `json:"confidence,omitempty"`
	TargetShares      int64   `json:"targetShares,omitempty"`
	AddShares         int64   `json:"addShares,omitempty"`
	TargetAmount      float64 `json:"targetAmount,omitempty"`
	PositionPct       float64 `json:"positionPct,omitempty"` // 占权益百分比数值，如 12.5 表示 12.5%
	BindingConstraint string  `json:"bindingConstraint,omitempty"`
	Reason            string  `json:"reason,omitempty"` // 人类可读；失败时必填
}

// QuantHoldingBias 持仓辅助（对齐 holdingPositionAdjust.js；不得覆盖 Action 主语义）。
type QuantHoldingBias struct {
	Action      string  `json:"action"` // add|reduce|hold|liquidate
	ActionLabel string  `json:"actionLabel,omitempty"`
	Score       float64 `json:"score"`
	SuggestPct  float64 `json:"suggestPct"` // 0–1
	SummaryLine string  `json:"summaryLine,omitempty"`
}

// QuantAction 派生动作（唯一对外动作语义）。
type QuantAction struct {
	Code       string `json:"code"`
	Label      string `json:"label,omitempty"` // 中文投影缓存
	AllowDraft bool   `json:"allowDraft"`
	Side       string `json:"side"` // buy|sell|none
}

// QuantBlocker 可观测阻断原因。
type QuantBlocker struct {
	Layer   string `json:"layer"` // gate|risk|size|regime
	Code    string `json:"code"`
	Message string `json:"message"`
}

// QuantDecisionMeta 溯源与账户上下文。
type QuantDecisionMeta struct {
	SchemaVersion    int     `json:"schemaVersion"`
	Producer         string  `json:"producer"` // go_engine | js_legacy
	StrategyName     string  `json:"strategyName,omitempty"`
	StrategyVersion  string  `json:"strategyVersion,omitempty"`
	SignalSnapshotID uint    `json:"signalSnapshotId,omitempty"`
	AccountEquity    float64 `json:"accountEquity,omitempty"`
	ExistingVolume   int64   `json:"existingVolume,omitempty"`
	CandidatePoolID  uint    `json:"candidatePoolId,omitempty"` // 可选反向钩子；Phase3-A 以 Item.DecisionID 正向引用为准
	TradePlanID      uint    `json:"tradePlanId,omitempty"`
	DecisionHash     string  `json:"decisionHash,omitempty"`
}
