// Package sellallocation is Phase12-H1.3 Sell Allocation Engine (suggest_only).
//
// HoldingDecision → SellIntent → SellAllocation (how many shares to suggest selling).
// Never creates SellTradePlan, never calls Execution, never imports the buy write chain.
package sellallocation

import "go-stock/backend/portfoliorisk"

const (
	SchemaVersion    = "sellallocation.h1-3-v1"
	PhaseSuggestOnly = "suggest_only"

	ActionReduce = "REDUCE"
	ActionExit   = "EXIT"

	IntentFlattenSellable = "flatten_sellable"

	BindingOK          = "ok"
	BindingT1          = "t1"
	BindingIntent      = "intent"
	BindingRisk        = "risk"
	BindingLot         = "lot"
	BindingBelowMin    = "below_min"
	BindingDataMissing = "data_missing"
	BindingHoldSkip    = "hold_skip"
	BindingAtTarget    = "already_at_target"

	ReasonT1Locked       = "T1_LOCKED"
	ReasonT1Clip         = "T1_CLIP"
	ReasonLotRound       = "LOT_ROUND"
	ReasonBelowMin       = "BELOW_MIN"
	ReasonDataMissing    = "DATA_MISSING"
	ReasonActionHold     = "ACTION_HOLD"
	ReasonAlreadyAtTarget = "ALREADY_AT_TARGET"
	ReasonRiskBoost      = "RISK_BOOST"
	ReasonFromDecision   = "FROM_HOLDING_DECISION"

	SeverityNone = "none"
	SeverityMild = "mild"
	SeverityHigh = "high"

	PreferMin         = "min"
	PreferMax         = "max"
	PreferWeightOnly  = "weight_only"
	PreferRatioOnly   = "ratio_only"

	DefaultLotSize            int64   = 100
	DefaultReduceFraction     float64 = 0.5
	DefaultEpsilonWeight      float64 = 1e-6
	DefaultRiskBoostMildExtra float64 = 0.15
	DefaultRiskBoostHighExtra float64 = 0.35
)

// SellIntent is projected from HoldingDecision (REDUCE|EXIT only).
type SellIntent struct {
	Symbol               string   `json:"symbol"`
	Action               string   `json:"action"` // REDUCE | EXIT
	TargetPositionWeight float64  `json:"target_position_weight"`
	TargetReduceRatio    float64  `json:"target_reduce_ratio"`
	Reason               string   `json:"reason"`
	ReasonCodes          []string `json:"reason_codes,omitempty"`
	RecordOnly           bool     `json:"record_only"`
	NotAnOrder           bool     `json:"not_an_order"`
}

// CurrentPosition is the book + T+1 sellability surface for one symbol.
type CurrentPosition struct {
	Symbol       string  `json:"symbol"`
	TotalQty     int64   `json:"total_qty"`
	AvailableQty int64   `json:"available_qty"`
	LockedQty    int64   `json:"locked_qty"`
	MarketValue  float64 `json:"market_value"`
	Weight       float64 `json:"weight"`
	MarkPrice    float64 `json:"mark_price,omitempty"`
	CanSell      bool    `json:"can_sell"`
	HoldingDays  int     `json:"holding_days,omitempty"`
}

// TargetName is one desired weight row (from TargetPortfolio / rebalance target).
type TargetName struct {
	Symbol       string  `json:"symbol"`
	TargetWeight float64 `json:"target_weight"`
}

// TargetPortfolio is an optional weight book used to refine REDUCE target weight.
type TargetPortfolio struct {
	EquityRef float64      `json:"equity_ref,omitempty"`
	Names     []TargetName `json:"names,omitempty"`
}

// RiskOvershootFact is optional precomputed overshoot (or derived from PortfolioRiskSnapshot).
type RiskOvershootFact struct {
	NameOverCapRatio   *float64 `json:"name_over_cap_ratio,omitempty"`
	SectorOverCapRatio *float64 `json:"sector_over_cap_ratio,omitempty"`
	GrossHeadroom      *float64 `json:"gross_headroom,omitempty"`
	Severity           string   `json:"severity"` // none|mild|high
}

// Options controls suggest-only sell sizing.
type Options struct {
	Phase              string  // forced suggest_only
	RiskBoostEnabled   bool    // default false
	PreferWeightVsRatio string // min|max|weight_only|ratio_only；default min
	EpsilonWeight      float64
	DefaultReduceRatio float64 // when Decision has no fraction
	LotSize            int64
	MinSellQty         int64 // 0 → LotSize
	MinSellNotional    float64
}

// DefaultOptions is production-safe suggest-only policy.
func DefaultOptions() Options {
	return Options{
		Phase:               PhaseSuggestOnly,
		RiskBoostEnabled:    false,
		PreferWeightVsRatio: PreferMin,
		EpsilonWeight:       DefaultEpsilonWeight,
		DefaultReduceRatio:  DefaultReduceFraction,
		LotSize:             DefaultLotSize,
		MinSellQty:          0,
	}
}

// Caps records how suggested_sell_qty was bounded.
type Caps struct {
	AvailableQtyCap int64 `json:"available_qty_cap"`
	IntentQtyCap    int64 `json:"intent_qty_cap"`
	RiskBoostQty    int64 `json:"risk_boost_qty,omitempty"`
	LotRounded      int64 `json:"lot_rounded"`
}

// SellAllocation is the H1.3 suggest-only output (not an order).
type SellAllocation struct {
	SchemaVersion    string `json:"schema_version"`
	Phase            string `json:"phase"`
	SuggestOnly      bool   `json:"suggest_only"`
	Symbol           string `json:"symbol"`
	Action           string `json:"action"` // REDUCE|EXIT|"" when skipped HOLD
	CurrentQty       int64  `json:"current_qty"`
	AvailableQty     int64  `json:"available_qty"`
	SuggestedSellQty int64  `json:"suggested_sell_qty"`
	CurrentWeight    float64 `json:"current_weight"`
	TargetWeight     float64 `json:"target_weight"`
	Reason           string  `json:"reason"`
	ReasonCodes      []string `json:"reason_codes,omitempty"`

	SuggestedSellNotional         float64  `json:"suggested_sell_notional,omitempty"`
	SuggestedReduceRatioEffective float64  `json:"suggested_reduce_ratio_effective,omitempty"`
	TargetPositionWeightAfter     *float64 `json:"target_position_weight_after,omitempty"`
	Caps                          Caps     `json:"caps"`
	Binding                       string   `json:"binding"`
	SkippedReason                 string   `json:"skipped_reason,omitempty"`
	ExecutableHint                bool     `json:"executable_hint"`

	RecordOnly       bool `json:"record_only"`
	NotAnOrder       bool `json:"not_an_order"`
	NotExecution     bool `json:"not_execution"`
	NotTradePlan     bool `json:"not_trade_plan"`
	NotBuyChain      bool `json:"not_buy_chain"`
	PersistSellPlans bool `json:"persist_sell_plans"`
}

// Input is the full Allocate bundle.
type Input struct {
	Intent        SellIntent
	Position      CurrentPosition
	PortfolioRisk *portfoliorisk.PortfolioRiskSnapshot
	Target        *TargetPortfolio // optional; refines REDUCE target weight
	Overshoot     *RiskOvershootFact
	Equity        float64
	Options       Options
}
