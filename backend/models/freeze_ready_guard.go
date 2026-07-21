package models

import "fmt"

// Phase6-A6.1 Execution Freeze Guard reason codes.
// Guard is standalone: not yet wired into Prepare/Buy (A6.2+).
const (
	// ReasonPlanNotFrozen is returned when a TradePlan is not frozen-ready.
	ReasonPlanNotFrozen = "PLAN_NOT_FROZEN"
)

// FrozenReadyGuardResult is the outcome of RequireFrozenReadyTradePlan.
type FrozenReadyGuardResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"` // empty when Allowed; else PLAN_NOT_FROZEN
	Message string `json:"message,omitempty"`
}

// RequireFrozenReadyTradePlan is the Phase6-A6.1 Execution Freeze Guard.
//
// Allow:  status=ready AND FreezeAt != nil (and not zero) — i.e. IsFrozen().
// Block:  naked ready (FreezeAt=nil), draft, failed, and any other non-frozen plan.
// Reason: always PLAN_NOT_FROZEN when blocked.
//
// It does not mutate the plan, call Execution, touch cron, or change status enums.
func RequireFrozenReadyTradePlan(plan *TradePlan) FrozenReadyGuardResult {
	if plan == nil {
		return FrozenReadyGuardResult{
			Allowed: false,
			Reason:  ReasonPlanNotFrozen,
			Message: "trade plan is nil",
		}
	}
	if plan.IsFrozen() {
		return FrozenReadyGuardResult{Allowed: true}
	}
	return FrozenReadyGuardResult{
		Allowed: false,
		Reason:  ReasonPlanNotFrozen,
		Message: fmt.Sprintf("planId=%d status=%s freezeAt_nil=%v", plan.ID, plan.Status, plan.FreezeAt == nil || plan.FreezeAt.IsZero()),
	}
}
