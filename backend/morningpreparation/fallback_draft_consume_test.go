package morningpreparation

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func TestCaseD_FallbackDraftIsMaterializeConsumable(t *testing.T) {
	setupMorningIsolationDB(t)
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    false,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))

	td := "2026-08-19"
	items := []models.CandidatePoolItem{{
		TradeDate: td, StockCode: "sz000001", StockName: "平安银行", Rank: 1, Score: 1,
	}}
	pool := &models.CandidatePool{
		TradeDate: td, GeneratedAt: time.Now(),
		Source: models.CandidatePoolSourceFollow, Status: models.CandidatePoolStatusReady,
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))

	plan, err := strategy.BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.False(t, plan.EnableExecute)
	require.Nil(t, plan.FreezeAt)
	require.True(t, shouldAttemptMaterialize(plan), "09:26 gate must accept fallback draft")

	looked, err := LookupMorningMaterializePlan(td)
	require.NoError(t, err)
	require.Equal(t, plan.ID, looked.ID)

	restoreMorningJobFns(t)
	var attemptedID uint
	runMaterializeFn = func(planID uint) (*strategy.MorningIntentMaterializeResult, error) {
		attemptedID = planID
		return &strategy.MorningIntentMaterializeResult{Success: true, PlanID: planID, PricingStage: "morning_materialized"}, nil
	}

	res := RunMorningPlanPreparationJob(JobOptions{
		TradeDate:          td,
		Now:                time.Date(2026, 8, 19, 9, 26, 0, 0, time.Local),
		AttemptMaterialize: true,
	})
	require.True(t, res.MaterializeAttempted)
	require.True(t, res.MaterializeSuccess)
	require.Equal(t, plan.ID, attemptedID)
}
