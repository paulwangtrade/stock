package strategy

import (
	"errors"
	"os"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/positionsizing"

	"github.com/stretchr/testify/require"
)

func p1Snap(cash, equity, exposure float64) *portfolio.Snapshot {
	return &portfolio.Snapshot{
		Found:         true,
		Cash:          cash,
		AvailableCash: cash,
		TotalEquity:   equity,
		MarketValue:   exposure,
		TotalExposure: exposure,
	}
}

func p1Cleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ResetDraftPortfolioSnapshotForTest()
		papertrading.ResetConfigCache()
		positionsizing.ResetDefaultSizer()
		positionsizing.ResetLogAppliedForTest()
	})
}

func TestP1SizerMode_CaseA_FixedAmountThreeNames(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true})

	pool := seedReadyPool(t, "2026-08-20", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, float64(100_000), plan.AmountPerStock)
	require.GreaterOrEqual(t, len(plan.Items), 3)
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			require.Equal(t, float64(100_000), it.TargetAmount, it.StockCode)
		}
	}
}

func TestP1SizerMode_CaseB_PortfolioAwareSplit(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModePortfolioAware,
	})
	SetDraftPortfolioSnapshotForTest(p1Snap(400_000, 1_000_000, 0), nil)

	pool := seedReadyPool(t, "2026-08-20", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, float64(133_333), plan.AmountPerStock)
	require.LessOrEqual(t, plan.AmountPerStock*3, 400_000.0)
	require.Greater(t, plan.AmountPerStock, 0.0)
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			require.Equal(t, float64(133_333), it.TargetAmount, it.StockCode)
		}
	}
}

func TestP1SizerMode_CaseC_NearFullDoesNotEmit100000(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModePortfolioAware,
	})
	SetDraftPortfolioSnapshotForTest(p1Snap(400_000, 1_000_000, 830_000), nil)

	pool := seedReadyPool(t, "2026-08-20", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, float64(6_666), plan.AmountPerStock)
	require.NotEqual(t, float64(100_000), plan.AmountPerStock)
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			require.NotEqual(t, float64(100_000), it.TargetAmount, it.StockCode)
		}
	}
}

func TestP1SizerMode_CaseD_NoCashSkipsFilterFallback(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModePortfolioAware,
	})
	SetDraftPortfolioSnapshotForTest(p1Snap(0, 1_000_000, 0), nil)

	pool := seedReadyPool(t, "2026-08-20", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoBudget), err.Error())
	require.Nil(t, plan)

	_, listErr := data.NewTradePlanRepo().GetLatestByTradeDate("2026-08-20")
	require.Error(t, listErr, "no_budget must not persist a plan (FilterPool would have written 100000)")
}

func TestP1SizerMode_CaseE_SwitchOffMatchesLegacy(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModeFixedAmount,
	})
	// Snapshot would change amounts if the switch leaked; it must be ignored.
	SetDraftPortfolioSnapshotForTest(p1Snap(1, 1, 0), nil)

	pool := seedReadyPool(t, "2026-08-20", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, float64(100_000), plan.AmountPerStock)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.False(t, plan.EnableExecute)
	require.NotContains(t, plan.Message, "raw_amount")
	require.NotContains(t, plan.Message, "capped_by")
}

func TestP2SizerMode_CaseC_SingleWeightCap(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModePortfolioAware,
	})
	SetDraftPortfolioSnapshotForTest(p1Snap(1_000_000, 1_000_000, 0), nil)

	pool := seedReadyPool(t, "2026-08-20", "sz000001")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, float64(200_000), plan.AmountPerStock)
	require.NotEqual(t, float64(850_000), plan.AmountPerStock)
	require.NotContains(t, plan.Message, `"method"`)
}

func TestP2SizerMode_CaseE_FullBookNoPlan(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModePortfolioAware,
	})
	SetDraftPortfolioSnapshotForTest(p1Snap(400_000, 1_000_000, 900_000), nil)

	pool := seedReadyPool(t, "2026-08-20", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoBudget), err.Error())
	require.Contains(t, err.Error(), "insufficient_allocation")
	require.Nil(t, plan)
	_, listErr := data.NewTradePlanRepo().GetLatestByTradeDate("2026-08-20")
	require.Error(t, listErr)
}

func TestP1SizerMode_FilterPoolZeroFallbackUnchanged(t *testing.T) {
	raw, err := os.ReadFile("plan_risk_bridge.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "if amount <= 0 {")
	require.Contains(t, src, "amount = 100_000")
}
