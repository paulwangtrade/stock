package approvegate

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func seedSellHolding(t *testing.T, code string, total, avail, locked int64) {
	t.Helper()
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
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

func TestPureSellDraft_ReadinessApproveFreeze(t *testing.T) {
	setupApproveWriteTestDB(t)
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))
	seedSellHolding(t, "sz000001", 1000, 1000, 0)

	plan, err := strategy.BuildDraftTSellTradePlan(strategy.TSellDraftRequest{
		TradeDate: "2026-08-18",
		StockCode: "sz000001",
		Quantity:  100,
		Actor:     "tester",
	})
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Nil(t, plan.ApprovedAt)
	require.Nil(t, plan.FreezeAt)

	rd := readiness.EvaluateExecutionIntentReadiness(plan, nil)
	require.True(t, rd.Ready, "blockers=%+v", rd.Blockers)

	approve, err := ApproveTradePlanByID(plan.ID, "tester", "unit_test", nil)
	require.NoError(t, err)
	require.True(t, approve.OK, "approve blockers=%+v", approve.Blockers)
	require.Equal(t, models.TradePlanStatusDraft, approve.Plan.Status)
	require.NotNil(t, approve.Plan.ApprovedAt)
	require.Nil(t, approve.Plan.FreezeAt)

	frozen, err := strategy.FreezeTradePlan(approve.Plan, "tester", "unit freeze")
	require.NoError(t, err)
	require.True(t, frozen.IsFrozen())
	require.Equal(t, models.TradePlanStatusReady, frozen.Status)
	require.Equal(t, "sell", frozen.Side)
	require.Equal(t, int64(100), frozen.Items[0].TargetVolume)
	require.Equal(t, models.TradePlanItemPending, frozen.Items[0].Status)
}
