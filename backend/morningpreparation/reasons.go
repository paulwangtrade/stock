package morningpreparation

// Morning readiness reason codes (Observation / UI / logs).
const (
	ReasonPlanNotCreated         = "PLAN_NOT_CREATED"
	ReasonPlanNotFrozen          = "PLAN_NOT_FROZEN"
	ReasonFreezeAfterDeadline    = "FREEZE_AFTER_DEADLINE"
	ReasonMissingOpenReadiness   = "MISSING_OPEN_READINESS"
	ReasonMaterializationPending = "MATERIALIZATION_PENDING"
	ReasonMaterializationFailed  = "MATERIALIZATION_FAILED"
)

// ReasonLabel returns a short human-readable label for UI display.
func ReasonLabel(reason string) string {
	switch reason {
	case ReasonPlanNotCreated:
		return "No trade plan for today"
	case ReasonPlanNotFrozen:
		return "Plan not frozen before open deadline"
	case ReasonFreezeAfterDeadline:
		return "Plan frozen after open deadline"
	case ReasonMissingOpenReadiness:
		return "Missing open readiness at deadline checkpoint"
	case ReasonMaterializationPending:
		return "Morning materialization not completed"
	case ReasonMaterializationFailed:
		return "Morning materialization failed"
	default:
		return ""
	}
}
