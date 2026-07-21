package data

import (
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestGetDailyTradingStatus_EmptyDay(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	require.NoError(t, SavePaperOpenBuyConfig(PaperOpenBuyConfig{
		EnablePaperOpenBuy:     true,
		OpenBuyAmountPerStock:  100_000,
	}))

	st := GetDailyTradingStatusForDate("2026-07-20")
	require.Equal(t, "2026-07-20", st.TradeDate)
	require.True(t, st.EnablePaperOpenBuy)
	require.Equal(t, "EMPTY", st.Candidate.Status)
	require.Equal(t, 0, st.Candidate.Count)
	require.Equal(t, "EMPTY", st.Plan.Status)
	require.Equal(t, "N/A", st.Risk.Status)
	require.Equal(t, "not_started", st.Execution.Phase)
	require.False(t, st.Execution.Ready)
	require.Contains(t, st.BlockReasons, "candidate_pool_empty")
	require.Contains(t, st.BlockReasons, "trade_plan_missing")
	require.Equal(t, "candidate_pool_empty", st.BlockReason)
}

func TestGetDailyTradingStatus_ReadyPlan(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	require.NoError(t, SavePaperOpenBuyConfig(PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 100_000,
	}))

	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-07-21", GeneratedAt: now, Source: "strategy_run",
		Status: models.CandidatePoolStatusReady, ItemCount: 2, Message: "test",
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{
		{TradeDate: "2026-07-21", StockCode: "sz000001", StockName: "平安", Rank: 1, Score: 0.9},
		{TradeDate: "2026-07-21", StockCode: "sh600519", StockName: "茅台", Rank: 2, Score: 0.8},
	}))

	plan := &models.TradePlan{
		TradeDate: "2026-07-21", PoolID: pool.ID, GeneratedAt: now,
		Status: models.TradePlanStatusReady, EnableExecute: true,
		RiskStatus: "partial", MarketLevel: 3,
		RiskAcceptedCount: 1, RiskFilteredCount: 1, RiskSummary: "1 accepted",
		AmountPerStock: 100000,
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Status: models.TradePlanItemPending, RiskCode: "APPROVED"},
		{StockCode: "sh600519", Status: models.TradePlanItemSkipped, RiskCode: "SINGLE_NAME_EXCEEDED"},
	}))

	SetPlanItemExecutor(&noopPlanItemExecutor{})
	t.Cleanup(func() { SetPlanItemExecutor(nil) })

	st := GetDailyTradingStatusForDate("2026-07-21")
	require.Equal(t, "READY", st.Candidate.Status)
	require.Equal(t, 2, st.Candidate.Count)
	require.Equal(t, models.TradePlanStatusReady, st.Plan.Status)
	require.Equal(t, 2, st.Plan.ItemCount)
	require.Equal(t, 1, st.Plan.PendingCount)
	require.Equal(t, 1, st.Plan.SkippedCount)
	require.Equal(t, "partial", st.Risk.Status)
	require.Equal(t, 1, st.Risk.AcceptedCount)
	require.Equal(t, 1, st.Risk.FilteredCount)
	require.Equal(t, "ready", st.Execution.Phase)
	require.True(t, st.Execution.ExecutorConfigured)
	require.True(t, st.Execution.Ready)
	require.Empty(t, st.BlockReason)
}

func TestGetDailyTradingStatus_AllFiltered(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	require.NoError(t, SavePaperOpenBuyConfig(PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 100_000,
	}))

	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-07-22", GeneratedAt: now, Source: "strategy_run",
		Status: models.CandidatePoolStatusReady, ItemCount: 1,
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{
		{TradeDate: "2026-07-22", StockCode: "sz000001", Rank: 1, Score: 0.5},
	}))

	plan := &models.TradePlan{
		TradeDate: "2026-07-22", PoolID: pool.ID, GeneratedAt: now,
		Status: models.TradePlanStatusReady, RiskStatus: "reject",
		RiskAcceptedCount: 0, RiskFilteredCount: 1,
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Status: models.TradePlanItemSkipped, RiskCode: "GROSS_EXPOSURE"},
	}))

	st := GetDailyTradingStatusForDate("2026-07-22")
	require.Contains(t, st.BlockReasons, "all_plan_items_filtered_or_skipped")
	require.Equal(t, "skipped", st.Execution.Phase)
	require.False(t, st.Execution.Ready)
}

type noopPlanItemExecutor struct{}

func (noopPlanItemExecutor) ExecutePlanItem(item models.TradePlanItem, opts PlanItemExecOpts) (*PaperOrder, error) {
	return nil, nil
}
