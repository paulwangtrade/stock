package holdingdecision

import (
	"time"

	"go-stock/backend/portfoliorisk"
)

// H.1 observation actions (not sell order verbs).
const (
	ActionHold   = "HOLD"
	ActionReduce = "REDUCE"
	ActionExit   = "EXIT"
)

const (
	ReasonConcentration = "CONCENTRATION"
	ReasonExitPolicy    = "EXIT_POLICY"
	ReasonReducePolicy  = "REDUCE_POLICY"
	ReasonT1Locked      = "T1_LOCKED"
)

const (
	ExitIntentFlattenSellable = "flatten_sellable"
	actionObserveNote         = "HoldingDecision H1.1 observation · HOLD|REDUCE|EXIT only; persist_sell_plans=false; not Execution; not SellTradePlan; not buy-chain"
	actionObserveNoteH12      = "HoldingDecision H1.2 Rule Engine · suggest_only; Fact→RuleHit→Merger; persist_sell_plans=false; not Execution; not SellTradePlan; not buy-chain"
)

// ActionPolicy controls H.1/H1.2 observation. PersistSellPlans is always ignored for side effects.
type ActionPolicy struct {
	PersistSellPlans     bool    // default false; Observe never creates sell plans
	ReduceEnabled        bool    // default false
	ExitEnabled          bool    // default false; distinct from ExitCandidateEnabled (state only)
	ExitCandidateEnabled bool    // D.8 state gate only; does not imply action=EXIT
	ConcentrationCap     float64 // 0 → use RiskSnapshot.MaxSingleNamePct when present
	DefaultReduceFraction float64

	// UseRuleEngine enables H1.2 Fact→Rules→Merger path inside Observe (default false → H1.1 switch).
	UseRuleEngine bool
	// RulePolicy is used when UseRuleEngine is true. Zero value = all families off → HOLD.
	RulePolicy RuleEnginePolicy
}

// RuleEnginePolicy is the Observe-facing H1.2 policy (mirrors rules.Policy fields).
type RuleEnginePolicy struct {
	EngineEnabled          bool
	EnablePnL              bool
	EnableTenure           bool
	EnableTrend            bool
	EnableRisk             bool
	EnablePortfolioTighten bool
	TakeProfitReturn       float64
	LargeProfitProtect     float64
	MaxLossReturn          float64 // negative threshold
	StaleHoldingDays       int
	SoftHoldingDays        int // 超期观察（HOLD review）；0=off
}

// DefaultActionPolicy is production-safe: all actions HOLD, persist off, rule engine off.
func DefaultActionPolicy() ActionPolicy {
	return ActionPolicy{
		PersistSellPlans:      false,
		ReduceEnabled:         false,
		ExitEnabled:           false,
		ExitCandidateEnabled:  false,
		DefaultReduceFraction: 0.5,
		UseRuleEngine:         false,
	}
}

// HoldingFact is a book holding row (from Snapshot / positions).
type HoldingFact struct {
	Symbol      string
	Weight      float64
	TotalQty    int64
	MarketValue float64
}

// PositionStateFact is T+1 / sellability (from PositionState).
type PositionStateFact struct {
	Symbol       string
	CanSell      bool
	TotalQty     int64
	AvailableQty int64
	LockedQty    int64
	HoldingDays  int
	State        string
}

// EvalFact is Holding Evaluation evidence for one symbol.
type EvalFact struct {
	Symbol       string
	CurrentPrice *float64
	ReturnRate   *float64
	HoldingDays  int
	RiskState    string
	ProfitState  string
	PeriodState  string
	QuoteSource  string
}

// StrategyScoreFact is optional quality score evidence (never sole EXIT trigger).
type StrategyScoreFact struct {
	Symbol string
	Score  *float64 // nil = missing
}

// RiskSnapshotFact is optional portfolio risk / config对照 (not PlanFilter).
type RiskSnapshotFact struct {
	MaxSingleNamePct *float64
	GrossExposure    *float64
	Top1Weight       *float64
	MarketRegime     string
	Found            bool
}

