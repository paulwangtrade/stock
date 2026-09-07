package execution

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func scenarioSpecItem(vol int64) models.TradePlanItem {
	return models.TradePlanItem{
		TradeDate:    "2026-08-03",
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		LimitPrice:   10.0,
		TargetVolume: vol,
		TargetAmount: float64(vol) * 10,
		IntentStatus: "priced",
		RefPrice:     9.8,
		EntryRule:    "limit",
		Status:       models.TradePlanItemPending,
	}
}

func TestPaperScenario_PartialThenFilled(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)
	item := scenarioSpecItem(1000)
	eng := NewPaperScenarioEngine(NewPaperBroker(nil))
	ctx := context.Background()

	sub, err := eng.SubmitFromSpec(ctx, item, acc.ID, "partial-chain")
	require.NoError(t, err)
	require.Equal(t, PaperLogicSubmitted, sub.LogicStatus)
	require.Equal(t, data.PaperOrderStatusPending, sub.OMSStatus)
	require.Equal(t, int64(0), sub.FilledQty)
	require.Equal(t, int64(1000), sub.RemainingQty)
	oid := sub.Order.ID

	p1, err := eng.PartialFill(ctx, oid, 10.0, 300, item.LimitPrice, item.TargetVolume)
	require.NoError(t, err)
	require.Equal(t, PaperLogicPartial, p1.LogicStatus)
	require.Equal(t, data.PaperOrderStatusPending, p1.OMSStatus)
	require.Equal(t, int64(300), p1.FilledQty)
	require.Equal(t, int64(700), p1.RemainingQty)

	var pos data.PaperPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, item.StockCode).First(&pos).Error)
	require.Equal(t, int64(300), pos.Volume)

	p2, err := eng.PartialFill(ctx, oid, 10.0, 700, item.LimitPrice, item.TargetVolume)
	require.NoError(t, err)
	require.Equal(t, PaperLogicFilled, p2.LogicStatus)
	require.Equal(t, data.PaperOrderStatusFilled, p2.OMSStatus)
	require.Equal(t, int64(1000), p2.FilledQty)
	require.Equal(t, int64(0), p2.RemainingQty)

	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, item.StockCode).First(&pos).Error)
	require.Equal(t, int64(1000), pos.Volume)

	_, err = eng.PartialFill(ctx, oid, 10.0, 100, item.LimitPrice, item.TargetVolume)
	require.Error(t, err, "filled order must not accept further fill / rollback")
}

func TestPaperScenario_RejectLeavesPositionUnchanged(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)
	cashBefore := acc.Cash
	item := scenarioSpecItem(1000)
	eng := NewPaperScenarioEngine(NewPaperBroker(nil))
	ctx := context.Background()

	sub, err := eng.SubmitFromSpec(ctx, item, acc.ID, "reject-sim")
	require.NoError(t, err)

	rej, err := eng.RejectSim(ctx, sub.Order.ID, RejectReasonLiquidity, "no liquidity", item.LimitPrice, item.TargetVolume)
	require.NoError(t, err)
	require.Equal(t, PaperLogicRejected, rej.LogicStatus)
	require.Equal(t, data.PaperOrderStatusRejected, rej.OMSStatus)
	require.Equal(t, RejectReasonLiquidity, rej.Order.RejectCode)

	var posCount int64
	require.NoError(t, db.Dao.Model(&data.PaperPosition{}).Where("account_id = ?", acc.ID).Count(&posCount).Error)
	require.Zero(t, posCount)

	var after data.PaperAccount
	require.NoError(t, db.Dao.First(&after, acc.ID).Error)
	require.InDelta(t, cashBefore, after.Cash, 1e-9)
}

func TestPaperScenario_CancelThenFillRejected(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)
	item := scenarioSpecItem(1000)
	eng := NewPaperScenarioEngine(NewPaperBroker(nil))
	ctx := context.Background()

	sub, err := eng.SubmitFromSpec(ctx, item, acc.ID, "cancel-guard")
	require.NoError(t, err)

	can, err := eng.Cancel(ctx, sub.Order.ID, item.LimitPrice, item.TargetVolume)
	require.NoError(t, err)
	require.Equal(t, PaperLogicCancelled, can.LogicStatus)
	require.Equal(t, data.PaperOrderStatusCancelled, can.OMSStatus)

	_, err = eng.PartialFill(ctx, sub.Order.ID, 10.0, 100, item.LimitPrice, item.TargetVolume)
	require.Error(t, err)
}

func TestPaperScenario_SpecImmutableThroughLifecycle(t *testing.T) {
	setupCutoverTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	acc := seedCashForFillEngine(t, 1_000_000)

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-08-03", GeneratedAt: now, Status: models.TradePlanStatusReady,
		PlanVersion: 1, AmountPerStock: 100_000, Side: "buy",
		EnableExecute: true, ApprovedAt: &now, ApprovedBy: "scenario-spec", FreezeAt: &now, FreezeBy: "scenario-spec",
	}
	item := scenarioSpecItem(1000)
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{item}))
	before, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	bi := before.Items[0]
	snap := struct {
		Limit, Ref float64
		Vol        int64
		Intent, Entry, Status string
	}{bi.LimitPrice, bi.RefPrice, bi.TargetVolume, bi.IntentStatus, bi.EntryRule, bi.Status}

	eng := NewPaperScenarioEngine(NewPaperBroker(nil))
	ctx := context.Background()
	sub, err := eng.SubmitFromSpec(ctx, bi, acc.ID, "spec-imm")
	require.NoError(t, err)
	_, err = eng.PartialFill(ctx, sub.Order.ID, 10.1, 300, bi.LimitPrice, bi.TargetVolume)
	require.NoError(t, err)
	_, err = eng.PartialFill(ctx, sub.Order.ID, 10.0, 700, bi.LimitPrice, bi.TargetVolume)
	require.NoError(t, err)

	after, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	ai := after.Items[0]
	require.Equal(t, snap.Limit, ai.LimitPrice)
	require.Equal(t, snap.Vol, ai.TargetVolume)
	require.Equal(t, snap.Intent, ai.IntentStatus)
	require.Equal(t, snap.Entry, ai.EntryRule)
	require.Equal(t, snap.Ref, ai.RefPrice)
	require.Equal(t, snap.Status, ai.Status)
	require.Equal(t, before.Status, after.Status)
}

func TestPaperScenario_RejectReasons(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)
	eng := NewPaperScenarioEngine(NewPaperBroker(nil))
	ctx := context.Background()

	for _, reason := range []string{RejectReasonPrice, RejectReasonLiquidity, RejectReasonBroker} {
		item := scenarioSpecItem(1000)
		item.StockCode = "sz00000" + reason[:1] // unique-ish codes not required; new order each time
		sub, err := eng.SubmitFromSpec(ctx, item, acc.ID, reason)
		require.NoError(t, err)
		rej, err := eng.RejectSim(ctx, sub.Order.ID, reason, reason+" msg", item.LimitPrice, item.TargetVolume)
		require.NoError(t, err)
		require.Equal(t, reason, rej.Order.RejectCode)
	}
}
