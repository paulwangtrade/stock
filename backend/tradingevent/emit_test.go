package tradingevent

import (
	"testing"
	"time"
)

func TestEmit_FieldsComplete(t *testing.T) {
	var got []TradingEvent
	SetSink(func(ev TradingEvent) { got = append(got, ev) })
	t.Cleanup(func() { SetSink(nil) })

	now := time.Date(2026, 8, 17, 9, 31, 0, 0, time.Local)
	Emit(TradingEvent{
		Timestamp: now,
		TradeDate: "2026-08-17",
		Phase:     PhaseExecution,
		Session:   "A",
		EventType: EventExecutionStarted,
		Status:    StatusPass,
		Reason:    "SESSION_ALLOW",
		PlanID:    42,
		Source:    SourceGateway,
	})
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d", len(got))
	}
	ev := got[0]
	if ev.EventID == "" {
		t.Fatal("event_id required")
	}
	if ev.Timestamp.IsZero() {
		t.Fatal("timestamp required")
	}
	if ev.TradeDate != "2026-08-17" {
		t.Fatalf("trade_date=%s", ev.TradeDate)
	}
	if ev.Phase != PhaseExecution {
		t.Fatalf("phase=%s", ev.Phase)
	}
	if ev.EventType != EventExecutionStarted {
		t.Fatalf("event_type=%s", ev.EventType)
	}
	if ev.Status != StatusPass {
		t.Fatalf("status=%s", ev.Status)
	}
	if ev.Source != SourceGateway {
		t.Fatalf("source=%s", ev.Source)
	}
	if ev.PlanID != 42 {
		t.Fatalf("plan_id=%d", ev.PlanID)
	}
	if ev.Reason != "SESSION_ALLOW" {
		t.Fatalf("reason=%s", ev.Reason)
	}
}

func TestEmitAutomationStep_EventTypes(t *testing.T) {
	var got []TradingEvent
	SetSink(func(ev TradingEvent) { got = append(got, ev) })
	t.Cleanup(func() { SetSink(nil) })

	now := time.Date(2026, 8, 17, 9, 20, 0, 0, time.Local)
	cases := []struct {
		outcome string
		ok      bool
		want    string
		status  string
	}{
		{outcomeMaterializeSuccess, true, EventMaterializeSuccess, StatusPass},
		{outcomeMaterializeFailed, false, EventMaterializeFailed, StatusFail},
		{outcomeApproveSuccess, true, EventApproveSuccess, StatusPass},
		{outcomeApproveBlocked, false, EventApproveBlocked, StatusFail},
		{outcomeFreezeSuccess, true, EventFreezeSuccess, StatusPass},
		{outcomeFreezeFailed, false, EventFreezeBlocked, StatusFail},
		{outcomeFreezeAfterDeadline, false, EventFreezeBlocked, StatusFail},
	}
	for _, tc := range cases {
		got = nil
		EmitAutomationStep("2026-08-17", now, "x", tc.outcome, "r", 7, tc.ok)
		if len(got) != 1 {
			t.Fatalf("outcome=%s: want 1 event, got %d", tc.outcome, len(got))
		}
		if got[0].EventType != tc.want {
			t.Fatalf("outcome=%s: event_type=%s want %s", tc.outcome, got[0].EventType, tc.want)
		}
		if got[0].Status != tc.status {
			t.Fatalf("outcome=%s: status=%s want %s", tc.outcome, got[0].Status, tc.status)
		}
		if got[0].PlanID != 7 {
			t.Fatalf("plan_id missing")
		}
		if got[0].Source != SourceAutomation {
			t.Fatalf("source=%s", got[0].Source)
		}
	}
}

func TestEmitAutomationStep_SkippedNoEmit(t *testing.T) {
	var got []TradingEvent
	SetSink(func(ev TradingEvent) { got = append(got, ev) })
	t.Cleanup(func() { SetSink(nil) })

	EmitAutomationStep("2026-08-17", time.Now(), "materialize", "MATERIALIZATION_SKIPPED", "mode=MANUAL", 0, true)
	EmitAutomationStep("2026-08-17", time.Now(), "approve", "AUTO_APPROVAL_SKIPPED", "mode=ASSISTED", 0, true)
	EmitAutomationStep("2026-08-17", time.Now(), "freeze", "AUTO_FREEZE_SKIPPED", "mode=ASSISTED", 0, true)
	if len(got) != 0 {
		t.Fatalf("skipped must not emit, got %d", len(got))
	}
}

func TestEmit_DoesNotPanicOnSinkFailure(t *testing.T) {
	SetSink(func(ev TradingEvent) { panic("sink boom") })
	t.Cleanup(func() { SetSink(nil) })
	Emit(TradingEvent{EventType: EventExecutionFailed, Source: SourceGateway, TradeDate: "2026-08-17"})
}

func TestEmitExecution_RequiresPlanIDAndReasonFields(t *testing.T) {
	var got []TradingEvent
	SetSink(func(ev TradingEvent) { got = append(got, ev) })
	t.Cleanup(func() { SetSink(nil) })

	EmitExecution(EventExecutionSkipped, "2026-08-17", "closed", StatusSkip, "OUTSIDE_SESSION", 99, time.Now(), "")
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].PlanID != 99 {
		t.Fatalf("plan_id=%d", got[0].PlanID)
	}
	if got[0].Reason != "OUTSIDE_SESSION" {
		t.Fatalf("reason=%s", got[0].Reason)
	}
	if got[0].EventType != EventExecutionSkipped {
		t.Fatalf("type=%s", got[0].EventType)
	}
}

func TestEmitSettlement(t *testing.T) {
	var got []TradingEvent
	SetSink(func(ev TradingEvent) { got = append(got, ev) })
	t.Cleanup(func() { SetSink(nil) })

	EmitSettlement(EventSettlementCompleted, "2026-08-17", StatusPass, "ok", 1, time.Now())
	EmitSettlement(EventSettlementFailed, "2026-08-17", StatusFail, "db error", 0, time.Now())
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].EventType != EventSettlementCompleted || got[1].EventType != EventSettlementFailed {
		t.Fatalf("types=%s,%s", got[0].EventType, got[1].EventType)
	}
	if got[0].Phase != PhaseSettlement {
		t.Fatalf("phase=%s", got[0].Phase)
	}
}