// ObservationInput is the pure H.1 / H1.2 observation input bundle.
type ObservationInput struct {
	AsOf           time.Time
	TradeDate      string
	Holdings       []HoldingFact
	PositionStates map[string]PositionStateFact // key = lower symbol
	Evaluations    map[string]EvalFact
	StrategyScores map[string]StrategyScoreFact
	RiskSnapshot   *RiskSnapshotFact
	// H1.2 optional enrichments (Rule Engine path).
	TrendFacts    map[string]TrendFactInput           // explicit only; missing → no trend rules
	PortfolioRisk *portfoliorisk.PortfolioRiskSnapshot // full risk; nil → risk rules skip safely
	Industries    map[string]string                   // symbol → industry for sector rule
	Policy        ActionPolicy
}

// TrendFactInput is the Observe-facing explicit trend fact (same semantics as rules.TrendFact).
type TrendFactInput struct {
	Available    bool   `json:"available"`
	ThesisBroken bool   `json:"thesis_broken,omitempty"`
	BelowMA      bool   `json:"below_ma,omitempty"`
	Source       string `json:"source,omitempty"`
	Note         string `json:"note,omitempty"`
}

// ReduceHint is observational sizing intent (not an order).
type ReduceHint struct {
	Fraction          *float64 `json:"fraction,omitempty"`
	TargetWeightAfter *float64 `json:"target_weight_after,omitempty"`
	TargetQty         *int64   `json:"target_qty,omitempty"` // unset in observation; allocation is later
}

// ExitHint is observational exit intent (flatten sellable only).
type ExitHint struct {
	Intent string `json:"intent"`
}

// ActionDecision is one symbol-level H.1/H1.2 HoldingDecision observation.
type ActionDecision struct {
	Symbol              string      `json:"symbol"`
	State               string      `json:"state"`
	Action              string      `json:"action"` // HOLD|REDUCE|EXIT
	Reason              string      `json:"reason"`
	ReasonCodes         []string    `json:"reason_codes"`
	FiredRules          []FiredRule `json:"fired_rules,omitempty"` // H1.2 rule engine hits
	ConflictResolution  []string    `json:"conflict_resolution,omitempty"`
	Summary             string      `json:"summary"`
	Evidence            Evidence    `json:"evidence"`
	Weight              float64     `json:"weight,omitempty"`
	StrategyScore       *float64    `json:"strategy_score,omitempty"`
	Reduce              *ReduceHint `json:"reduce,omitempty"`
	Exit                *ExitHint   `json:"exit,omitempty"`
	ExecutableHint      bool        `json:"executable_hint"`
	RecordOnly          bool        `json:"record_only"`
	NotAnOrder          bool        `json:"not_an_order"`
	PersistSellPlans    bool        `json:"persist_sell_plans"` // always false in this observation slice
	SuggestOnly         bool        `json:"suggest_only,omitempty"`
}

// ActionView is the H.1 observation payload.
type ActionView struct {
	SchemaVersion     string           `json:"schema_version"`
	AsOf              string           `json:"as_of,omitempty"`
	TradeDate         string           `json:"trade_date,omitempty"`
	PersistSellPlans  bool             `json:"persist_sell_plans"`
	ReduceEnabled     bool             `json:"reduce_enabled"`
	ExitEnabled       bool             `json:"exit_enabled"`
	RecordOnly        bool             `json:"record_only"`
	NotAnOrder        bool             `json:"not_an_order"`
	NotExecution      bool             `json:"not_execution"`
	NotSellTradePlan  bool             `json:"not_sell_trade_plan"`
	NotBuyChain       bool             `json:"not_buy_chain"`
	ByAction          map[string]int   `json:"by_action"`
	Decisions         []ActionDecision `json:"decisions"`
	DataSourceNote    string           `json:"data_source_note"`
	InputsFingerprint string           `json:"inputs_fingerprint"`
	SuggestOnly       bool             `json:"suggest_only"`
	RuleEngineUsed    bool             `json:"rule_engine_used,omitempty"`
}

const ActionSchemaVersion = "holding_decision.action.h1-1"
const ActionSchemaVersionH12 = "holding_decision.action.h1-2"
