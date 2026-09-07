package morningpreparation

import (
	"strings"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/tradingwindow"
)

// Morning readiness overall status.
const (
	MorningStatusReady          = "READY"
	MorningStatusNotReady       = "NOT_READY"
	MorningStatusMissedDeadline = "MISSED_DEADLINE"
)

// Per-dimension check labels for UI.
const (
	CheckPass          = "PASS"
	CheckFail          = "FAIL"
	CheckPending       = "PENDING"
	CheckNotApplicable = "NOT_APPLICABLE"
)

const morningPricingStage = "morning_materialized"

// MorningReadinessInput drives derived morning-flow observation.
type MorningReadinessInput struct {
	TradeDate                      string
	Now                            time.Time
	Plan                           *models.TradePlan
	MaterializeAttempted           bool
	MaterializeSuccess             bool
	DeadlineCheckpoint             bool // 09:25: emit MISSING_OPEN_READINESS when not frozen
}

// MorningReadinessObservation is a derived snapshot; not persisted.
type MorningReadinessObservation struct {
	TradeDate             string `json:"tradeDate"`
	PlanID                uint   `json:"planId,omitempty"`
	Status                string `json:"status"`
	MaterializationStatus string `json:"materializationStatus"`
	FreezeStatus          string `json:"freezeStatus"`
	DeadlineStatus        string `json:"deadlineStatus"`
	Reason                string `json:"reason"`
	WindowStatus          string `json:"windowStatus"`
	WindowReason          string `json:"windowReason"`
}

// EvaluateMorningReadiness derives morning preparation state from plan facts + PlanWindowPolicy.
func EvaluateMorningReadiness(in MorningReadinessInput) MorningReadinessObservation {
	out := MorningReadinessObservation{
		TradeDate: strings.TrimSpace(in.TradeDate),
		Status:    MorningStatusNotReady,
	}
	if out.TradeDate == "" && in.Now.IsZero() {
		out.Reason = ReasonPlanNotCreated
		out.MaterializationStatus = CheckFail
		out.FreezeStatus = CheckFail
		out.DeadlineStatus = MorningStatusNotReady
		return out
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	if out.TradeDate == "" {
		out.TradeDate = now.Format("2006-01-02")
	}

	plan := in.Plan
	if plan == nil {
		out.MaterializationStatus = CheckFail
		out.FreezeStatus = CheckFail
		out.DeadlineStatus = morningDeadlineStatus(now, out.TradeDate, false, nil)
		out.Reason = ReasonPlanNotCreated
		if out.DeadlineStatus == MorningStatusMissedDeadline {
			out.Status = MorningStatusMissedDeadline
		}
		return out
	}

	out.PlanID = plan.ID
	window := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   plan.TradeDate,
		CurrentTime: now,
		PlanStatus:  plan.Status,
		IsFrozen:    plan.IsFrozen(),
		FrozenTime:  plan.FreezeAt,
	})
	out.WindowStatus = string(window.Status)
	out.WindowReason = window.Reason

	out.FreezeStatus = freezeCheck(plan)
	out.MaterializationStatus = materializationCheck(plan, in.MaterializeAttempted, in.MaterializeSuccess)
	out.DeadlineStatus = morningDeadlineStatus(now, plan.TradeDate, plan.IsFrozen(), plan.FreezeAt)

	out.Reason = deriveMorningReason(plan, window, in.DeadlineCheckpoint, out.MaterializationStatus, out.FreezeStatus)
	out.Status = deriveMorningStatus(window, out.FreezeStatus, out.MaterializationStatus, out.Reason)
	return out
}

