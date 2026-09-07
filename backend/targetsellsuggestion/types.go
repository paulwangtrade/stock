// Package targetsellsuggestion is Phase13 Target-aware Sell Suggestion.
//
// Current holdings + TargetPortfolio → SELL weight delta → suggest_only qty.
// Composes portfoliotarget / rebalance / sellallocation (optional sellsuggestion contrast).
//
// DefaultEnabled=false. Never creates SellTradePlan, never calls Execution,
// never mutates Holdings.
package targetsellsuggestion

import (
	"time"

	"go-stock/backend/portfoliotarget"
	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
)

const (
	SchemaVersion  = "target_sell_suggestion.p13-v1"
	DefaultEnabled = false
	PhaseSuggestOnly = "suggest_only"

	ActionReduce = sellallocation.ActionReduce
	ActionExit   = sellallocation.ActionExit

	ReasonOverweight  = rebalance.ReasonOverweight
	ReasonTargetZero  = rebalance.ReasonTargetZero
	ReasonOrphan      = rebalance.ReasonOrphan
	ReasonBelowMin    = rebalance.ReasonBelowMinDelta
	ReasonDataMissing = rebalance.ReasonDataMissing
	ReasonAtTarget    = rebalance.ReasonAlreadyAtTarget

	dataSourceNote = "TargetSellSuggestion P13 · Current vs Target SELL delta; suggest_only; not SellTradePlan; not Execution; not Holdings mutate"
)

// Options controls the engine (observation / suggest-only).
type Options struct {
	// Enabled must be true to run; default false → skipped report.
	Enabled bool `json:"enabled"`
	// IncludeAtTarget emits rows with weight_gap≈0 / suggest_qty=0 (default false).
	IncludeAtTarget bool `json:"include_at_target"`
	// MinDeltaWeight overrides rebalance min gap (0 → H4 default 0.005).
	MinDeltaWeight float64 `json:"min_delta_weight,omitempty"`
	// AllocOptions sizes suggest_qty (forced suggest_only).
	AllocOptions sellallocation.Options `json:"-"`
	// RebalanceOptions for H4 Calculate (forced suggest_only; BUY deltas ignored).
	RebalanceOptions rebalance.CalculateOptions `json:"-"`
}

// DefaultOptions is production-safe: engine OFF.
func DefaultOptions() Options {
	return Options{
		Enabled:          DefaultEnabled,
		IncludeAtTarget:  false,
		AllocOptions:     sellallocation.DefaultOptions(),
		RebalanceOptions: rebalance.DefaultCalculateOptions(),
	}
}

// HoldingPosition is one current book row (injected; never written back).
type HoldingPosition struct {
	Symbol       string  `json:"symbol"`
	Qty          int64   `json:"qty"`
	AvailableQty int64   `json:"available_qty"`
	LockedQty    int64   `json:"locked_qty"`
	MarketValue  float64 `json:"market_value"`
	Weight       float64 `json:"weight"`
	MarkPrice    float64 `json:"mark_price,omitempty"`
	CanSell      bool    `json:"can_sell"`
}

// Input is the Generate entry. Facts are injected; no DB / no Holdings mutation.
type Input struct {
	AsOf      time.Time `json:"as_of"`
	TradeDate string    `json:"trade_date"`
	AccountID string    `json:"account_id,omitempty"`
	Equity    float64   `json:"equity,omitempty"`
	Cash      float64   `json:"cash,omitempty"`
	Exposure  float64   `json:"exposure,omitempty"`

	// Holdings is the current book (required).
	Holdings []HoldingPosition `json:"holdings"`

	// Target is preferred (portfoliotarget). Alternate: RebalanceTarget.
	Target          *portfoliotarget.TargetPortfolio `json:"-"`
	RebalanceTarget *rebalance.TargetPortfolio       `json:"-"`

	Options Options `json:"options"`
}

// TargetSellSuggestion is one SELL delta suggestion (not an order).
type TargetSellSuggestion struct {
	Symbol         string  `json:"symbol"`
	CurrentWeight  float64 `json:"current_weight"`
	TargetWeight   float64 `json:"target_weight"`
	WeightGap      float64 `json:"weight_gap"` // current − target; >0 means overweight
	SuggestQty     int64   `json:"suggest_qty"`
	Reason         string  `json:"reason"`

	Action       string   `json:"action,omitempty"` // REDUCE | EXIT
	ReasonCodes  []string `json:"reason_codes,omitempty"`
	AvailableQty int64    `json:"available_qty,omitempty"`
	Binding      string   `json:"binding,omitempty"`

	RecordOnly       bool `json:"record_only"`
	SuggestOnly      bool `json:"suggest_only"`
	NotAnOrder       bool `json:"not_an_order"`
	NotSellTradePlan bool `json:"not_sell_trade_plan"`
	NotExecution     bool `json:"not_execution"`
	NotHoldingsMutate bool `json:"not_holdings_mutate"`
}

// Report is the suggest-only batch output.
type Report struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of"`
	TradeDate     string    `json:"trade_date"`
	AccountID     string    `json:"account_id,omitempty"`

	Enabled    bool   `json:"enabled"`
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skip_reason,omitempty"`

	SuggestOnly       bool `json:"suggest_only"`
	RecordOnly        bool `json:"record_only"`
	NotASellTradePlan bool `json:"not_a_sell_trade_plan"`
	NotExecution      bool `json:"not_execution"`
	NotHoldingsMutate bool `json:"not_holdings_mutate"`
	NotAutoTrade      bool `json:"not_auto_trade"`

	Suggestions []TargetSellSuggestion `json:"suggestions"`
	Summary     Summary                `json:"summary"`

	RebalanceFingerprint string   `json:"rebalance_fingerprint,omitempty"`
	DataSourceNote       string   `json:"data_source_note"`
	Notes                []string `json:"notes,omitempty"`
}

// Summary aggregates suggestion counts.
type Summary struct {
	SuggestionCount int     `json:"suggestion_count"`
	ReduceCount     int     `json:"reduce_count"`
	ExitCount       int     `json:"exit_count"`
	TotalSuggestQty int64   `json:"total_suggest_qty"`
	TotalWeightGap  float64 `json:"total_weight_gap"`
}
