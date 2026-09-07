package strategy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/readiness"

	"github.com/stretchr/testify/require"
)

func setupTSellDraftTestDB(t *testing.T) {
	t.Helper()
	setupDraftPlanTestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
}

func seedPaperSimHolding(t *testing.T, code string, total, avail, locked int64) {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000, Equity: 1_000_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	pos := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: code, StockName: "测试",
		TotalVolume: total, AvailableVolume: avail, LockedVolume: locked,
		AvgCost: 10, MarkPrice: 10, UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(&pos).Error)
}

func TestBuildDraftTSellTradePlan_AvailableSufficient(t *testing.T) {
	setupTSellDraftTestDB(t)
	seedPaperSimHolding(t, "sz000001", 1000, 1000, 0)

	plan, err := BuildDraftTSellTradePlan(TSellDraftRequest{
		TradeDate: "2026-08-18",
		StockCode: "sz000001",
		Quantity:  100,
		Actor:     "test",
		Reason:    "T manual sell",
	})
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotZero(t, plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, "sell", plan.Side)
	require.False(t, plan.EnableExecute)
	require.Nil(t, plan.ApprovedAt)
	require.Nil(t, plan.FreezeAt)
	require.Equal(t, models.TradePlanSourceTSell, plan.SourceSession)
	require.Equal(t, 1, plan.PricingPolicyVersion)
	require.Len(t, plan.Items, 1)
	require.Equal(t, "sell", plan.Items[0].Side)
	require.Equal(t, int64(100), plan.Items[0].TargetVolume)
	require.Equal(t, 0.0, plan.Items[0].TargetAmount)
	require.Equal(t, models.TradePlanItemPending, plan.Items[0].Status)
}

func TestBuildDraftTSellTradePlan_ExitReviewSourceSession(t *testing.T) {
	setupTSellDraftTestDB(t)
	seedPaperSimHolding(t, "sz000001", 1000, 1000, 0)

	plan, err := BuildDraftTSellTradePlan(TSellDraftRequest{
		TradeDate:     "2026-08-18",
		StockCode:     "sz000001",
		Quantity:      100,
		Actor:         "ui:exit-review-sell",
		Reason:        "exit review: loss threshold",
		SourceSession: models.TradePlanSourceExitReview,
	})
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, models.TradePlanSourceExitReview, plan.SourceSession)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, "exit review: loss threshold", plan.Items[0].Reason)
}

func TestBuildDraftTSellTradePlan_AvailableInsufficient(t *testing.T) {
	setupTSellDraftTestDB(t)
	seedPaperSimHolding(t, "sz000001", 1000, 200, 800)

	_, err := BuildDraftTSellTradePlan(TSellDraftRequest{
		TradeDate: "2026-08-18",
		StockCode: "sz000001",
		Quantity:  500,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSellInsufficientAvailable))

	var n int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestBuildDraftTSellTradePlan_CannotSell(t *testing.T) {
	setupTSellDraftTestDB(t)
	seedPaperSimHolding(t, "sz000001", 1000, 0, 1000)

	_, err := BuildDraftTSellTradePlan(TSellDraftRequest{
		TradeDate: "2026-08-18",
		StockCode: "sz000001",
		Quantity:  100,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSellNotSellable))

	var n int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestBuildDraftTSellTradePlan_InvalidStock(t *testing.T) {
	setupTSellDraftTestDB(t)
	seedPaperSimHolding(t, "sz000001", 1000, 1000, 0)

	_, err := BuildDraftTSellTradePlan(TSellDraftRequest{
		TradeDate: "2026-08-18",
		StockCode: "sh600000",
		Quantity:  100,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSellNoPosition) || errors.Is(err, ErrSellInvalidStock))
}

func TestBuildDraftTSellTradePlan_PureSellReadiness(t *testing.T) {
	setupTSellDraftTestDB(t)
	seedPaperSimHolding(t, "sz000001", 1000, 1000, 0)

	plan, err := BuildDraftTSellTradePlan(TSellDraftRequest{
		TradeDate: "2026-08-18",
		StockCode: "sz000001",
		Quantity:  100,
		Actor:     "test",
	})
	require.NoError(t, err)

	rd := readiness.EvaluateExecutionIntentReadiness(plan, nil)
	require.True(t, rd.Ready, "pure sell draft should pass readiness: %+v", rd.Blockers)
	require.Equal(t, 1, rd.TradeableCount)

	riskRes, err := EvaluateDraftTradePlanRisk(plan)
	require.NoError(t, err)
	require.True(t, riskRes.Passed)
}

func TestBuildDraftTSell_BuyDraftRegression(t *testing.T) {
	setupTSellDraftTestDB(t)
	pool := seedReadyPool(t, "2026-08-18", "sz000001")

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.NotEqual(t, "sell", strings.ToLower(strings.TrimSpace(plan.Side)))
	require.Nil(t, plan.ApprovedAt)
	require.Nil(t, plan.FreezeAt)
	require.Equal(t, models.TradePlanSourceAfterClose, plan.SourceSession)
	require.GreaterOrEqual(t, len(plan.Items), 1)
	require.NotEqual(t, "sell", strings.ToLower(strings.TrimSpace(plan.Items[0].Side)))
}

func TestBuildDraftTSellTradePlan_SourceBoundary(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("build_draft_t_sell_plan.go"))
	require.NoError(t, err)
	src := string(body)
	require.Contains(t, src, "CheckSellPositionGate")
	require.Contains(t, src, `Side:                 "sell"`)
	require.NotContains(t, src, "ApproveTradePlanByID")
	require.NotContains(t, src, "FreezeTradePlan(")
	require.NotContains(t, src, "RunExecution")
	require.NotContains(t, src, "fillSell(")
	require.NotContains(t, src, "SettleNewTradingDay(")
}
