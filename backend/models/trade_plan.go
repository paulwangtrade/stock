package models

import "time"

// TradePlan 生命周期状态（Phase6-A1）。
// 不新增 frozen 字符串：冻结 = ready AND FreezeAt != nil。
// 现网兼容：ready AND FreezeAt == nil 仍可由 9:25/9:30 消费（直至后续 Execute gate 收紧）。
//
//	draft ──Approve/Freeze──► ready(+FreezeAt) ──TryBeginExecute──► executing
//	  │                            │                                    │
//	  │                            └── supersede ◄── 同日新 ready        ▼
//	  └── failed / superseded                         done|partial|skipped|failed
const (
	// --- lifecycle / in-flight ---
	TradePlanStatusDraft     = "draft"
	TradePlanStatusReady     = "ready"
	TradePlanStatusExecuting = "executing"

	// --- terminal ---
	TradePlanStatusDone       = "done"
	TradePlanStatusPartial    = "partial"
	TradePlanStatusSkipped    = "skipped"
	TradePlanStatusFailed     = "failed"
	TradePlanStatusSuperseded = "superseded"

	TradePlanItemPending = "pending"
	TradePlanItemFilled  = "filled"
	TradePlanItemSkipped = "skipped"
	TradePlanItemError   = "error"

	PaperStrategyTagTradePlan = "trade_plan"

	// SourceSession / SourceKind 取值（计划来源观测；A1 不强制写入）。
	TradePlanSourceAfterClose     = "after_close"
	TradePlanSourceMorningRebuild = "morning_rebuild"
	// TradePlanSourceCashRescale marks a plan created by cash rescale (not morning rebuild).
	TradePlanSourceCashRescale = "cash_rescale"
	// TradePlanSourceTSell marks a T manual sell draft (Phase13-D2-W2).
	TradePlanSourceTSell = "t_sell"
	// TradePlanSourceExitReview marks a sell draft created from Exit Review UI (Phase14-M1).
	TradePlanSourceExitReview = "exit_review"
	// TradePlanSourceWatchlist marks a buy draft created from Watchlist UI (Phase16.26-C2.3.2-B1).
	TradePlanSourceWatchlist = "watchlist"

	// Provider metadata (G.13) — plan-header identity of who sized the draft.
	// Empty strings mean "unrecorded / pre-metadata era"; do not backfill history.
	TradePlanProviderModeOff           = "off"
	TradePlanProviderModeShadow        = "shadow"
	TradePlanProviderModeSimulation    = "simulation"
	TradePlanProviderModeControlled    = "controlled"
	TradePlanProviderModeOn            = "on"
	TradePlanDecisionProviderFixed     = "fixed_amount"
	TradePlanDecisionProviderPortfolio = "portfolio_allocation"
	TradePlanDecisionVersionG21        = "decision_provider.g2-1"
	TradePlanAllocationVersionF1Equal  = "allocation@f1-v1-equal"
)

