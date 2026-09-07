package safetygate

import (
	"fmt"
	"strings"

	"go-stock/backend/models"
	"go-stock/backend/tradingrule"
)

// Phase6.5.7.4.1 Broker Submit Safety Gate (pure validation).
// Does not call Broker, mutate Spec, or change order lifecycle.
// Phase12-M2.5-B: buy volume lot check is Flag-aware (QuantityPolicy Validate-only).

const (
	// DefaultMinLot is the legacy A-share lot (100). Used when EnableQuantityPolicy is false.
	// Retained for Flag OFF behavior and Context.MinLot defaults — not used for Flag ON buy Validate.
	DefaultMinLot int64 = 100

	BlockPlanNotFrozen         = "PLAN_NOT_FROZEN"
	BlockSpecSymbolMissing     = "SPEC_SYMBOL_MISSING"
	BlockSpecSideInvalid       = "SPEC_SIDE_INVALID"
	BlockSpecPriceInvalid      = "SPEC_PRICE_INVALID"
	BlockSpecVolumeInvalid     = "SPEC_VOLUME_INVALID"
	BlockRuntimePriceOverride  = "RUNTIME_PRICE_OVERRIDE"
	BlockRuntimeVolumeOverride = "RUNTIME_VOLUME_OVERRIDE"

	WarnNoPlanContext = "WARN_NO_PLAN_CONTEXT"
)

// Spec is the Frozen Order Spec snapshot consumed at submit time (read-only).
type Spec struct {
	Symbol       string
	Side         string
	LimitPrice   float64
	TargetVolume int64
}

// Context carries proposed submit values and optional plan for freeze check.
type Context struct {
	Plan *models.TradePlan
	// SubmitPrice / SubmitVolume are the values about to be sent to Broker.
	SubmitPrice  float64
	SubmitVolume int64
	MinLot       int64 // default 100
	// SkipFrozenCheck: Manual/Research / unit paths without a TradePlan.
	// TradePlan open-buy should pass Plan and leave this false.
	SkipFrozenCheck bool
}

// Result is the gate outcome.
type Result struct {
	Allowed  bool     `json:"allowed"`
	Blockers []string `json:"blockers,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// SpecFromItem maps a TradePlanItem to Spec (does not mutate item).
func SpecFromItem(item models.TradePlanItem) Spec {
	side := strings.TrimSpace(item.Side)
	if side == "" {
		side = "buy"
	}
	return Spec{
		Symbol:       strings.TrimSpace(item.StockCode),
		Side:         strings.ToLower(side),
		LimitPrice:   item.LimitPrice,
		TargetVolume: item.TargetVolume,
	}
}

// ValidateBrokerSubmit checks Frozen + Spec integrity + no runtime price/volume override.
// Does not call Broker or write any state.
func ValidateBrokerSubmit(spec Spec, ctx Context) Result {
	var blockers, warnings []string
	minLot := ctx.MinLot
	if minLot <= 0 {
		minLot = DefaultMinLot
	}

	// 1. Frozen status
	if ctx.SkipFrozenCheck {
		if ctx.Plan == nil {
			warnings = append(warnings, WarnNoPlanContext)
		}
	} else {
		guard := models.RequireFrozenReadyTradePlan(ctx.Plan)
		if !guard.Allowed {
			reason := guard.Reason
			if reason == "" {
				reason = BlockPlanNotFrozen
			}
			blockers = append(blockers, reason)
		}
	}

	// 2. Spec integrity: symbol / side
	if strings.TrimSpace(spec.Symbol) == "" {
		blockers = append(blockers, BlockSpecSymbolMissing)
	}
	side := strings.ToLower(strings.TrimSpace(spec.Side))
	if side == "" {
		side = "buy"
	}
	if side != "buy" && side != "sell" {
		blockers = append(blockers, BlockSpecSideInvalid)
	}

	// 3. Price
	if spec.LimitPrice <= 0 {
		blockers = append(blockers, BlockSpecPriceInvalid)
	}

	// 4. Volume — validate only; never normalize / rewrite Spec.
	if !volumeSpecAllowed(side, spec.Symbol, spec.TargetVolume, minLot) {
		blockers = append(blockers, BlockSpecVolumeInvalid)
	}

	// 5. No runtime override of price/volume vs Frozen Spec
	if spec.LimitPrice > 0 && ctx.SubmitPrice > 0 {
		if !almostEqual(ctx.SubmitPrice, spec.LimitPrice) {
			blockers = append(blockers, fmt.Sprintf("%s: submit=%.6f spec=%.6f", BlockRuntimePriceOverride, ctx.SubmitPrice, spec.LimitPrice))
		}
	} else if spec.LimitPrice > 0 && ctx.SubmitPrice <= 0 {
		blockers = append(blockers, BlockRuntimePriceOverride+": submit price missing")
	}
	if spec.TargetVolume > 0 && ctx.SubmitVolume > 0 {
		if ctx.SubmitVolume != spec.TargetVolume {
			blockers = append(blockers, fmt.Sprintf("%s: submit=%d spec=%d", BlockRuntimeVolumeOverride, ctx.SubmitVolume, spec.TargetVolume))
		}
	} else if spec.TargetVolume > 0 && ctx.SubmitVolume <= 0 {
		blockers = append(blockers, BlockRuntimeVolumeOverride+": submit volume missing")
	}

	return Result{
		Allowed:  len(blockers) == 0,
		Blockers: blockers,
		Warnings: warnings,
	}
}

// volumeSpecAllowed checks TargetVolume legality without mutating Spec.
//
//	Flag OFF (or sell): legacy minLot + multiple-of-minLot (DefaultMinLot=100)
//	Flag ON + buy: QuantityPolicy ValidateBuyQuantity only (no Normalize)
func volumeSpecAllowed(side, symbol string, volume, minLot int64) bool {
	if side == "sell" || !tradingrule.EnableQuantityPolicy() {
		return volume >= minLot && volume%minLot == 0
	}
	meta := tradingrule.MetaFromStockCode(symbol)
	return tradingrule.ValidateBuyQuantity(meta, volume).Accepted
}

func almostEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

// Error formats a blocked result as an error for ExecutionService.
func (r Result) Error() error {
	if r.Allowed {
		return nil
	}
	return &GateError{Blockers: append([]string(nil), r.Blockers...)}
}

// GateError is returned when ValidateBrokerSubmit blocks submit.
type GateError struct {
	Blockers []string
}

func (e *GateError) Error() string {
	if e == nil || len(e.Blockers) == 0 {
		return "safetygate: blocked"
	}
	return "safetygate: blocked: " + strings.Join(e.Blockers, "; ")
}

// IsGateError reports whether err is a Safety Gate block.
func IsGateError(err error) bool {
	_, ok := err.(*GateError)
	return ok
}
