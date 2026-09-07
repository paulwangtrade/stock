package morningpreparation_test

import (
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/morningpreparation"
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

func draftPlan(stage string) *models.TradePlan {
	return &models.TradePlan{
		ID:                   39,
		TradeDate:            "2026-08-14",
		Status:               models.TradePlanStatusDraft,
		PricingPolicyVersion: 1,
		PricingStage:         stage,
	}
}

func frozenPlan(freeze time.Time) *models.TradePlan {
	return &models.TradePlan{
		ID:                   39,
		TradeDate:            "2026-08-14",
		Status:               models.TradePlanStatusReady,
		PricingPolicyVersion: 1,
		PricingStage:         "morning_materialized",
		ApprovedAt:           frozenAt(freeze),
		FreezeAt:             frozenAt(freeze),
	}
}

// Case 1: normal morning prep — frozen on time + materialized.
func TestEvaluateMorningReadiness_Case1_Ready(t *testing.T) {
	freeze := at(9, 25, 0)
	now := at(9, 28, 0)
	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: "2026-08-14",
		Now:       now,
		Plan:      frozenPlan(freeze),
	})
	require.Equal(t, morningpreparation.MorningStatusReady, obs.Status)
	require.Equal(t, morningpreparation.CheckPass, obs.FreezeStatus)
	require.Equal(t, morningpreparation.CheckNotApplicable, obs.MaterializationStatus)
	require.Equal(t, morningpreparation.MorningStatusReady, obs.DeadlineStatus)
}

// Case 2: 09:25 no frozen plan → checkpoint reason.
func TestEvaluateMorningReadiness_Case2_0925NoFrozen_MissedReadiness(t *testing.T) {
	now := at(9, 25, 0)
	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate:          "2026-08-14",
		Now:                now,
		Plan:               draftPlan("morning_materialized"),
		DeadlineCheckpoint: true,
	})
	require.Equal(t, morningpreparation.MorningStatusNotReady, obs.Status)
	require.Equal(t, morningpreparation.ReasonMissingOpenReadiness, obs.Reason)
	require.Equal(t, morningpreparation.CheckFail, obs.FreezeStatus)
	require.Equal(t, morningpreparation.CheckPass, obs.MaterializationStatus)
}

// Case 3: late freeze → miss reason.
func TestEvaluateMorningReadiness_Case3_LateFreeze(t *testing.T) {
	freeze := at(11, 12, 0)
	now := freeze
	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: "2026-08-14",
		Now:       now,
		Plan:      frozenPlan(freeze),
	})
	require.Equal(t, morningpreparation.MorningStatusMissedDeadline, obs.Status)
	require.Equal(t, morningpreparation.ReasonFreezeAfterDeadline, obs.Reason)
	require.Equal(t, string(tradingwindow.StatusMissedOpenWindow), obs.WindowStatus)
}

// Case 4: manual execution not blocked (policy layer unchanged).
func TestEvaluateMorningReadiness_Case4_ManualExecutionUnaffected(t *testing.T) {
	freeze := at(11, 12, 0)
	window := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   "2026-08-14",
		CurrentTime: freeze,
		PlanStatus:  models.TradePlanStatusReady,
		IsFrozen:    true,
		FrozenTime:  frozenAt(freeze),
	})
	require.False(t, tradingwindow.BlocksManualRunExecution(window))
}

func TestEvaluateMorningReadiness_NoPlan(t *testing.T) {
	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: "2026-08-14",
		Now:       at(9, 25, 0),
		Plan:      nil,
	})
	require.Equal(t, morningpreparation.ReasonPlanNotCreated, obs.Reason)
	require.Equal(t, morningpreparation.CheckFail, obs.MaterializationStatus)
}

func TestEvaluateMorningReadiness_AfterDeadlineNotFrozen(t *testing.T) {
	now := at(9, 31, 0)
	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: "2026-08-14",
		Now:       now,
		Plan:      draftPlan("after_close_intent"),
	})
	require.Equal(t, morningpreparation.MorningStatusMissedDeadline, obs.Status)
	require.Equal(t, morningpreparation.CheckPending, obs.MaterializationStatus)
}
