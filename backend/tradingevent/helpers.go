package tradingevent

import (
	"strings"
	"time"
)

// Automation outcomes → v0 event types (SUCCESS / FAILED / BLOCKED only).
// Skipped modes do not emit.

const (
	outcomeMaterializeSuccess = "MATERIALIZATION_AUTO_SUCCESS"
	outcomeMaterializeFailed  = "MATERIALIZATION_FAILED"
	outcomeApproveSuccess     = "AUTO_APPROVAL_SUCCESS"
	outcomeApproveBlocked     = "AUTO_APPROVAL_BLOCKED"
	outcomeFreezeSuccess      = "AUTO_FREEZE_SUCCESS"
	outcomeFreezeFailed       = "AUTO_FREEZE_FAILED"
	outcomeFreezeAfterDeadline = "AUTO_FREEZE_AFTER_DEADLINE"
)

// EmitAutomationStep maps a tradingautomation StepResult-shaped outcome into a TradingEvent.
// Unknown / skipped outcomes are ignored (no emit).
// Position new/old semantics are NOT derived here — consumers use portfolio/positionstate.
func EmitAutomationStep(tradeDate string, now time.Time, step, outcome, reason string, planID uint, ok bool) {
	eventType, phase, status, emit := mapAutomationOutcome(outcome, ok)
	if !emit {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = now.Format("2006-01-02")
	}
	Emit(TradingEvent{
		Timestamp: now,
		TradeDate: tradeDate,
		Phase:     phase,
		EventType: eventType,
		Status:    status,
		Reason:    strings.TrimSpace(reason),
		PlanID:    planID,
		Source:    SourceAutomation,
	})
	_ = step // reserved for future correlation; step already encoded in event_type
}

func mapAutomationOutcome(outcome string, ok bool) (eventType, phase, status string, emit bool) {
	switch strings.TrimSpace(outcome) {
	case outcomeMaterializeSuccess:
		return EventMaterializeSuccess, PhaseMaterialization, StatusPass, true
	case outcomeMaterializeFailed:
		return EventMaterializeFailed, PhaseMaterialization, StatusFail, true
	case outcomeApproveSuccess:
		return EventApproveSuccess, PhaseApproval, StatusPass, true
	case outcomeApproveBlocked:
		return EventApproveBlocked, PhaseApproval, StatusFail, true
	case outcomeFreezeSuccess:
		return EventFreezeSuccess, PhaseFreeze, StatusPass, true
	case outcomeFreezeFailed, outcomeFreezeAfterDeadline:
		return EventFreezeBlocked, PhaseFreeze, StatusFail, true
	default:
		if !ok && outcome != "" {
			// defensive: unexpected failure-shaped codes still surface as FAIL under nearest phase
			return "", "", "", false
		}
		return "", "", "", false
	}
}

// EmitExecution publishes an EXECUTION_* event. plan_id and reason are required by contract
// (reason may be empty string only when truly unknown; callers should still set a stable code).
func EmitExecution(eventType, tradeDate, session, status, reason string, planID uint, now time.Time, executionID string) {
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = now.Format("2006-01-02")
	}
	if status == "" {
		status = StatusPass
	}
	Emit(TradingEvent{
		Timestamp:   now,
		TradeDate:   tradeDate,
		Phase:       PhaseExecution,
		Session:     session,
		EventType:   eventType,
		Status:      status,
		Reason:      strings.TrimSpace(reason),
		PlanID:      planID,
		ExecutionID: strings.TrimSpace(executionID),
		Source:      SourceGateway,
	})
}

// EmitSettlement publishes SETTLEMENT_COMPLETED or SETTLEMENT_FAILED.
func EmitSettlement(eventType, tradeDate, status, reason string, accountID uint, now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = now.Format("2006-01-02")
	}
	if status == "" {
		if eventType == EventSettlementFailed {
			status = StatusFail
		} else {
			status = StatusPass
		}
	}
	Emit(TradingEvent{
		Timestamp: now,
		TradeDate: tradeDate,
		Phase:     PhaseSettlement,
		EventType: eventType,
		Status:    status,
		Reason:    strings.TrimSpace(reason),
		AccountID: accountID,
		Source:    SourceSettlement,
	})
}
