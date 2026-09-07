package tradingevent

import "time"

// Phase is the workflow stage (not Fill session).
const (
	PhasePreparation     = "PREPARATION"
	PhaseMaterialization = "MATERIALIZATION"
	PhaseApproval        = "APPROVAL"
	PhaseFreeze          = "FREEZE"
	PhaseExecution       = "EXECUTION"
	PhaseSettlement      = "SETTLEMENT"
	PhaseSystem          = "SYSTEM"
)

// Status is normalized event outcome.
const (
	StatusPass    = "PASS"
	StatusFail    = "FAIL"
	StatusSkip    = "SKIP"
	StatusPending = "PENDING"
)

// Source identifies the emitter.
const (
	SourceAutomation = "automation"
	SourceGateway    = "gateway"
	SourceSettlement = "settlement"
	SourceSystem     = "system"
)

// Event types — Observation v0 minimal set (implementation approval names).
const (
	EventMaterializeSuccess = "MATERIALIZE_SUCCESS"
	EventMaterializeFailed  = "MATERIALIZE_FAILED"
	EventApproveSuccess     = "APPROVE_SUCCESS"
	EventApproveBlocked     = "APPROVE_BLOCKED"
	EventFreezeSuccess      = "FREEZE_SUCCESS"
	EventFreezeBlocked      = "FREEZE_BLOCKED"

	EventExecutionStarted   = "EXECUTION_STARTED"
	EventExecutionSkipped   = "EXECUTION_SKIPPED"
	EventExecutionCompleted = "EXECUTION_COMPLETED"
	EventExecutionFailed    = "EXECUTION_FAILED"

	EventSettlementCompleted = "SETTLEMENT_COMPLETED"
	EventSettlementFailed    = "SETTLEMENT_FAILED"
)

// TradingEvent is the unified observation envelope (v0: structured log only).
type TradingEvent struct {
	EventID        string    `json:"event_id"`
	Timestamp      time.Time `json:"timestamp"`
	TradeDate      string    `json:"trade_date"`
	Phase          string    `json:"phase"`
	Session        string    `json:"session,omitempty"`
	EventType      string    `json:"event_type"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitempty"`
	ReasonDetail   string    `json:"reason_detail,omitempty"`
	PlanID         uint      `json:"plan_id,omitempty"`
	ExecutionID    string    `json:"execution_id,omitempty"`
	OrderID        uint      `json:"order_id,omitempty"`
	FillID         uint      `json:"fill_id,omitempty"`
	AccountID      uint      `json:"account_id,omitempty"`
	Source         string    `json:"source"`
	CorrelationID  string    `json:"correlation_id,omitempty"`
}
