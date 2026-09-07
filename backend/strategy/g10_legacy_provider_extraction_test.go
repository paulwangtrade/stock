package strategy

import (
	"os"
	"strings"
	"testing"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/models"
	"go-stock/backend/risk"
	"go-stock/backend/selection"

	"github.com/stretchr/testify/require"
)

func g10PoolItems() []models.CandidatePoolItem {
	return []models.CandidatePoolItem{
		{StockCode: "sz000005", StockName: "E", Rank: 5, Score: 10, Reason: "re", StrategyName: "s", StrategyVersion: "v1"},
		{StockCode: "sz000001", StockName: "A", Rank: 1, Score: 90, Reason: "ra", StrategyName: "s", StrategyVersion: "v1"},
		{StockCode: "sz000003", StockName: "C", Rank: 3, Score: 50, Reason: "rc", StrategyName: "s", StrategyVersion: "v1"},
		{StockCode: "sz000002", StockName: "B", Rank: 2, Score: 80, Reason: "rb", StrategyName: "s", StrategyVersion: "v1"},
		{StockCode: "sz000004", StockName: "D", Rank: 4, Score: 20, Reason: "rd", StrategyName: "s", StrategyVersion: "v1"},
	}
}

// preExtractionPlanCandidates is the G.10 golden: Select + same-amount copy (old FilterPool body).
func preExtractionPlanCandidates(items []models.CandidatePoolItem, amount float64, maxNames int) ([]risk.PlanCandidate, int) {
	if maxNames <= 0 {
		maxNames = defaultMaxPlanNames
	}
	result := selection.Select(poolItemsToSelectionCandidates(items), selection.SelectionContext{
		MaxSelectedNames: maxNames,
	})
	ranked := result.RankedForPlanFilter()
	out := make([]risk.PlanCandidate, 0, len(ranked))
	for _, c := range ranked {
		src := matchPoolItem(items, c)
		code := strings.TrimSpace(src.StockCode)
		if code == "" {
			code = c.StockCode
		}
		name := strings.TrimSpace(src.StockName)
		if name == "" {
			name = c.StockName
		}
		reason := strings.TrimSpace(src.Reason)
		if reason == "" {
			reason = c.Reason
		}
		out = append(out, risk.PlanCandidate{
			StockCode:       code,
			StockName:       name,
			Rank:            c.Rank,
			Score:           c.Score,
			Reason:          reason,
			StrategyName:    src.StrategyName,
			StrategyVersion: src.StrategyVersion,
			TargetAmount:    amount,
		})
	}
	return out, result.SelectionLimit
}

func TestG10_EnvelopeMapsToSamePlanCandidates(t *testing.T) {
	items := g10PoolItems()
	const amount = 100_000.0
	const maxNames = 2

	want, wantLimit := preExtractionPlanCandidates(items, amount, maxNames)
	got, gotLimit, err := rankedPlanCandidatesFromPool(items, amount, maxNames)
	require.NoError(t, err)
	require.Equal(t, wantLimit, gotLimit)
	require.Equal(t, len(want), len(got))
	for i := range want {
		require.Equal(t, want[i].StockCode, got[i].StockCode, "i=%d symbol", i)
		require.Equal(t, want[i].TargetAmount, got[i].TargetAmount, "i=%d amount", i)
		require.Equal(t, want[i].Rank, got[i].Rank, "i=%d rank/order", i)
		require.Equal(t, want[i].Reason, got[i].Reason, "i=%d reason", i)
		require.Equal(t, want[i].StrategyName, got[i].StrategyName)
		require.Equal(t, want[i].StockName, got[i].StockName)
		require.Equal(t, want[i].Score, got[i].Score)
	}
	require.Equal(t, []int{1, 2, 3, 4, 5}, []int{got[0].Rank, got[1].Rank, got[2].Rank, got[3].Rank, got[4].Rank})
	for _, c := range got {
		require.Equal(t, amount, c.TargetAmount)
	}
}

func TestG10_FilterPoolRejectCodesMatchPreExtraction(t *testing.T) {
	ctx := r1WideRisk(1_000_000, 2, 5)
	ctx.LongMarketValue = 150_000
	ctx.NameMarketValue = map[string]float64{"sz000001": 150_000}
	stubPlanFilterContext(t, ctx)

	pool := &models.CandidatePool{Items: g10PoolItems()}
	res, err := FilterPoolForTradePlan(pool, 100_000, 2)
	require.NoError(t, err)

	wantCands, _ := preExtractionPlanCandidates(pool.Items, 100_000, 2)
	want := risk.PlanFilter(wantCands, ctx)
	require.Equal(t, len(want.Items), len(res.Items))
	for i := range want.Items {
		require.Equal(t, want.Items[i].RiskCode, res.Items[i].RiskCode, "i=%d code=%s", i, res.Items[i].Candidate.StockCode)
		require.Equal(t, want.Items[i].Status, res.Items[i].Status)
		require.Equal(t, want.Items[i].Candidate.Rank, res.Items[i].Candidate.Rank)
		require.Equal(t, want.Items[i].Candidate.TargetAmount, res.Items[i].Candidate.TargetAmount)
	}
	require.Equal(t, risk.ReasonSingleNameExceeded, res.Items[0].RiskCode)
}

func TestG10_FilterPoolZeroAmountStillRewrites100000(t *testing.T) {
	stubPlanFilterContext(t, r1WideRisk(1_000_000, 5, 5))
	pool := &models.CandidatePool{Items: []models.CandidatePoolItem{
		{StockCode: "sz000001", StockName: "A", Rank: 1, Score: 90},
	}}
	res, err := FilterPoolForTradePlan(pool, 0, 5)
	require.NoError(t, err)
	require.NotEmpty(t, res.Items)
	require.Equal(t, 100_000.0, res.Items[0].Candidate.TargetAmount)
}

func TestG10_DraftTradePlanAmountAndOrderUnchanged(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-08-20", "sz000002", "sz000001", "sz000003")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
	require.GreaterOrEqual(t, len(plan.Items), 3)

	filtered, err := FilterPoolForTradePlan(pool, plan.AmountPerStock, defaultMaxPlanNames)
	require.NoError(t, err)
	require.Equal(t, len(filtered.Items), len(plan.Items))
	for i := range plan.Items {
		require.Equal(t, filtered.Items[i].Candidate.StockCode, plan.Items[i].StockCode, "i=%d", i)
		require.Equal(t, filtered.Items[i].Candidate.TargetAmount, plan.Items[i].TargetAmount)
		require.Equal(t, 100_000.0, plan.Items[i].TargetAmount)
	}
}

func TestG10_NoPortfolioProviderOnBridge(t *testing.T) {
	require.Equal(t, decisionprovider.ProviderFixedAmount, legacyDecisionProvider.Name())
	src, err := os.ReadFile("plan_risk_bridge.go")
	require.NoError(t, err)
	require.NotContains(t, string(src), "NewPortfolioDecisionProvider")
	require.NotContains(t, string(src), "ProviderPortfolioAllocation")
}
