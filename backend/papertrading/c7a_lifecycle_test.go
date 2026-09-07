package papertrading_test

import (
	"errors"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestC7A_Lifecycle_AllFilled_ReadyToDone(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
		buyItem("sz000002", "万科A", 500),
	})
	require.Equal(t, models.TradePlanStatusReady, plan.Status)

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerCron,
		Actor:            "cron",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
			"sz000001": {Open: 10.0, LimitUp: 11},
			"sz000002": {Open: 8.0, LimitUp: 9},
		}},
		SkipWeekdayCheck: true,
		Now:              sessionANow(),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 2, res.FilledCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDone, got.Status)
	require.NotNil(t, got.ExecutedAt)
	require.Len(t, got.Items, 2)
	for _, it := range got.Items {
		require.Equal(t, models.TradePlanItemFilled, it.Status)
		require.Greater(t, it.OrderID, uint(0))
		require.Greater(t, it.FillID, uint(0))
		require.Greater(t, it.FilledVolume, int64(0))
		require.Greater(t, it.FilledPrice, 0.0)
	}
}

func TestC7A_Lifecycle_PartialFill(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
		buyItem("sz000099", "无行情", 1000),
	})

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerCron,
		Actor:            "cron",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{
			"sz000001": {Open: 10.0, LimitUp: 11},
			// sz000099 missing → reject
		}},
		SkipWeekdayCheck: true,
		Now:              sessionANow(),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusCompletedWithRejects, res.Status)
	require.Equal(t, 1, res.FilledCount)
	require.Equal(t, 1, res.RejectCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusPartial, got.Status)

	byCode := map[string]models.TradePlanItem{}
	for _, it := range got.Items {
		byCode[it.StockCode] = it
	}
	require.Equal(t, models.TradePlanItemFilled, byCode["sz000001"].Status)
	require.Equal(t, models.TradePlanItemError, byCode["sz000099"].Status)
	require.Equal(t, papertrading.RejectMissingOpenPrice, byCode["sz000099"].Error)
}

func TestC7A_Lifecycle_BrokerError_ReadyToFailed(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	t.Cleanup(func() { papertrading.SetRunForPlanHookForTest(nil) })

	plan := seedFrozenPlan(t, "2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
	})
	papertrading.SetRunForPlanHookForTest(func() error {
		return errors.New("c7a simulated broker failure")
	})

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerCron,
		Actor:            "cron",
		Price:            papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}},
		SkipWeekdayCheck: true,
		Now:              sessionANow(),
	})
	require.Error(t, err)
	require.Equal(t, papertrading.RunStatusFailed, res.Status)
	require.Contains(t, err.Error(), "c7a simulated broker failure")

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusFailed, got.Status)
	require.NotNil(t, got.ExecutedAt)
}

func TestC7A_Lifecycle_TryBeginExecute_PreventsDuplicate(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	t.Cleanup(func() { papertrading.SetBeforeTryBeginForTest(nil) })

	plan := seedFrozenPlan(t, "2026-08-14", []models.TradePlanItem{
		buyItem("sz000001", "平安银行", 1000),
	})
	repo := data.NewTradePlanRepo()

	// Steal CAS before Job Begin → Job must skip Broker.
	papertrading.SetBeforeTryBeginForTest(func(planID uint) {
		ok, err := repo.TryBeginExecute(planID)
		require.NoError(t, err)
		require.True(t, ok, "pre-hook must win TryBeginExecute")
	})

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerCron,
		Actor:            "cron",
		Price:            papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}},
		SkipWeekdayCheck: true,
		Now:              sessionANow(),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusSkippedPlanLifecycle, res.Status)

	got, err := repo.GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusExecuting, got.Status)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, st.OrdersTotal, "CAS miss must not create orders")

	// Terminal plan also rejects a second Begin.
	ok, err := repo.FinishPlanCAS(plan.ID, models.TradePlanStatusExecuting, models.TradePlanStatusDone, "test finish")
	require.NoError(t, err)
	require.True(t, ok)
	ok2, err := repo.TryBeginExecute(plan.ID)
	require.NoError(t, err)
	require.False(t, ok2, "done plan must not TryBeginExecute again")
}
