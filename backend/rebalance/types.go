// Package rebalance provides read-only Rebalance Diff observation (Phase10-D.12).
//
// Diff(Current, Target) → RebalanceDiff. No BUY/SELL intents, orders, Gateway, or TradePlan writes.
package rebalance

import "time"

// Diff actions — portfolio gap labels, never BUY/SELL.
const (
	ActionKeep      = "KEEP"
	ActionAdd       = "ADD"
	ActionIncrease  = "INCREASE"
	ActionDecrease  = "DECREASE"
	ActionRemove    = "REMOVE"
)

// Constraint / reason codes (observation only).
const (
	ReasonWithinBand          = "WITHIN_BAND"
	ReasonNewTarget           = "NEW_TARGET"
	ReasonBelowTarget         = "BELOW_TARGET"
	ReasonAboveTarget         = "ABOVE_TARGET"
	ReasonNotInTarget         = "NOT_IN_TARGET"
	ReasonSwitchBlocked       = "SWITCH_BLOCKED_DEFAULT"
	ReasonHoldNormalBlocks    = "HOLD_NORMAL_BLOCKS_SWITCH"
	ConstraintT1Locked        = "t1_locked"
	ConstraintInsufficientAvail = "insufficient_available"
	ConstraintPendingBuy      = "pending_buy_overlap"
)

const (
	PriceBasisSnapshotMark = "snapshot_mark"
	dataSourceNote         = "Rebalance Diff · read-only Current vs Target; observation only; not a trade recommendation; no BUY/SELL intents"
)

// DiffPolicy controls band and switch. AllowSwitch default false; switch_ok stays false without cost model.
type DiffPolicy struct {
	WeightBand     float64 // |Δw| at or below → KEEP (default 0.02)
	MinDeltaAmount float64 // optional absolute amount band; 0 disables
	AllowSwitch    bool    // default false
}

// DefaultPolicy is production-safe observation policy.
func DefaultPolicy() DiffPolicy {
	return DiffPolicy{WeightBand: 0.02, AllowSwitch: false}
}

// CurrentPosition is one holding in the current book.
type CurrentPosition struct {
	Symbol           string  `json:"symbol"`
	Volume           int64   `json:"volume"`
	AvailableVolume  int64   `json:"available_volume"`
	LockedVolume     int64   `json:"locked_volume"`
	MarketValue      float64 `json:"market_value"`
	Weight           float64 `json:"weight"`
	DecisionState    string  `json:"decision_state,omitempty"`
}

// CurrentView is the left side of Diff (from PortfolioSnapshot).
type CurrentView struct {
	AsOf               time.Time         `json:"as_of"`
	AccountID          uint              `json:"account_id,omitempty"`
	Equity             float64           `json:"equity"`
	Cash               float64           `json:"cash"`
	Exposure           float64           `json:"exposure"`
	PriceBasis         string            `json:"price_basis"`
	PendingBuyNotional float64           `json:"pending_buy_notional,omitempty"`
	Positions          []CurrentPosition `json:"positions"`
}

// TargetPosition is one desired holding (D.10 subset).
type TargetPosition struct {
	Symbol        string  `json:"symbol"`
	TargetWeight  float64 `json:"target_weight"`
	TargetAmount  float64 `json:"target_amount"`
	Priority      int     `json:"priority,omitempty"`
	Reason        string  `json:"reason,omitempty"`
	Source        string  `json:"source,omitempty"`
}

// TargetPortfolio is the right side of Diff.
type TargetPortfolio struct {
	AsOf              time.Time        `json:"as_of,omitempty"`
	AccountID         uint             `json:"account_id,omitempty"`
	EquityRef         float64          `json:"equity_ref"`
	CashBufferWeight  float64          `json:"cash_buffer_weight,omitempty"`
	Positions         []TargetPosition `json:"positions"`
	ConstructionNote  string           `json:"construction_note,omitempty"`
}

// Item is one symbol gap.
type Item struct {
	Symbol            string   `json:"symbol"`
	CurrentWeight     float64  `json:"current_weight"`
	TargetWeight      float64  `json:"target_weight"`
	DeltaWeight       float64  `json:"delta_weight"`
	CurrentAmount     float64  `json:"current_amount"`
	TargetAmount      float64  `json:"target_amount"`
	DeltaAmount       float64  `json:"delta_amount"`
	Action            string   `json:"action"`
	Reason            string   `json:"reason"`
	AvailableVolume   int64    `json:"available_volume"`
	LockedVolume      int64    `json:"locked_volume"`
	ExecutableQty     int64    `json:"executable_qty"`
	ConstraintFlags   []string `json:"constraint_flags,omitempty"`
	SwitchGroupID     string   `json:"switch_group_id,omitempty"`
	SwitchOK          bool     `json:"switch_ok"`
	SwitchReason      string   `json:"switch_reason,omitempty"`
	IntentHint        string   `json:"intent_hint"` // always "none" in D.12
	DecisionState     string   `json:"decision_state,omitempty"`
}

// ActionCounts is the portfolio histogram.
type ActionCounts struct {
	Keep      int `json:"keep_count"`
	Add       int `json:"add_count"`
	Increase  int `json:"increase_count"`
	Decrease  int `json:"decrease_count"`
	Remove    int `json:"remove_count"`
	Total     int `json:"total_positions"`
}

// View is the observation payload.
type View struct {
	AsOf               string       `json:"as_of,omitempty"`
	AccountID          uint         `json:"account_id,omitempty"`
	PriceBasis         string       `json:"price_basis"`
	EquityRef          float64      `json:"equity_ref"`
	WeightBand         float64      `json:"weight_band"`
	AllowSwitch        bool         `json:"allow_switch"`
	Counts             ActionCounts `json:"counts"`
	BlockedSwitchCount int          `json:"blocked_switch_count"`
	Items              []Item       `json:"items"`
	DataSourceNote     string       `json:"data_source_note"`
}