func deriveMorningStatus(
	window tradingwindow.PlanWindowResult,
	freezeStatus, materializationStatus, reason string,
) string {
	if reason == ReasonFreezeAfterDeadline || window.Status == tradingwindow.StatusMissedOpenWindow {
		if freezeStatus == CheckFail || reason == ReasonPlanNotFrozen || reason == ReasonMissingOpenReadiness {
			if window.Reason == tradingwindow.ReasonPlanNotFrozenBeforeOpen || reason == ReasonMissingOpenReadiness || reason == ReasonPlanNotFrozen {
				if window.BlocksAutoExecution {
					return MorningStatusMissedDeadline
				}
			}
		}
		if reason == ReasonFreezeAfterDeadline {
			return MorningStatusMissedDeadline
		}
	}
	if freezeStatus == CheckPass && (materializationStatus == CheckPass || materializationStatus == CheckNotApplicable) {
		if window.Status == tradingwindow.StatusReadyForOpen || window.Status == tradingwindow.StatusOpenExecutable {
			return MorningStatusReady
		}
	}
	if window.Status == tradingwindow.StatusExpired {
		return MorningStatusMissedDeadline
	}
	if window.BlocksAutoExecution && (reason == ReasonPlanNotFrozen || reason == ReasonMissingOpenReadiness || reason == ReasonPlanNotCreated) {
		return MorningStatusMissedDeadline
	}
	return MorningStatusNotReady
}

func deriveMorningReason(
	plan *models.TradePlan,
	window tradingwindow.PlanWindowResult,
	deadlineCheckpoint bool,
	materializationStatus, freezeStatus string,
) string {
	if plan == nil {
		return ReasonPlanNotCreated
	}
	if plan.IsFrozen() && window.Reason == tradingwindow.ReasonPlanFrozenAfterDeadline {
		return ReasonFreezeAfterDeadline
	}
	if !plan.IsFrozen() {
		if deadlineCheckpoint {
			return ReasonMissingOpenReadiness
		}
		if window.Reason == tradingwindow.ReasonPlanNotFrozenBeforeOpen && window.BlocksAutoExecution {
			return ReasonPlanNotFrozen
		}
		if materializationStatus == CheckFail {
			return ReasonMaterializationFailed
		}
		if materializationStatus == CheckPending {
			return ReasonMaterializationPending
		}
		return ReasonPlanNotFrozen
	}
	return ""
}

func freezeCheck(plan *models.TradePlan) string {
	if plan != nil && plan.IsFrozen() {
		return CheckPass
	}
	return CheckFail
}

func materializationCheck(plan *models.TradePlan, attempted, success bool) string {
	if plan == nil {
		return CheckFail
	}
	if plan.IsFrozen() {
		return CheckNotApplicable
	}
	if isMorningMaterialized(plan) {
		return CheckPass
	}
	if attempted {
		if success {
			return CheckPass
		}
		return CheckFail
	}
	if strings.TrimSpace(plan.PricingStage) == "" || plan.PricingStage == "after_close_intent" {
		return CheckPending
	}
	return CheckPending
}

func isMorningMaterialized(plan *models.TradePlan) bool {
	if plan == nil {
		return false
	}
	if strings.TrimSpace(plan.PricingStage) == morningPricingStage {
		return true
	}
	for _, it := range plan.Items {
		side := strings.ToLower(strings.TrimSpace(it.Side))
		if side != "" && side != "buy" {
			continue
		}
		if it.LimitPrice > 0 && it.TargetVolume > 0 {
			return true
		}
	}
	return false
}

func morningDeadlineStatus(now time.Time, tradeDate string, frozen bool, frozenAt *time.Time) string {
	window := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   tradeDate,
		CurrentTime: now,
		PlanStatus:  "",
		IsFrozen:    frozen,
		FrozenTime:  frozenAt,
	})
	switch window.Status {
	case tradingwindow.StatusMissedOpenWindow, tradingwindow.StatusExpired:
		return MorningStatusMissedDeadline
	case tradingwindow.StatusReadyForOpen, tradingwindow.StatusOpenExecutable:
		if frozen {
			return MorningStatusReady
		}
		return MorningStatusNotReady
	default:
		return MorningStatusNotReady
	}
}
