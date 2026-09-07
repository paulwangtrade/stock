package morningpreparation

import (
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func TestRunMorningPlanPreparationJob_ObservationOnly(t *testing.T) {
	orig := lookupPlanFn
	defer func() { lookupPlanFn = orig }()
	freeze := time.Date(2026, 8, 14, 9, 25, 0, 0, time.Local)
	lookupPlanFn = func(_ string) (*models.TradePlan, error) {
		return frozenPlanFixture(freeze), nil
	}
	res := RunMorningPlanPreparationJob(JobOptions{
		TradeDate: "2026-08-14",
		Now:       time.Date(2026, 8, 14, 9, 26, 0, 0, time.Local),
	})
	require.Equal(t, MorningStatusReady, res.Observation.Status)
	require.False(t, res.MaterializeAttempted)
}

func TestRunMorningPlanPreparationJob_AttemptMaterializeDraft(t *testing.T) {
	origLookup := lookupPlanFn
	origMat := runMaterializeFn
	defer func() {
		lookupPlanFn = origLookup
		runMaterializeFn = origMat
	}()
	lookupPlanFn = func(_ string) (*models.TradePlan, error) {
		return &models.TradePlan{
			ID: 7, TradeDate: "2026-08-14", Status: models.TradePlanStatusDraft,
			PricingPolicyVersion: 1, PricingStage: "after_close_intent",
		}, nil
	}
	runMaterializeFn = func(planID uint) (*strategy.MorningIntentMaterializeResult, error) {
		require.Equal(t, uint(7), planID)
		return &strategy.MorningIntentMaterializeResult{Success: true, PlanID: planID, PricingStage: "morning_materialized"}, nil
	}
	res := RunMorningPlanPreparationJob(JobOptions{
		TradeDate:          "2026-08-14",
		Now:                time.Date(2026, 8, 14, 9, 26, 0, 0, time.Local),
		AttemptMaterialize: true,
	})
	require.True(t, res.MaterializeAttempted)
	require.True(t, res.MaterializeSuccess)
}

func frozenPlanFixture(freeze time.Time) *models.TradePlan {
	ft := freeze
	at := freeze
	return &models.TradePlan{
		ID: 1, TradeDate: "2026-08-14", Status: models.TradePlanStatusReady,
		PricingStage: "morning_materialized", FreezeAt: &ft, ApprovedAt: &at,
	}
}
