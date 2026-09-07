package tradingautomation

// Step outcome codes for observation / logs.
const (
	OutcomeMaterializationAutoSuccess = "MATERIALIZATION_AUTO_SUCCESS"
	OutcomeMaterializationFailed      = "MATERIALIZATION_FAILED"
	OutcomeMaterializationSkipped     = "MATERIALIZATION_SKIPPED"
	OutcomeAutoApprovalSuccess        = "AUTO_APPROVAL_SUCCESS"
	OutcomeAutoApprovalBlocked        = "AUTO_APPROVAL_BLOCKED"
	OutcomeAutoApprovalSkipped        = "AUTO_APPROVAL_SKIPPED"
	OutcomeAutoFreezeSuccess          = "AUTO_FREEZE_SUCCESS"
	OutcomeAutoFreezeFailed           = "AUTO_FREEZE_FAILED"
	OutcomeAutoFreezeSkipped          = "AUTO_FREEZE_SKIPPED"
	OutcomeAutoFreezeAfterDeadline    = "AUTO_FREEZE_AFTER_DEADLINE"
)

// Step status for UI (PASS / FAIL / SKIP / PENDING).
const (
	StepPass    = "PASS"
	StepFail    = "FAIL"
	StepSkip    = "SKIP"
	StepPending = "PENDING"
)

// OutcomeLabel returns human-readable text for UI.
func OutcomeLabel(code string) string {
	switch code {
	case OutcomeMaterializationAutoSuccess:
		return "Auto materialization succeeded"
	case OutcomeMaterializationFailed:
		return "Auto materialization failed"
	case OutcomeMaterializationSkipped:
		return "Materialization skipped (manual mode)"
	case OutcomeAutoApprovalSuccess:
		return "Auto approval succeeded"
	case OutcomeAutoApprovalBlocked:
		return "Auto approval blocked"
	case OutcomeAutoApprovalSkipped:
		return "Auto approval skipped"
	case OutcomeAutoFreezeSuccess:
		return "Auto freeze succeeded"
	case OutcomeAutoFreezeFailed:
		return "Auto freeze failed"
	case OutcomeAutoFreezeSkipped:
		return "Auto freeze skipped"
	case OutcomeAutoFreezeAfterDeadline:
		return "Auto freeze after open deadline"
	default:
		return code
	}
}
