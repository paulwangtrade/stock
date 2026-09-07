package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestRunExecution_StateGuard_PassApprovedFrozenReady(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	papertrading.SetExecutionNowForTest(sessionANow)
	t.Cleanup(func() { papertrading.SetExecutionNowForTest(nil) })

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	require.NotNil(t, plan.ApprovedAt)
	require.NotNil(t, plan.FreezeAt)

	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: "2026-07-30", PlanID: plan.ID, Trigger: papertrading.TriggerManual, Actor: "test:guard-pass",
		SkipWeekdayCheck: true, Price: price,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, papertrading.ExecutionEntryGateway, res.Entry)
}

func TestRunExecution_StateGuard_FailReadyWithoutFreeze(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	papertrading.SetExecutionNowForTest(sessionANow)
	t.Cleanup(func() { papertrading.SetExecutionNowForTest(nil) })

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: now, Status: models.TradePlanStatusReady,
		PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
		// ApprovedAt set but NO FreezeAt — Integrity-class naked ready
		ApprovedAt: &now, ApprovedBy: "test",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
	}))

	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10}}}
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: "2026-07-30", PlanID: plan.ID, Trigger: papertrading.TriggerManual, Actor: "test:guard-nofreeze",
		SkipWeekdayCheck: true, Price: price,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), models.ReasonPlanNotFrozen)
	require.Contains(t, err.Error(), "INV-P-RDY-01")
	_ = res
}

func TestRunExecution_StateGuard_FailReadyWithoutApprove(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	papertrading.SetExecutionNowForTest(sessionANow)
	t.Cleanup(func() { papertrading.SetExecutionNowForTest(nil) })

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: now, Status: models.TradePlanStatusReady,
		PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
		FreezeAt: &now, FreezeBy: "test",
		// Freeze without ApprovedAt
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
	}))
	require.True(t, plan.IsFrozen())

	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10}}}
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: "2026-07-30", PlanID: plan.ID, Trigger: papertrading.TriggerManual, Actor: "test:guard-noapprove",
		SkipWeekdayCheck: true, Price: price,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), models.ReasonPlanNotApproved)
	require.Contains(t, err.Error(), "ApprovedAt")
	_ = res
}
