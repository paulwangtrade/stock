// Holding Evaluation classifiers — read-only labels (Phase10-D.1.5).
// profit_state / holding_period_state are evaluation tags only (no BUY/SELL/EXIT).

package papertrading

import "math"

// ProfitState — direction of unrealized return (not a trade action).
const (
	ProfitStateProfit     = "PROFIT"
	ProfitStateBreakeven  = "BREAKEVEN"
	ProfitStateLoss       = "LOSS"
	ProfitStateUnknown    = "UNKNOWN"
)

// HoldingPeriodState — tenure band (not EXIT / not sell).
const (
	HoldingPeriodShort = "SHORT_TERM"
	HoldingPeriodMid   = "MID_TERM"
	HoldingPeriodLong  = "LONG_TERM"
)

// HoldingPeriodThresholds (calendar days, inclusive upper for short/mid).
type HoldingPeriodThresholds struct {
	ShortMaxDays int // default 5 → SHORT_TERM when days <= ShortMaxDays
	MidMaxDays   int // default 20 → MID_TERM when days <= MidMaxDays; else LONG_TERM
}

// DefaultHoldingPeriodThresholds aligns with ExitPolicy default_v1 max_holding_days=20.
var DefaultHoldingPeriodThresholds = HoldingPeriodThresholds{
	ShortMaxDays: 5,
	MidMaxDays:   20,
}

// ProfitStateEpsilon: |return| within this → BREAKEVEN (ratio, not percent).
const ProfitStateEpsilon = 0.001 // 0.1%

// ClassifyProfitState maps unrealized return ratio → profit_state.
// nil / NaN / Inf → UNKNOWN (no invented P&L).
func ClassifyProfitState(unrealizedReturnRatio *float64) string {
	if unrealizedReturnRatio == nil {
		return ProfitStateUnknown
	}
	r := *unrealizedReturnRatio
	if math.IsNaN(r) || math.IsInf(r, 0) {
		return ProfitStateUnknown
	}
	if r > ProfitStateEpsilon {
		return ProfitStateProfit
	}
	if r < -ProfitStateEpsilon {
		return ProfitStateLoss
	}
	return ProfitStateBreakeven
}

// ClassifyHoldingPeriodState maps holding_days → period band.
func ClassifyHoldingPeriodState(holdingDays int, th HoldingPeriodThresholds) string {
	if th.ShortMaxDays <= 0 {
		th.ShortMaxDays = DefaultHoldingPeriodThresholds.ShortMaxDays
	}
	if th.MidMaxDays <= 0 {
		th.MidMaxDays = DefaultHoldingPeriodThresholds.MidMaxDays
	}
	if th.MidMaxDays < th.ShortMaxDays {
		th.MidMaxDays = th.ShortMaxDays
	}
	if holdingDays < 0 {
		holdingDays = 0
	}
	if holdingDays <= th.ShortMaxDays {
		return HoldingPeriodShort
	}
	if holdingDays <= th.MidMaxDays {
		return HoldingPeriodMid
	}
	return HoldingPeriodLong
}
