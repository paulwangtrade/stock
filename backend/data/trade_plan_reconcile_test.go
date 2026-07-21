package data

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func tradePlanReconcileSetup(t *testing.T) *TradePlanRepo {
	t.Helper()
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	return NewTradePlanRepo()
}

func seedExecutingPlan(t *testing.T, repo *TradePlanRepo, tradeDate string, executedAt time.Time, items []models.TradePlanItem) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate,
		Status:    models.TradePlanStatusExecuting,
		Side:      "buy",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, items))
	execAt := executedAt
	require.NoError(t, db.Dao.Model(plan).Updates(map[string]any{
		"status":      models.TradePlanStatusExecuting,
		"executed_at": execAt,
	}).Error)
	got, err := repo.GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func TestTradePlanReconcile_StuckExecuting_NoFills(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	stale := time.Now().Add(-30 * time.Minute)
	plan := seedExecutingPlan(t, repo, "2026-07-17", stale, []models.TradePlanItem{
		{TradeDate: "2026-07-17", StockCode: "sz000001", Status: models.TradePlanItemPending},
		{TradeDate: "2026-07-17", StockCode: "sh600519", Status: models.TradePlanItemPending},
	})

	plans, err := repo.ListExecutingPlans(time.Now().Add(-15 * time.Minute))
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, plan.ID, plans[0].ID)

	rec, err := ReconcileTradePlan(plan.ID)
	require.NoError(t, err)
	require.True(t, rec.Applied)
	require.Equal(t, models.TradePlanStatusFailed, rec.TerminalStatus)

	got, err := repo.GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusFailed, got.Status)
}

func TestTradePlanReconcile_PartialFilled(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	api := NewPaperTradingApi()
	acc, err := api.ResetAccount(1_000_000)
	require.NoError(t, err)
	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: acc.ID, StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100, AutoFill: true,
	})
	require.NoError(t, err)

	stale := time.Now().Add(-20 * time.Minute)
	plan := seedExecutingPlan(t, repo, "2026-07-18", stale, []models.TradePlanItem{
		{TradeDate: "2026-07-18", StockCode: "sz000001", Status: models.TradePlanItemFilled, OrderID: order.ID},
		{TradeDate: "2026-07-18", StockCode: "sh600519", Status: models.TradePlanItemPending},
	})

	rec, err := ReconcileTradePlan(plan.ID)
	require.NoError(t, err)
	require.True(t, rec.Applied)
	require.Equal(t, models.TradePlanStatusPartial, rec.TerminalStatus)

	got, err := repo.GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusPartial, got.Status)
}

func TestTradePlanReconcile_AllFilled(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	api := NewPaperTradingApi()
	acc, err := api.ResetAccount(1_000_000)
	require.NoError(t, err)

	var items []models.TradePlanItem
	for i, code := range []string{"sz000001", "sh600519"} {
		order, oerr := api.SubmitPaperOrder(PaperSubmitOrderReq{
			AccountID: acc.ID, StockCode: code, Side: "buy", Price: 10 + float64(i), Volume: 100, AutoFill: true,
		})
		require.NoError(t, oerr)
		items = append(items, models.TradePlanItem{
			TradeDate: "2026-07-19", StockCode: code, Status: models.TradePlanItemFilled, OrderID: order.ID,
		})
	}

	stale := time.Now().Add(-20 * time.Minute)
	plan := seedExecutingPlan(t, repo, "2026-07-19", stale, items)

	rec, err := ReconcileTradePlan(plan.ID)
	require.NoError(t, err)
	require.True(t, rec.Applied)
	require.Equal(t, models.TradePlanStatusDone, rec.TerminalStatus)
}

func TestTradePlanReconcile_FinishPlanCAS_Miss(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	plan := &models.TradePlan{TradeDate: "2026-07-20", Status: models.TradePlanStatusReady}
	require.NoError(t, repo.CreatePlanWithItems(plan, nil))

	ok, err := repo.FinishPlanCAS(plan.ID, models.TradePlanStatusExecuting, models.TradePlanStatusDone, "should miss")
	require.NoError(t, err)
	require.False(t, ok)

	got, err := repo.GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, got.Status)
}

func TestTradePlanReconcile_ReconcileCAS_MissAfterDone(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	stale := time.Now().Add(-30 * time.Minute)
	plan := seedExecutingPlan(t, repo, "2026-07-21", stale, []models.TradePlanItem{
		{TradeDate: "2026-07-21", StockCode: "sz000001", Status: models.TradePlanItemPending},
	})

	ok, err := repo.FinishPlanCAS(plan.ID, models.TradePlanStatusExecuting, models.TradePlanStatusFailed, "manual finish")
	require.NoError(t, err)
	require.True(t, ok)

	rec, err := ReconcileTradePlan(plan.ID)
	require.NoError(t, err)
	require.False(t, rec.Applied)
	require.Equal(t, "not executing", rec.SkippedReason)
}

func TestTradePlanReconcile_PanicRecoveryUsesReconcile(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	api := NewPaperTradingApi()
	acc, err := api.ResetAccount(1_000_000)
	require.NoError(t, err)
	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: acc.ID, StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100, AutoFill: true,
	})
	require.NoError(t, err)

	plan := &models.TradePlan{
		TradeDate: todayTradeDateLocal(),
		Status:    models.TradePlanStatusReady,
		Side:      "buy",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: plan.TradeDate, StockCode: "sz000001", Status: models.TradePlanItemFilled, OrderID: order.ID},
		{TradeDate: plan.TradeDate, StockCode: "sh600519", Status: models.TradePlanItemPending},
	}))

	ok, err := repo.TryBeginExecute(plan.ID)
	require.NoError(t, err)
	require.True(t, ok)

	panicPlan := plan
	func() {
		defer func() {
			if r := recover(); r != nil {
				rec, rerr := ReconcileTradePlan(panicPlan.ID)
				require.NoError(t, rerr)
				require.True(t, rec.Applied)
				require.Equal(t, models.TradePlanStatusPartial, rec.TerminalStatus)
			} else {
				t.Fatal("expected panic")
			}
		}()
		panic(fmt.Sprintf("simulated mid-execution panic plan_id=%d", plan.ID))
	}()

	got, err := repo.GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusPartial, got.Status)
	require.Contains(t, got.Message, "reconcile:")
}

func TestTradePlanReconcile_StaleBatch(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	stale := time.Now().Add(-30 * time.Minute)
	seedExecutingPlan(t, repo, "2026-07-22", stale, []models.TradePlanItem{
		{TradeDate: "2026-07-22", StockCode: "sz000001", Status: models.TradePlanItemPending},
	})

	results, err := ReconcileStaleTradePlans(15 * time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Applied)
	require.Equal(t, models.TradePlanStatusFailed, results[0].TerminalStatus)
}

func TestTradePlanReconcile_DailyStatusObservability(t *testing.T) {
	repo := tradePlanReconcileSetup(t)
	require.NoError(t, SavePaperOpenBuyConfig(PaperOpenBuyConfig{EnablePaperOpenBuy: true}))

	stale := time.Now().Add(-20 * time.Minute)
	plan := seedExecutingPlan(t, repo, "2026-07-23", stale, []models.TradePlanItem{
		{TradeDate: "2026-07-23", StockCode: "sz000001", Status: models.TradePlanItemPending},
	})
	_ = plan

	st := GetDailyTradingStatusForDate("2026-07-23")
	require.True(t, st.Plan.ReconcileRecommended)
	require.NotEmpty(t, st.Plan.ExecutingSince)
	require.Contains(t, st.BlockReasons, "plan_reconcile_recommended")
}
