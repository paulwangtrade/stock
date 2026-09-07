package models

import "fmt"

// Phase6-A6.1 / Phase10-D.0.1 Execution readiness guard reason codes.
const (
	// ReasonPlanNotFrozen is returned when a TradePlan is not frozen-ready.
	ReasonPlanNotFrozen = "PLAN_NOT_FROZEN"
	// ReasonPlanNotApproved is returned when FreezeAt is set (or status looks ready)
	// but ApprovedAt is missing — INV-P-RDY-01 (Phase10-D.0).
	ReasonPlanNotApproved = "PLAN_NOT_APPROVED"
)

// FrozenReadyGuardResult is the outcome of RequireFrozenReadyTradePlan.
type FrozenReadyGuardResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"` // empty when Allowed
	Message string `json:"message,omitempty"`
}

// RequireFrozenReadyTradePlan is the Execution readiness guard (Phase6-A6.1 + Phase10-D.0.1).
//
// Allow (INV-P-RDY-01): status=ready AND FreezeAt != nil AND ApprovedAt != nil.
// Block:
//   - naked ready (FreezeAt=nil) → PLAN_NOT_FROZEN
//   - ready+freeze without approve → PLAN_NOT_APPROVED
//   - draft / other → PLAN_NOT_FROZEN
//
// It does not mutate the plan, call Execution, or rewrite historical rows.
func RequireFrozenReadyTradePlan(plan *TradePlan) FrozenReadyGuardResult {
	if plan == nil {
		return FrozenReadyGuardResult{
			Allowed: false,
			Reason:  ReasonPlanNotFrozen,
			Message: "trade plan is nil",
		}
	}
	if !plan.IsFrozen() {
		return FrozenReadyGuardResult{
			Allowed: false,
			Reason:  ReasonPlanNotFrozen,
			Message: fmt.Sprintf(
				"planId=%d status=%s freezeAt_nil=%v (INV-P-RDY-01: ready requires FreezeAt)",
				plan.ID, plan.Status, plan.FreezeAt == nil || plan.FreezeAt.IsZero(),
			),
		}
	}
	if plan.ApprovedAt == nil || plan.ApprovedAt.IsZero() {
		return FrozenReadyGuardResult{
			Allowed: false,
			Reason:  ReasonPlanNotApproved,
			Message: fmt.Sprintf(
				"planId=%d status=%s freezeAt_set=true approvedAt_nil=true (INV-P-RDY-01: ready requires ApprovedAt)",
				plan.ID, plan.Status,
			),
		}
	}
	return FrozenReadyGuardResult{Allowed: true}
}