// TradePlan 某交易日买入计划（同日可有历史版本；执行入口当前只认 status=ready）。
// TradeDate 即执行日（设计文档中的 TradingDate）；PlanVersion/FreezeAt 等为 Phase6-A 生命周期字段。
type TradePlan struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	TradeDate      string    `json:"tradeDate" gorm:"size:10;index;not null"`
	PoolID         uint      `json:"poolId" gorm:"index"`
	GeneratedAt    time.Time `json:"generatedAt"`
	Status         string    `json:"status" gorm:"size:16;index"`
	Side           string    `json:"side" gorm:"size:8"`
	AmountPerStock float64   `json:"amountPerStock"`
	MaxNames       int       `json:"maxNames"`
	EnableExecute  bool      `json:"enableExecute"`
	Message        string    `json:"message" gorm:"size:500"`

	// PlanVersion 同 TradeDate 下版本号；0=历史/未赋值（兼容旧行）。
	PlanVersion int `json:"planVersion" gorm:"column:plan_version"`
	// FreezeAt 非空且 Status=ready 表示已冻结；nil 的 ready 为早盘即时计划（兼容）。
	FreezeAt *time.Time `json:"freezeAt" gorm:"column:freeze_at"`
	// FreezeBy / FreezeReason 冻结审计（Freeze 台阶写入；不参与 IsFrozen 判定）。
	FreezeBy     string `json:"freezeBy" gorm:"column:freeze_by;size:64"`
	FreezeReason string `json:"freezeReason" gorm:"column:freeze_reason;size:500"`
	// ApprovedAt / ApprovedBy / ApprovalReason 审批审计（Approve 台阶写入；Freeze 前 Status 仍为 draft）。
	ApprovedAt     *time.Time `json:"approvedAt" gorm:"column:approved_at"`
	ApprovedBy     string     `json:"approvedBy" gorm:"column:approved_by;size:64"`
	ApprovalReason string     `json:"approvalReason" gorm:"column:approval_reason;size:500"`
	// ApprovedSource 审批渠道审计（Phase6.5.6.15 v5；如 system / http_api / wails）。
	ApprovedSource string `json:"approvedSource" gorm:"column:approved_source;size:32"`
	// SourceSession 如 after_close / morning_rebuild / cash_rescale。
	SourceSession string `json:"sourceSession" gorm:"column:source_session;size:32"`

	// --- Cash rescale audit (Phase11; append-only on v2+ plans; zero on normal plans) ---
	// ParentPlanID is the source plan id when SourceKind=cash_rescale; 0 otherwise.
	ParentPlanID uint `json:"parentPlanId" gorm:"column:parent_plan_id;index"`
	// SourceKind structured origin; prefer over parsing Message (e.g. cash_rescale).
	SourceKind string `json:"sourceKind" gorm:"column:source_kind;size:32"`
	// RescaleMode: none | scale_down | trim_names (persisted only when SourceKind=cash_rescale).
	RescaleMode        string  `json:"rescaleMode" gorm:"column:rescale_mode;size:16"`
	AvailableCashUsed  float64 `json:"availableCashUsed" gorm:"column:available_cash_used"`
	RequiredCashBefore float64 `json:"requiredCashBefore" gorm:"column:required_cash_before"`
	RequiredCashAfter  float64 `json:"requiredCashAfter" gorm:"column:required_cash_after"`
	// ScaleRatio = required_after / required_before when required_before > 0; else 0.
	ScaleRatio float64 `json:"scaleRatio" gorm:"column:scale_ratio"`

	// Provider metadata (G.13 / schema v10). Written only at CreatePlanWithItems INSERT.
	// Freeze / Approve / Execute / materialization must never UPDATE these columns.
	ProviderMode      string `json:"providerMode" gorm:"column:provider_mode;size:32"`
	DecisionProvider  string `json:"decisionProvider" gorm:"column:decision_provider;size:64"`
	DecisionVersion   string `json:"decisionVersion" gorm:"column:decision_version;size:64"`
	AllocationVersion string `json:"allocationVersion" gorm:"column:allocation_version;size:64"`

	// Execution Intent 默认（Phase6.5.6 schema v4；Writer 未接前保持零值）。
	DefaultEntryRule     string   `json:"defaultEntryRule" gorm:"column:default_entry_rule;size:32"`
	DefaultMaxSlippage   *float64 `json:"defaultMaxSlippage" gorm:"column:default_max_slippage"` // nil=未配置；0=禁止上浮
	PricingPolicyVersion int      `json:"pricingPolicyVersion" gorm:"column:pricing_policy_version"`
	PricingStage         string   `json:"pricingStage" gorm:"column:pricing_stage;size:32"`

	RiskStatus        string     `json:"riskStatus" gorm:"size:16"`
	MarketLevel       int        `json:"marketLevel"`
	RiskFilteredCount int        `json:"riskFilteredCount"`
	RiskAcceptedCount int        `json:"riskAcceptedCount"`
	RiskSummary       string     `json:"riskSummary" gorm:"size:500"`
	RiskSnapshotJSON  string     `json:"riskSnapshotJson" gorm:"type:text"`
	CheckedAt         *time.Time `json:"checkedAt"`
	ExecutedAt        *time.Time `json:"executedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`

	Items []TradePlanItem `json:"items,omitempty" gorm:"foreignKey:PlanID;references:ID"`
}

func (TradePlan) TableName() string { return "trade_plans" }

// StampLegacyBuyDraftProviderMetadata stamps OFF=Legacy write-chain identity (G.13).
// Call before CreatePlanWithItems; never UPDATE after persist.
func StampLegacyBuyDraftProviderMetadata(p *TradePlan) {
	if p == nil {
		return
	}
	p.ProviderMode = TradePlanProviderModeOff
	p.DecisionProvider = TradePlanDecisionProviderFixed
	p.DecisionVersion = TradePlanDecisionVersionG21
	p.AllocationVersion = ""
}

