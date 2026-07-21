package data

import (
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestGetTradePlanAnalysis_Chain(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())

	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-07-17", GeneratedAt: now, Source: "follow",
		Status: models.CandidatePoolStatusReady, Message: "test",
	}
	poolItems := []models.CandidatePoolItem{
		{TradeDate: "2026-07-17", StockCode: "sz000001", StockName: "平安", Rank: 1, Score: 0.9,
			StrategyName: "s1", StrategyVersion: "v1", SignalTag: "强", SignalScore: 1, SignalSnapshotID: 7},
		{TradeDate: "2026-07-17", StockCode: "sh600519", StockName: "茅台", Rank: 2, Score: 0.7,
			StrategyName: "s1", StrategyVersion: "v1", SignalTag: "", SignalScore: 0},
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, poolItems))

	plan := &models.TradePlan{
		TradeDate: "2026-07-17", PoolID: pool.ID, GeneratedAt: now,
		Status: models.TradePlanStatusReady, RiskStatus: "partial", MarketLevel: 3,
		RiskAcceptedCount: 1, RiskFilteredCount: 1, RiskSummary: "test",
		AmountPerStock: 100000,
	}
	planItems := []models.TradePlanItem{
		{StockCode: "sz000001", StockName: "平安", Priority: 1, Status: models.TradePlanItemPending,
			Score: 0.9, StrategyName: "s1", StrategyVersion: "v1", RiskCode: "APPROVED", TargetAmount: 100000},
		{StockCode: "sh600519", StockName: "茅台", Priority: 2, Status: models.TradePlanItemSkipped,
			Score: 0.7, StrategyName: "s1", StrategyVersion: "v1",
			RiskCode: "SINGLE_NAME_EXCEEDED", RiskMessage: "单票仓位超过20%", TargetAmount: 100000},
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, planItems))

	ana, err := NewTradeAnalysisRepo().GetTradePlanAnalysis("2026-07-17")
	require.NoError(t, err)
	require.NotNil(t, ana.Pool)
	require.NotNil(t, ana.Plan)
	require.Equal(t, pool.ID, ana.Pool.ID)
	require.Equal(t, plan.ID, ana.Plan.ID)
	require.GreaterOrEqual(t, len(ana.Items), 2)

	var skipped *AnalysisChainItem
	for i := range ana.Items {
		if ana.Items[i].StockCode == "sh600519" {
			skipped = &ana.Items[i]
			break
		}
	}
	require.NotNil(t, skipped)
	require.Contains(t, skipped.WhyNotBought, "SINGLE_NAME_EXCEEDED")
	require.Equal(t, "强", ana.Items[0].SignalTag)

	perf, err := NewTradeAnalysisRepo().GetStrategyPerformance()
	require.NoError(t, err)
	require.NotEmpty(t, perf)
}
