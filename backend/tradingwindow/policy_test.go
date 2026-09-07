package tradingwindow_test

import (
	"testing"
	"time"

	"go-stock/backend/tradingwindow"

	"github.com/stretchr/testify/require"
)

var loc = time.FixedZone("CST", 8*3600)

func tradeDay() time.Time {
	return time.Date(2026, 8, 14, 0, 0, 0, 0, loc)
}

func at(h, m, s int) time.Time {
	d := tradeDay()
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, s, 0, loc)
}

func frozenAt(t time.Time) *time.Time { return &t }

func TestEvaluatePlanWindow_Case1_Freeze0920_ReadyForOpen(t *testing.T) {
	freeze := at(9, 20, 0)
	res := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: freeze,
		PlanStatus:  "ready",
		IsFrozen:    true,
		FrozenTime:  frozenAt(freeze),
	})
	require.Equal(t, tradingwindow.StatusReadyForOpen, res.Status)
	require.Empty(t, res.Reason)
	require.False(t, res.BlocksManualExecution)
	require.False(t, tradingwindow.BlocksManualRunExecution(res))
}

func TestEvaluatePlanWindow_Case2_Freeze0929_ReadyForOpen(t *testing.T) {
	freeze := at(9, 29, 0)
	res := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: freeze,
		PlanStatus:  "ready",
		IsFrozen:    true,
		FrozenTime:  frozenAt(freeze),
	})
	require.Equal(t, tradingwindow.StatusReadyForOpen, res.Status)
	require.Empty(t, res.Reason)
}

func TestEvaluatePlanWindow_Case3_Freeze0931_MissedOpenWindow(t *testing.T) {
	freeze := at(9, 31, 0)
	res := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: freeze,
		PlanStatus:  "ready",
		IsFrozen:    true,
		FrozenTime:  frozenAt(freeze),
	})
	require.Equal(t, tradingwindow.StatusMissedOpenWindow, res.Status)
	require.Equal(t, tradingwindow.ReasonPlanFrozenAfterDeadline, res.Reason)
	require.True(t, res.BlocksAutoExecution)
	require.False(t, tradingwindow.BlocksManualRunExecution(res))
}

func TestEvaluatePlanWindow_Case4_Freeze1112_MissedOpenWindow(t *testing.T) {
	freeze := at(11, 12, 0)
	res := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: freeze,
		PlanStatus:  "ready",
		IsFrozen:    true,
		FrozenTime:  frozenAt(freeze),
	})
	require.Equal(t, tradingwindow.StatusMissedOpenWindow, res.Status)
	require.Equal(t, tradingwindow.ReasonPlanFrozenAfterDeadline, res.Reason)
	require.True(t, res.BlocksAutoExecution)
}

func TestEvaluatePlanWindow_Case5_ManualRunExecutionNotBlocked(t *testing.T) {
	cases := []tradingwindow.PlanWindowResult{
		{Status: tradingwindow.StatusMissedOpenWindow, Reason: tradingwindow.ReasonPlanFrozenAfterDeadline, BlocksAutoExecution: true},
		{Status: tradingwindow.StatusExpired, Reason: tradingwindow.ReasonNoExecutionWindow, BlocksAutoExecution: true},
		{Status: tradingwindow.StatusReadyForOpen, BlocksAutoExecution: false},
	}
	for _, c := range cases {
		require.False(t, tradingwindow.BlocksManualRunExecution(c))
		require.False(t, c.BlocksManualExecution)
	}
}

func TestEvaluatePlanWindow_NotFrozenAfterDeadline_Missed(t *testing.T) {
	now := at(9, 31, 0)
	res := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: now,
		PlanStatus:  "draft",
		IsFrozen:    false,
	})
	require.Equal(t, tradingwindow.StatusMissedOpenWindow, res.Status)
	require.Equal(t, tradingwindow.ReasonPlanNotFrozenBeforeOpen, res.Reason)
}

func TestEvaluatePlanWindow_FrozenOnTime_InOpenWindow_Executable(t *testing.T) {
	freeze := at(9, 20, 0)
	now := at(10, 0, 0)
	res := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: now,
		PlanStatus:  "ready",
		IsFrozen:    true,
		FrozenTime:  frozenAt(freeze),
	})
	require.Equal(t, tradingwindow.StatusOpenExecutable, res.Status)
}

func TestReasonLabel(t *testing.T) {
	require.Equal(t, "Plan frozen after deadline", tradingwindow.ReasonLabel(tradingwindow.ReasonPlanFrozenAfterDeadline))
}
