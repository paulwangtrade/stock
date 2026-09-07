// Package sellsuggestion is Phase13 Sell Suggestion Engine.
//
// After-close (or any caller) may Generate a suggest-only report:
// HoldingDecision → SellAllocation → optional Rebalance contrast → SellSuggestion[].
//
// DefaultEnabled=false. Never creates SellTradePlan, never calls Execution / Broker,
// never imports strategy buy/sell write chain.
package sellsuggestion

import (
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
)

const (
	SchemaVersion  = "sell_suggestion.p13-v1"
	DefaultEnabled = false

	PhaseSuggestOnly = "suggest_only"

	ActionHold   = holdingdecision.ActionHold
	ActionReduce = holdingdecision.ActionReduce
	ActionExit   = holdingdecision.ActionExit

	dataSourceNote = "SellSuggestion P13 · HoldingDecision + SellAllocation + optional Rebalance contrast; suggest_only; not SellTradePlan; not Execution; not Broker"
)

// Options controls the engine (all observation / suggest-only).
type Options struct {
	// Enabled must be true to run; default false → skipped report.
	Enabled bool `json:"enabled"`
	// IncludeHold emits HOLD rows with suggest_sell_qty=0 (default false → only REDUCE|EXIT).
	IncludeHold bool `json:"include_hold"`
	// AttachRebalance runs H4 Calculate for REDUCE/EXIT contrast annotations (default false).
	AttachRebalance bool `json:"attach_rebalance"`
	// DecisionPolicy drives holdingdecision.Observe (defaults: all action gates off → HOLD).
	DecisionPolicy holdingdecision.ActionPolicy `json:"-"`
	// AllocOptions drives sellallocation (forced suggest_only).
	AllocOptions sellallocation.Options `json:"-"`
	// RebalanceOptions when AttachRebalance (forced suggest_only).
	RebalanceOptions rebalance.CalculateOptions `json:"-"`
}

// DefaultOptions is production-safe: engine OFF, no rebalance attach, Decision gates off.
func DefaultOptions() Options {
	return Options{
		Enabled:           DefaultEnabled,
		IncludeHold:       false,
		AttachRebalance:   false,
		DecisionPolicy:    holdingdecision.DefaultActionPolicy(),
		AllocOptions:      sellallocation.DefaultOptions(),
		RebalanceOptions:  rebalance.DefaultCalculateOptions(),
	}
}

// CurrentPositionView is the book surface embedded in each suggestion.
type CurrentPositionView struct {
	TotalQty     int64   `json:"total_qty"`
	AvailableQty int64   `json:"available_qty"`
	LockedQty    int64   `json:"locked_qty"`
	MarketValue  float64 `json:"market_value"`
	Weight       float64 `json:"weight"`
	CanSell      bool    `json:"can_sell"`
	HoldingDays  int     `json:"holding_days,omitempty"`
}

// SellSuggestion is one symbol-level suggest-only sell row (not an order).
type SellSuggestion struct {
	Symbol          string              `json:"symbol"`
	Action          string              `json:"action"` // HOLD|REDUCE|EXIT
	Reason          string              `json:"reason"`
	CurrentPosition CurrentPositionView `json:"current_position"`
	SuggestSellQty  int64               `json:"suggest_sell_qty"`
	TargetWeight    float64             `json:"target_weight"`
	RiskReason      string              `json:"risk_reason"`

	ReasonCodes      []string `json:"reason_codes,omitempty"`
	Binding          string   `json:"binding,omitempty"`
	ExecutableHint   bool     `json:"executable_hint"`
	RebalanceContrast string  `json:"rebalance_contrast,omitempty"`

	RecordOnly   bool `json:"record_only"`
	NotAnOrder   bool `json:"not_an_order"`
	NotTradePlan bool `json:"not_trade_plan"`
	NotExecution bool `json:"not_execution"`
	NotBroker    bool `json:"not_broker"`
	SuggestOnly  bool `json:"suggest_only"`
}

// Input is the Generate entry. Facts are injected; engine does not query DB.
type Input struct {
	AsOf      time.Time `json:"as_of"`
	TradeDate string    `json:"trade_date"`
	AccountID string    `json:"account_id,omitempty"`
	Equity    float64   `json:"equity,omitempty"`

	// Observation facts for HoldingDecision (required when PrecomputedDecisions is nil).
	Observation holdingdecision.ObservationInput `json:"-"`

	// PrecomputedDecisions skips Observe when set (tests / upstream already ran H1).
	PrecomputedDecisions *holdingdecision.ActionView `json:"-"`

	// Positions keyed by lower symbol; required for sizing. Missing → DATA_MISSING row skip.
	Positions map[string]sellallocation.CurrentPosition `json:"-"`

	// Optional risk for allocation boost / risk_reason (RiskBoost still default off).
	PortfolioRisk *portfoliorisk.PortfolioRiskSnapshot `json:"-"`

	// Optional target book for SellAllocation REDUCE refine + Rebalance Calculate.
	SellTarget      *sellallocation.TargetPortfolio `json:"-"`
	RebalanceTarget *rebalance.TargetPortfolio      `json:"-"`
	RebalanceCurrent *rebalance.CurrentPortfolio    `json:"-"`

	Options Options `json:"options"`
}

// Report is the after-close suggest-only output.
type Report struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	TradeDate     string    `json:"trade_date"`
	AccountID     string    `json:"account_id,omitempty"`

	Enabled    bool   `json:"enabled"`
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skip_reason,omitempty"`

	SuggestOnly      bool `json:"suggest_only"`
	RecordOnly       bool `json:"record_only"`
	NotASellTradePlan bool `json:"not_a_sell_trade_plan"`
	NotExecution     bool `json:"not_execution"`
	NotBroker        bool `json:"not_broker"`
	NotBuyChain      bool `json:"not_buy_chain"`
	NotAutoTrade     bool `json:"not_auto_trade"`
	PersistSellPlans bool `json:"persist_sell_plans"` // always false

	Suggestions []SellSuggestion `json:"suggestions"`

	ByAction map[string]int `json:"by_action"`
	Summary  Summary        `json:"summary"`

	DecisionFingerprint  string `json:"decision_fingerprint,omitempty"`
	RebalanceAttached    bool   `json:"rebalance_attached"`
	RebalanceFingerprint string `json:"rebalance_fingerprint,omitempty"`

	DataSourceNote string   `json:"data_source_note"`
	Notes          []string `json:"notes,omitempty"`
}

// Summary aggregates suggestion counts / qty.
type Summary struct {
	SuggestionCount int   `json:"suggestion_count"`
	ReduceCount     int   `json:"reduce_count"`
	ExitCount       int   `json:"exit_count"`
	HoldCount       int   `json:"hold_count"`
	TotalSuggestQty int64 `json:"total_suggest_qty"`
}
