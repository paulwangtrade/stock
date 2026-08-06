package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func atClock(hour, min, sec int) time.Time {
	return time.Date(2026, 8, 5, hour, min, sec, 0, time.Local) // Wednesday
}

func TestResolveExecutionSession_Windows(t *testing.T) {
	cases := []struct {
		name string
		h, m int
		want papertrading.ExecutionSession
	}{
		{"before_open", 9, 29, papertrading.SessionClosed},
		{"A_open", 9, 30, papertrading.SessionA},
		{"A_morning", 10, 0, papertrading.SessionA},
		{"A_before_lunch", 11, 29, papertrading.SessionA},
		{"lunch_start", 11, 30, papertrading.SessionClosed},
		{"lunch_mid", 12, 0, papertrading.SessionClosed},
		{"A_afternoon", 13, 0, papertrading.SessionA},
		{"A_before_close", 14, 59, papertrading.SessionA},
		{"B_start", 15, 0, papertrading.SessionB},
		{"B_mid", 15, 14, papertrading.SessionB},
		{"B_last_minute", 15, 29, papertrading.SessionB},
		{"C_start", 15, 30, papertrading.SessionC},
		{"C_evening", 16, 0, papertrading.SessionC},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := papertrading.ResolveExecutionSession(atClock(tc.h, tc.m, 0))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEvaluateSessionPolicy_AllowReject(t *testing.T) {
	a := papertrading.EvaluateSessionPolicy(atClock(10, 0, 0))
	require.True(t, a.Allow)
	require.Equal(t, papertrading.SessionA, a.Session)
	require.Equal(t, papertrading.PolicyAllow, a.Decision)

	b := papertrading.EvaluateSessionPolicy(atClock(15, 14, 0))
	require.True(t, b.Allow)
	require.Equal(t, papertrading.SessionB, b.Session)

	lunch := papertrading.EvaluateSessionPolicy(atClock(12, 0, 0))
	require.False(t, lunch.Allow)
	require.Equal(t, papertrading.SessionClosed, lunch.Session)
	require.Equal(t, papertrading.PolicyReject, lunch.Decision)

	c := papertrading.EvaluateSessionPolicy(atClock(15, 30, 0))
	require.False(t, c.Allow)
	require.Equal(t, papertrading.SessionC, c.Session)
}

func TestRunExecution_SessionA_Allows(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerManual, Actor: "test",
		Price: price, SkipWeekdayCheck: true,
		Now: atClock(10, 0, 0),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionA, res.Session)
	require.Equal(t, papertrading.PolicyAllow, res.Decision)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)
}

func TestRunExecution_SessionB_Allows(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerManual, Actor: "test",
		Price: price, SkipWeekdayCheck: true,
		Now: atClock(15, 14, 19),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionB, res.Session)
	require.Equal(t, papertrading.PolicyAllow, res.Decision)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)
	// B.1 does not yet switch to close_price — still market_open fill path.
}

func TestRunExecution_Lunch_Rejects(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerManual, Actor: "test",
		Price: price, SkipWeekdayCheck: true,
		Now: atClock(12, 0, 0),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionClosed, res.Session)
	require.Equal(t, papertrading.PolicyReject, res.Decision)
	require.Equal(t, papertrading.RunStatusSkippedOutsideSession, res.Status)
	require.Equal(t, 0, res.FilledCount)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, st.OrdersTotal)
}

func TestRunExecution_After1530_Rejects(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerManual, Actor: "test",
		Price: price, SkipWeekdayCheck: true,
		Now: atClock(15, 31, 0),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionC, res.Session)
	require.Equal(t, papertrading.PolicyReject, res.Decision)
	require.Equal(t, papertrading.RunStatusSkippedOutsideSession, res.Status)
	require.Equal(t, 0, res.FilledCount)
}

func TestRunExecution_WeekendBypass_DoesNotSkipSession(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	// Saturday calendar date on the clock, but session forced to C via time-of-day.
	saturdayC := time.Date(2026, 8, 8, 16, 0, 0, 0, time.Local) // Saturday
	require.Equal(t, time.Saturday, saturdayC.Weekday())

	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerManual, Actor: "test",
		Price: price,
		SkipWeekdayCheck: true, // weekday bypass ON
		Now:              saturdayC,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionC, res.Session)
	require.Equal(t, papertrading.PolicyReject, res.Decision)
	require.Equal(t, papertrading.RunStatusSkippedOutsideSession, res.Status)
	require.Equal(t, 0, res.FilledCount)
}
