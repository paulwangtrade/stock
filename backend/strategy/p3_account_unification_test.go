package strategy

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func setupP3AccountUnificationDB(t *testing.T) {
	t.Helper()
	setupDraftPlanTestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
}

func seedLegacyPaperAccountCashOnly(t *testing.T, cash float64) {
	t.Helper()
	acc := data.PaperAccount{
		Name:            "默认模拟账户",
		Cash:            cash,
		InitialCash:     cash,
		Equity:          cash,
		ValuationStatus: "complete",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	require.NoError(t, db.Dao.Create(&acc).Error)
	var n int64
	require.NoError(t, db.Dao.Model(&data.PaperPosition{}).Count(&n).Error)
	require.Equal(t, int64(0), n, "legacy paper_positions must stay empty")
}

func seedPaperSimCash(t *testing.T, cash float64) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: cash, Cash: cash, Equity: cash,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func enablePlanFilterForCashProbe(t *testing.T) {
	t.Helper()
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:       false,
		OpenBuyAmountPerStock:    100_000,
		EnableRiskFilter:         true,
		PlanMarketLevel:          3,
		BlockNewEntriesOnDefense: true,
		// Snapshot equity = sim cash (no holdings). Caps must not bind before cash:
		// 2nd name postGross=200000 / equity=150000 ≈ 1.33.
		MaxGrossExposurePct: 2.0,
		MaxSingleNamePct:    0.90,
	}))
	t.Cleanup(func() {
		_ = data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
			EnablePaperOpenBuy:    false,
			OpenBuyAmountPerStock: 100_000,
			EnableRiskFilter:      false,
		})
	})
}

// Case A: paper_sim 有仓、legacy 无仓 → 默认 Materialize loader 必须 size_skip。
func TestP3A_CaseA_MaterializeSkipsSimHoldingWhenLegacyEmpty(t *testing.T) {
	setupP3AccountUnificationDB(t)
	seedLegacyPaperAccountCashOnly(t, 1_000_000)
	seedPaperSimHolding(t, "sz000001", 1000, 1000, 0)

	plan := seedDraftPlanPriced(t, "2026-08-18", []models.TradePlanItem{{
		TradeDate: "2026-08-18", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})

	res, err := MaterializeMorningTargetVolumes(plan.ID, nil)
	require.NoError(t, err)
	require.False(t, res.NoOp)
	require.Equal(t, 0, res.SizedCount, "legacy-empty book must not size a sim holding")
	require.Equal(t, 1, res.SizeSkipCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, morningIntentStatusSizeSkip, got.Items[0].IntentStatus)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)
}

// Case B: sim cash=150000 vs legacy cash=1e6 → PlanFilter 用 paper_sim。
func TestP3A_CaseB_PlanFilterUsesPaperSimCash(t *testing.T) {
	setupP3AccountUnificationDB(t)
	p1Cleanup(t)
	enablePlanFilterForCashProbe(t)
	seedLegacyPaperAccountCashOnly(t, 1_000_000)
	seedPaperSimCash(t, 150_000)

	pool := seedReadyPool(t, "2026-08-18", "sz000001", "sh600519", "sh601127")
	res, err := FilterPoolForTradePlan(pool, 100_000, 3)
	require.NoError(t, err)
	require.Equal(t, 1, res.AcceptedCount, "sim 150k must accept only one 100k name")
	require.Equal(t, 2, res.FilteredCount)
	require.Equal(t, risk.PlanRiskStatusPartial, res.RiskStatus)
	require.Equal(t, "pending", res.Items[0].Status)
	for i := 1; i < len(res.Items); i++ {
		require.Equal(t, "skipped", res.Items[i].Status, res.Items[i].Candidate.StockCode)
		require.Equal(t, risk.ReasonCashInsufficient, res.Items[i].RiskCode, res.Items[i].Candidate.StockCode)
	}

	var snap risk.PlanSnapshot
	require.NoError(t, json.Unmarshal([]byte(res.RiskSnapshotJSON), &snap))
	require.InDelta(t, 150_000, snap.Cash, 1e-6, "PlanFilter cash must be paper_sim, not legacy 1e6")
	require.NotEqual(t, 1_000_000.0, snap.Cash)
}

// Case C: fixed_amount 仍每票 100000（不读账户）。
func TestP3A_CaseC_FixedAmountUnchanged(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true})

	pool := seedReadyPool(t, "2026-08-18", "sz000001", "sh600519", "sh601127")
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

// Case D: portfolio_aware Sizer 仍按注入 Snapshot 等权，不受 P3 loader 影响。
func TestP3A_CaseD_PortfolioAwareSizerUnchanged(t *testing.T) {
	setupDraftPlanTestDB(t)
	p1Cleanup(t)
	papertrading.SetConfigForTest(papertrading.Config{
		EnablePaperTrading: true,
		PositionSizerMode:  papertrading.PositionSizerModePortfolioAware,
	})
	SetDraftPortfolioSnapshotForTest(p1Snap(400_000, 1_000_000, 0), nil)

	pool := seedReadyPool(t, "2026-08-18", "sz000001", "sh600519", "sh601127")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, float64(133_333), plan.AmountPerStock)
	require.LessOrEqual(t, plan.AmountPerStock*3, 400_000.0)
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			require.Equal(t, float64(133_333), it.TargetAmount, it.StockCode)
		}
	}
}

func TestP3A_SourceBoundary_LoadersUsePortfolioSnapshot(t *testing.T) {
	for _, name := range []string{"plan_risk_bridge.go", "morning_position_materialize.go"} {
		raw, err := os.ReadFile(name)
		require.NoError(t, err)
		src := string(raw)
		require.Contains(t, src, "portfolio.NewService().Snapshot", name)
		require.NotContains(t, src, "NewPaperTradingApi()", name)
		require.NotContains(t, src, "GetSnapshot(0)", name)
	}
}
