package data

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// Phase1.5 验收：GetDailyTradingStatus blockReason 异常场景。

func phase15AcceptanceSetup(t *testing.T) {
	t.Helper()
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	require.NoError(t, SavePaperOpenBuyConfig(PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 100_000,
	}))
	SetPlanItemExecutor(&noopPlanItemExecutor{})
	t.Cleanup(func() { SetPlanItemExecutor(nil) })
}

func TestPhase15_GetDailyTradingStatus_NoCandidate(t *testing.T) {
	phase15AcceptanceSetup(t)

	st := GetDailyTradingStatusForDate("2026-07-30")
	require.Equal(t, "EMPTY", st.Candidate.Status)
	require.Equal(t, 0, st.Candidate.Count)
	require.Equal(t, "EMPTY", st.Plan.Status)
	require.Equal(t, "not_started", st.Execution.Phase)
	require.Contains(t, st.BlockReasons, "candidate_pool_empty")
	require.Contains(t, st.BlockReasons, "trade_plan_missing")
	require.Equal(t, "candidate_pool_empty", st.BlockReason)
	require.Contains(t, st.Message, "blocked: candidate_pool_empty")
}

func TestPhase15_GetDailyTradingStatus_CandidateWithoutPlan(t *testing.T) {
	phase15AcceptanceSetup(t)

	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-07-31", GeneratedAt: now, Source: "strategy_run",
		Status: models.CandidatePoolStatusReady, ItemCount: 3, Message: "pool only",
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{
		{TradeDate: "2026-07-31", StockCode: "sz000001", Rank: 1, Score: 0.9},
		{TradeDate: "2026-07-31", StockCode: "sh600519", Rank: 2, Score: 0.8},
		{TradeDate: "2026-07-31", StockCode: "sz000792", Rank: 3, Score: 0.7},
	}))

	st := GetDailyTradingStatusForDate("2026-07-31")
	require.Equal(t, "READY", st.Candidate.Status)
	require.Equal(t, 3, st.Candidate.Count)
	require.Equal(t, "EMPTY", st.Plan.Status)
	require.Zero(t, st.Plan.PlanID)
	require.Equal(t, "not_started", st.Execution.Phase)
	require.False(t, st.Execution.Ready)
	require.NotContains(t, st.BlockReasons, "candidate_pool_empty")
	require.Contains(t, st.BlockReasons, "trade_plan_missing")
	require.Equal(t, "trade_plan_missing", st.BlockReason)
}

func TestPhase15_GetDailyTradingStatus_RiskAllFiltered(t *testing.T) {
	phase15AcceptanceSetup(t)

	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-08-01", GeneratedAt: now, Source: "strategy_run",
		Status: models.CandidatePoolStatusReady, ItemCount: 2,
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{
		{TradeDate: "2026-08-01", StockCode: "sz000001", Rank: 1, Score: 0.9},
		{TradeDate: "2026-08-01", StockCode: "sh600519", Rank: 2, Score: 0.8},
	}))

	plan := &models.TradePlan{
		TradeDate: "2026-08-01", PoolID: pool.ID, GeneratedAt: now,
		Status: models.TradePlanStatusReady, RiskStatus: "reject",
		RiskAcceptedCount: 0, RiskFilteredCount: 2, RiskSummary: "all rejected",
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Status: models.TradePlanItemSkipped, RiskCode: "GROSS_EXPOSURE", RiskMessage: "总暴露超限"},
		{StockCode: "sh600519", Status: models.TradePlanItemSkipped, RiskCode: "SINGLE_NAME_EXCEEDED", RiskMessage: "单票超限"},
	}))

	st := GetDailyTradingStatusForDate("2026-08-01")
	require.Equal(t, "READY", st.Candidate.Status)
	require.Equal(t, models.TradePlanStatusReady, st.Plan.Status)
	require.Equal(t, 2, st.Plan.SkippedCount)
	require.Equal(t, 0, st.Plan.PendingCount)
	require.Equal(t, 0, st.Risk.AcceptedCount)
	require.Equal(t, 2, st.Risk.FilteredCount)
	require.Equal(t, "reject", st.Risk.Status)
	require.Equal(t, "skipped", st.Execution.Phase)
	require.False(t, st.Execution.Ready)
	require.Contains(t, st.BlockReasons, "all_plan_items_filtered_or_skipped")
	require.Equal(t, "all_plan_items_filtered_or_skipped", st.BlockReason)
}