// StampControlledBuyDraftProviderMetadata stamps G.8 Controlled write-chain identity.
// whitelistMatched=true → portfolio_allocation; false → fixed_amount (miss path).
// Call before CreatePlanWithItems; never UPDATE after persist / Freeze.
func StampControlledBuyDraftProviderMetadata(p *TradePlan, whitelistMatched bool) {
	if p == nil {
		return
	}
	p.ProviderMode = TradePlanProviderModeControlled
	p.DecisionVersion = TradePlanDecisionVersionG21
	if whitelistMatched {
		p.DecisionProvider = TradePlanDecisionProviderPortfolio
		p.AllocationVersion = TradePlanAllocationVersionF1Equal
		return
	}
	p.DecisionProvider = TradePlanDecisionProviderFixed
	p.AllocationVersion = ""
}

// IsDraft 是否草稿（不可执行）。
func (p TradePlan) IsDraft() bool { return p.Status == TradePlanStatusDraft }

// IsReady 是否 ready 状态（含未冻结的早盘 ready）。
func (p TradePlan) IsReady() bool { return p.Status == TradePlanStatusReady }

// IsFrozen 已冻结：ready 且 FreezeAt 已设置。
func (p TradePlan) IsFrozen() bool {
	return p.Status == TradePlanStatusReady && p.FreezeAt != nil && !p.FreezeAt.IsZero()
}

// IsExecutableStatus 现网可执行状态判定：仅看 status==ready。
// 故意不要求 FreezeAt，以兼容早盘即时 ready（Phase6-A6 再收紧）。
func (p TradePlan) IsExecutableStatus() bool { return p.Status == TradePlanStatusReady }

// IsTerminal 是否终态（含 superseded）。
func (p TradePlan) IsTerminal() bool {
	switch p.Status {
	case TradePlanStatusDone, TradePlanStatusPartial, TradePlanStatusSkipped,
		TradePlanStatusFailed, TradePlanStatusSuperseded:
		return true
	default:
		return false
	}
}

// TradePlanItem 计划内单票意图与执行回写（含风控拒绝项 status=skipped）。
type TradePlanItem struct {
	ID              uint    `json:"id" gorm:"primaryKey"`
	PlanID          uint    `json:"planId" gorm:"index;not null"`
	TradeDate       string  `json:"tradeDate" gorm:"size:10;index"`
	StockCode       string  `json:"stockCode" gorm:"size:16;index"`
	StockName       string  `json:"stockName" gorm:"size:64"`
	Side            string  `json:"side" gorm:"size:8"`
	Priority        int     `json:"priority"`
	TargetAmount    float64 `json:"targetAmount"`
	TargetVolume    int64   `json:"targetVolume"`
	LimitPrice      float64 `json:"limitPrice"`
	Score           float64 `json:"score"`
	Reason          string  `json:"reason" gorm:"size:255"`
	StrategyName    string  `json:"strategyName" gorm:"size:120"`
	StrategyVersion string  `json:"strategyVersion" gorm:"size:64"`
	Status          string  `json:"status" gorm:"size:16;index"`
	RiskCode        string  `json:"riskCode" gorm:"size:64"`
	RiskMessage     string  `json:"riskMessage" gorm:"size:500"`
	Error           string  `json:"error" gorm:"size:500"`
	OrderID         uint    `json:"orderId" gorm:"index"`
	FillID          uint    `json:"fillId" gorm:"index"`
	FilledPrice     float64 `json:"filledPrice"`
	FilledVolume    int64   `json:"filledVolume"`
	FilledFee       float64 `json:"filledFee"`

	// OriginalTargetAmount preserves pre-rescale target for trim audit rows
	// (CASH_RESCALE_TRIMMED); zero when not applicable.
	OriginalTargetAmount float64 `json:"originalTargetAmount" gorm:"column:original_target_amount"`

	// Execution Intent（Phase6.5.6 schema v4；零值=legacy，不回填历史）。
	RefPrice     float64    `json:"refPrice" gorm:"column:ref_price"`
	RefSource    string     `json:"refSource" gorm:"column:ref_source;size:32"`
	RefAsOf      string     `json:"refAsOf" gorm:"column:ref_as_of;size:32"`
	EntryRule    string     `json:"entryRule" gorm:"column:entry_rule;size:32"`
	MaxSlippage  *float64   `json:"maxSlippage" gorm:"column:max_slippage"` // nil=继承 Plan 默认
	IntentStatus string     `json:"intentStatus" gorm:"column:intent_status;size:16"`
	OpenRefPrice float64    `json:"openRefPrice" gorm:"column:open_ref_price"`
	PricedAt     *time.Time `json:"pricedAt" gorm:"column:priced_at"`
	PricedBy     string     `json:"pricedBy" gorm:"column:priced_by;size:64"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TradePlanItem) TableName() string { return "trade_plan_items" }
