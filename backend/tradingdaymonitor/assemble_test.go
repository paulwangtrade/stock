package tradingdaymonitor_test

import (
	"testing"
	"time"

	"go-stock/backend/tradingdaymonitor"
	"go-stock/backend/tradingevent"

	"github.com/stretchr/testify/require"
)

func TestBuild_NormalTradingDay(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2026-08-17"
	now := time.Date(2026, 8, 17, 15, 10, 0, 0, time.Local)

	tradingevent.EmitAutomationStep(td, now, "materialize", "MATERIALIZATION_AUTO_SUCCESS", "", 42, true)
	tradingevent.EmitAutomationStep(td, now, "approve", "AUTO_APPROVAL_SUCCESS", "", 42, true)
	tradingevent.EmitAutomationStep(td, now, "freeze", "AUTO_FREEZE_SUCCESS", "", 42, true)
	tradingevent.EmitExecution(tradingevent.EventExecutionStarted, td, "A", tradingevent.StatusPass, "GATEWAY_ENTER", 42, now, "")
	tradingevent.EmitExecution(tradingevent.EventExecutionCompleted, td, "A", tradingevent.StatusPass, "session_A", 42, now, "exec-1")
	tradingevent.EmitSettlement(tradingevent.EventSettlementCompleted, td, tradingevent.StatusPass, "SETTLEMENT_OK", 1, now)

	view := tradingdaymonitor.Build(tradingdaymonitor.Options{
		TradeDate:     td,
		AsOf:          now,
		SkipReadiness: true,
	})
	require.Equal(t, td, view.TradeDate)
	require.Equal(t, tradingdaymonitor.StatusPass, view.Morning.Materialize.Status)
	require.Equal(t, tradingdaymonitor.StatusPass, view.Morning.Approve.Status)
	require.Equal(t, tradingdaymonitor.StatusPass, view.Morning.Freeze.Status)
	require.Equal(t, "A", view.Execution.Session)
	require.Equal(t, tradingdaymonitor.StatusPass, view.Execution.Status)
	require.Equal(t, uint(42), view.Execution.PlanID)
	require.Equal(t, "session_A", view.Execution.Reason)
	require.Equal(t, tradingevent.EventExecutionCompleted, view.Execution.EventType)
	require.Equal(t, tradingdaymonitor.StatusPass, view.Settlement.Status)
	require.Equal(t, tradingdaymonitor.SourceTradingEvent, view.Morning.Materialize.Source)
}

func TestBuild_NoPlan(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2099-01-01"
	view := tradingdaymonitor.Build(tradingdaymonitor.Options{
		TradeDate:         td,
		AsOf:              time.Date(2099, 1, 1, 10, 0, 0, 0, time.Local),
		Events:            []tradingevent.TradingEvent{},
		Plan:              nil,
		DisablePlanLookup: true,
		SkipReadiness:     false,
	})
	require.Equal(t, tradingdaymonitor.StatusUnknown, view.Morning.Materialize.Status)
	require.Equal(t, "PLAN_NOT_FOUND", view.Morning.Materialize.Reason)
	require.Equal(t, tradingdaymonitor.SourceReadiness, view.Morning.Materialize.Source)
	require.Equal(t, tradingdaymonitor.StatusUnknown, view.Morning.Approve.Status)
	require.Equal(t, tradingdaymonitor.StatusUnknown, view.Morning.Freeze.Status)
	require.Equal(t, tradingdaymonitor.StatusPending, view.Execution.Status)
	require.Equal(t, tradingdaymonitor.StatusPending, view.Settlement.Status)
}

func TestBuild_ExecutionFailed(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2026-08-18"
	now := time.Date(2026, 8, 18, 9, 35, 0, 0, time.Local)
	tradingevent.EmitExecution(tradingevent.EventExecutionStarted, td, "A", tradingevent.StatusPass, "GATEWAY_ENTER", 7, now, "")
	tradingevent.EmitExecution(tradingevent.EventExecutionFailed, td, "A", tradingevent.StatusFail, "broker_error", 7, now, "exec-x")

	view := tradingdaymonitor.Build(tradingdaymonitor.Options{
		TradeDate:     td,
		AsOf:          now,
		SkipReadiness: true,
	})
	require.Equal(t, tradingdaymonitor.StatusFail, view.Execution.Status)
	require.Equal(t, uint(7), view.Execution.PlanID)
	require.Equal(t, "broker_error", view.Execution.Reason)
	require.Equal(t, tradingevent.EventExecutionFailed, view.Execution.EventType)
	require.Equal(t, "A", view.Execution.Session)
}
