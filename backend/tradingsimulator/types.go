package tradingsimulator

import "time"

const (
	ScenarioNormal     = "NORMAL"
	ScenarioNoPlan     = "NO_PLAN"
	ScenarioLateFreeze = "LATE_FREEZE"
	ScenarioRiskBlock  = "RISK_BLOCK"
	ScenarioManualMode = "MANUAL_MODE"
)

const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusSkip = "SKIP"
)

const (
	EventAutoMaterializeSuccess = "AUTO_MATERIALIZE_SUCCESS"
	EventMaterializationFailed  = "MATERIALIZATION_FAILED"
	EventMaterializationSkipped = "MATERIALIZATION_SKIPPED"
	EventAutoApproveSuccess     = "AUTO_APPROVE_SUCCESS"
	EventAutoApprovalBlocked    = "AUTO_APPROVAL_BLOCKED"
	EventAutoApprovalSkipped    = "AUTO_APPROVAL_SKIPPED"
	EventAutoFreezeSuccess      = "AUTO_FREEZE_SUCCESS"
	EventAutoFreezeFailed       = "AUTO_FREEZE_FAILED"
	EventAutoFreezeAfterDeadline = "AUTO_FREEZE_AFTER_DEADLINE"
	EventMissedOpenWindow       = "MISSED_OPEN_WINDOW"
	EventExecutionSuccess       = "EXECUTION_SUCCESS"
	EventExecutionSkipped       = "EXECUTION_SKIPPED"
	EventSettlementSuccess      = "SETTLEMENT_SUCCESS"
	EventSettlementSkipped      = "SETTLEMENT_SKIPPED"
)

type SimulationRequest struct {
	Date           string
	AutomationMode string
	Scenario       string
}

type SimulationEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Session   string    `json:"session"`
	Event     string    `json:"event"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	PlanID    uint      `json:"plan_id,omitempty"`
}

type StepScore struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type SimulationResult struct {
	Date          string            `json:"date"`
	Scenario      string            `json:"scenario"`
	Mode          string            `json:"mode"`
	Timeline      []SimulationEvent `json:"timeline"`
	Morning       StepScore         `json:"morning"`
	Approve       StepScore         `json:"approve"`
	Freeze        StepScore         `json:"freeze"`
	Execution     StepScore         `json:"execution"`
	Settlement    StepScore         `json:"settlement"`
	WindowStatus  string            `json:"window_status,omitempty"`
	WindowReason  string            `json:"window_reason,omitempty"`
	Orders        int               `json:"orders"`
	Fills         int               `json:"fills"`
	FailureReason string            `json:"failure_reason,omitempty"`
	Overall       string            `json:"overall"`
}
