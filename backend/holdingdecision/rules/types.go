// Package rules is the H1.2 Holding Evaluation Rule Engine.
// Observation only: RuleHit → Merger → HoldingDecisionResult.
// Never creates SellTradePlan, never calls Execution, never touches the buy chain.
package rules

import "go-stock/backend/portfoliorisk"

const (
	ActionHold   = "HOLD"
	ActionReduce = "REDUCE"
	ActionExit   = "EXIT"

	SchemaVersion = "holding_decision.rules.h1-2"

	FamilyPnL       = "pnl"
	FamilyTenure    = "tenure"
	FamilyTrend     = "trend"
	FamilyRisk      = "risk"
	FamilyPortfolio = "portfolio"

	SeverityInfo     = "info"
	SeverityWatch    = "watch"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Reason codes (stable enums; not order verbs).
const (
	ReasonPnLTakeProfit      = "PNL_TAKE_PROFIT"
	ReasonPnLLargeProtect    = "PNL_LARGE_PROFIT_PROTECT"
	ReasonPnLMaxLoss         = "PNL_MAX_LOSS"
	ReasonPnLRiskWorse       = "PNL_RISK_DETERIORATION"
	ReasonTenureStale        = "TENURE_STALE_NO_CHANGE"
	ReasonTenureLongReview   = "TENURE_LONG_REVIEW"
	ReasonTrendBreak         = "TREND_BREAK"
	ReasonRiskNameOverCap    = "RISK_NAME_OVER_CAP"
	ReasonRiskGrossHot       = "RISK_GROSS_HOT"
	ReasonRiskSectorHot      = "RISK_SECTOR_HOT"
	ReasonPortfolioTighten   = "PORTFOLIO_TIGHTEN_OBSERVE"
	ReasonDataMissing        = "DATA_MISSING"
	ReasonRuleGated          = "RULE_GATED"
	ReasonDefaultHold        = "DEFAULT_HOLD"
)

// Rule IDs.
const (
	RulePnLTakeProfit    = "pnl.profit_target"
	RulePnLLargeProtect  = "pnl.large_profit_protect"
	RulePnLMaxLoss       = "pnl.max_loss"
	RulePnLRiskWorse     = "pnl.risk_deterioration"
	RuleTenureStale      = "tenure.stale_no_change"
	RuleTenureLongReview = "tenure.long_review"
	RuleTrendBreak       = "trend.break"
	RuleRiskNameOverCap  = "risk.name_over_cap"
	RuleRiskGrossHot     = "risk.gross_hot"
	RuleRiskSectorHot    = "risk.sector_hot"
	RulePortfolioTighten = "portfolio.tighten_observe"
)

// RuleHit is one fired observation rule (not an order).
type RuleHit struct {
	RuleID          string         `json:"rule_id"`
	Family          string         `json:"family"`
	ActionCandidate string         `json:"action_candidate"` // HOLD|REDUCE|EXIT
	Severity        string         `json:"severity"`
	ReasonCode      string         `json:"reason_code"`
	Evidence        map[string]any `json:"evidence,omitempty"`
}

// TrendFact must be explicit; missing / Available=false → trend rules skip.
type TrendFact struct {
	Available    bool   `json:"available"`
	ThesisBroken bool   `json:"thesis_broken,omitempty"`
	BelowMA      bool   `json:"below_ma,omitempty"`
	Source       string `json:"source,omitempty"`
	Note         string `json:"note,omitempty"`
}

// TightenedConstraints is an optional post-SuggestTighten observation surface.
// Not a TradePlan; not PlanFilter. Only used when Policy.EnablePortfolioTighten.
type TightenedConstraints struct {
	Available       bool     `json:"available"`
	MaxSingleWeight *float64 `json:"max_single_weight,omitempty"`
	MaxGrossPct     *float64 `json:"max_gross_pct,omitempty"`
	Source          string   `json:"source,omitempty"` // e.g. suggest_tighten
}

// RuleContext is per-symbol evaluation input (read-only facts).
type RuleContext struct {
	Symbol string

	Weight       float64
	TotalQty     int64
	MarketValue  float64
	Cost         *float64
	CurrentPrice *float64
	ReturnRate   *float64 // optional precomputed; else derived from cost/price
	HoldingDays  int
	ProfitState  string
	RiskState    string
	PeriodState  string

	CanSell      bool
	AvailableQty int64

	Trend *TrendFact

	PortfolioRisk *portfoliorisk.PortfolioRiskSnapshot
	Industry      string // for sector rule when sector block available
	Tightened     *TightenedConstraints

	Policy Policy
}

// Policy controls which rules may fire. DefaultPolicy() disables everything → HOLD.
type Policy struct {
	EngineEnabled bool // master switch; default false → no business rules
	SuggestOnly   bool // always treated as true by Decide/Evaluate

	ReduceEnabled bool
	ExitEnabled   bool

	EnablePnL              bool
	EnableTenure           bool
	EnableTrend            bool
	EnableRisk             bool
	EnablePortfolioTighten bool

	// Thresholds: ≤0 disables that predicate even when family enabled.
	TakeProfitReturn      float64 // e.g. 0.20 → REDUCE
	LargeProfitProtect    float64 // e.g. 0.40 → REDUCE (protect gains)
	MaxLossReturn         float64 // e.g. -0.10 → REDUCE (negative)
	StaleHoldingDays      int     // e.g. 60 → stale no-change
	SoftHoldingDays       int     // e.g. 40 → long review HOLD observation
	DefaultReduceFraction float64
}

// DefaultPolicy is production-safe: engine off, all families off, reduce/exit off, suggest_only.
func DefaultPolicy() Policy {
	return Policy{
		EngineEnabled:          false,
		SuggestOnly:            true,
		ReduceEnabled:          false,
		ExitEnabled:            false,
		EnablePnL:              false,
		EnableTenure:           false,
		EnableTrend:            false,
		EnableRisk:             false,
		EnablePortfolioTighten: false,
		DefaultReduceFraction:  0.5,
	}
}

// HoldingDecisionResult is the H1.2 per-symbol observation output (HoldingDecision).
type HoldingDecisionResult struct {
	SchemaVersion      string         `json:"schema_version"`
	Symbol             string         `json:"symbol"`
	FinalAction        string         `json:"final_action"` // HOLD|REDUCE|EXIT
	Action             string         `json:"action"`       // alias of FinalAction for HoldingDecision naming
	RuleHits           []RuleHit      `json:"rule_hits"`
	ReasonCodes        []string       `json:"reason_codes"`
	Evidence           map[string]any `json:"evidence,omitempty"`
	ConflictResolution []string       `json:"conflict_resolution,omitempty"`
	Explanation        string         `json:"explanation"`
	SuggestOnly        bool           `json:"suggest_only"`
	ExecutableHint     bool           `json:"executable_hint"`
	RecordOnly         bool           `json:"record_only"`
	NotAnOrder         bool           `json:"not_an_order"`
	NotSellTradePlan   bool           `json:"not_sell_trade_plan"`
	NotExecution       bool           `json:"not_execution"`
	NotBuyChain        bool           `json:"not_buy_chain"`
	PersistSellPlans   bool           `json:"persist_sell_plans"` // always false
}

// HoldingDecision is the H1.2 decision name (alias of HoldingDecisionResult).
type HoldingDecision = HoldingDecisionResult

// HoldingRule evaluates one rule against a context.
type HoldingRule interface {
	ID() string
	Family() string
	Eval(ctx RuleContext) []RuleHit
}