func TestPhase15_GetDailyTradingStatus_PlanStuckExecuting(t *testing.T) {
	phase15AcceptanceSetup(t)

	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-08-02", GeneratedAt: now, Source: "strategy_run",
		Status: models.CandidatePoolStatusReady, ItemCount: 1,
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{
		{TradeDate: "2026-08-02", StockCode: "sz000001", Rank: 1, Score: 0.9},
	}))

	plan := &models.TradePlan{
		TradeDate: "2026-08-02", PoolID: pool.ID, GeneratedAt: now,
		Status: models.TradePlanStatusExecuting, RiskStatus: "pass",
		RiskAcceptedCount: 2, RiskFilteredCount: 0,
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Status: models.TradePlanItemPending, RiskCode: "APPROVED"},
		{StockCode: "sh600519", Status: models.TradePlanItemPending, RiskCode: "APPROVED"},
	}))

	st := GetDailyTradingStatusForDate("2026-08-02")
	require.Equal(t, models.TradePlanStatusExecuting, st.Plan.Status)
	require.Equal(t, 2, st.Plan.PendingCount)
	require.Equal(t, "executing", st.Execution.Phase)
	require.False(t, st.Execution.Ready)
	require.Contains(t, st.BlockReasons, "plan_stuck_executing")
	require.Equal(t, "plan_stuck_executing", st.BlockReason)
	require.Equal(t, 0, st.Paper.FillCount)
}

func TestPhase15_GetDailyTradingStatus_PaperFilled(t *testing.T) {
	phase15AcceptanceSetup(t)

	now := time.Now()
	filledAt := time.Date(2026, 8, 3, 9, 31, 0, 0, time.Local)

	pool := &models.CandidatePool{
		TradeDate: "2026-08-03", GeneratedAt: now, Source: "strategy_run",
		Status: models.CandidatePoolStatusReady, ItemCount: 1,
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{
		{TradeDate: "2026-08-03", StockCode: "sz000001", Rank: 1, Score: 0.9},
	}))

	order := PaperOrder{
		AccountID: 1, StockCode: "sz000001", StockName: "平安", Side: "buy",
		Status: PaperOrderStatusFilled, Price: 10, Volume: 1000, FilledPrice: 10, FilledVol: 1000,
		StrategyTag: models.PaperStrategyTagTradePlan, CreatedAt: filledAt, UpdatedAt: filledAt,
	}
	require.NoError(t, db.Dao.Create(&order).Error)
	fill := PaperFill{
		AccountID: 1, OrderID: order.ID, StockCode: "sz000001", StockName: "平安", Side: "buy",
		Price: 10, Volume: 1000, StrategyTag: models.PaperStrategyTagTradePlan, FilledAt: filledAt,
	}
	require.NoError(t, db.Dao.Create(&fill).Error)

	plan := &models.TradePlan{
		TradeDate: "2026-08-03", PoolID: pool.ID, GeneratedAt: now,
		Status: models.TradePlanStatusDone, RiskStatus: "pass",
		RiskAcceptedCount: 1, RiskFilteredCount: 0,
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{
			StockCode: "sz000001", Status: models.TradePlanItemFilled, RiskCode: "APPROVED",
			OrderID: order.ID, FillID: fill.ID, FilledPrice: 10, FilledVolume: 1000,
		},
	}))

	st := GetDailyTradingStatusForDate("2026-08-03")
	require.Equal(t, models.TradePlanStatusDone, st.Plan.Status)
	require.Equal(t, 1, st.Plan.FilledCount)
	require.Equal(t, "done", st.Execution.Phase)
	require.Equal(t, 1, st.Paper.OrderCount)
	require.Equal(t, 1, st.Paper.FillCount)
	require.Equal(t, 1, st.Paper.FilledOrderCount)
	require.Empty(t, st.BlockReason, "成交完成不应再阻断")
	require.Empty(t, st.BlockReasons)
	require.Contains(t, st.Message, "execution=done")
	require.False(t, st.Execution.Ready, "已完成执行不应再 ready")
}
