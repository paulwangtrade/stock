package tradingwindow

// Standard miss / window reason codes for Observation, UI, and logs.
const (
	ReasonPlanNotFrozenBeforeOpen = "PLAN_NOT_FROZEN_BEFORE_OPEN"
	ReasonPlanFrozenAfterDeadline = "PLAN_FROZEN_AFTER_DEADLINE"
	ReasonNoExecutionWindow       = "NO_EXECUTION_WINDOW"
	ReasonManualLateExecution     = "MANUAL_LATE_EXECUTION"
)

// ReasonLabel returns a short human-readable label for UI display.
func ReasonLabel(reason string) string {
	switch reason {
	case ReasonPlanNotFrozenBeforeOpen:
		return "Plan not frozen before open deadline"
	case ReasonPlanFrozenAfterDeadline:
		return "Plan frozen after deadline"
	case ReasonNoExecutionWindow:
		return "No execution window available"
	case ReasonManualLateExecution:
		return "Manual late execution"
	default:
		return ""
	}
}
