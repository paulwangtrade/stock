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

func seedCashForFillEngine(t *testing.T, cash float64) *data.PaperAccount {
	t.Helper()
	data.EnsurePaperTradingTables()
	acc, err := data.NewPaperTradingApi().GetOrCreateDefaultAccount(cash)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Model(acc).Updates(map[string]any{
		"cash":         cash,
		"initial_cash": cash,
		"equity":       cash,
	}).Error)
	var got data.PaperAccount
	require.NoError(t, db.Dao.First(&got, acc.ID).Error)
	return &got
}

func frozenSpecItem() models.TradePlanItem {
	return models.TradePlanItem{
		TradeDate:    "2026-08-03",
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		LimitPrice:   10.0,
		TargetVolume: 1000,
		TargetAmount: 10_000,
		IntentStatus: "priced",
		RefPrice:     9.8,
		EntryRule:    "limit",
		Status:       models.TradePlanItemPending,
	}
}

func TestPaperFillEngine_FrozenSpecToPaperOrder(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)

	item := frozenSpecItem()
	eng := NewPaperFillEngine(NewPaperBroker(nil))
	res, err := eng.ExecuteFullFillFromSpec(context.Background(), item, PaperFillOpts{
		AccountID: acc.ID,
		Slippage:  0,
		Reason:    "fill-engine-mvp",
	})
	require.NoError(t, err)
	require.NotNil(t, res.Order)
	require.Equal(t, PaperLogicFilled, res.LogicStatus)
	require.Equal(t, data.PaperOrderStatusFilled, res.OMSStatus)
	require.Equal(t, data.PaperOrderStatusFilled, res.Order.Status)
	require.Equal(t, item.LimitPrice, res.Order.Price, "order price must equal Frozen limit_price")
	require.Equal(t, item.TargetVolume, res.Order.Volume)
	require.Equal(t, item.StockCode, res.Order.StockCode)
	require.Equal(t, "paper", res.Order.BrokerStatus)
	require.Empty(t, res.Order.BrokerOrderID)
}

func TestPaperFillEngine_FillUpdatesPosition(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)
	cashBefore := acc.Cash

	item := frozenSpecItem()
	eng := NewPaperFillEngine(NewPaperBroker(nil))
	res, err := eng.ExecuteFullFillFromSpec(context.Background(), item, PaperFillOpts{
		AccountID: acc.ID,
		Slippage:  0.05, // fill at 10.05
		Reason:    "pos-update",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1000), res.Fill.FillQty)
	require.InDelta(t, 10.05, res.Fill.FillPrice, 1e-9)
	require.InDelta(t, 0.05, res.Fill.Slippage, 1e-9)
	require.Greater(t, res.Fill.Commission, 0.0)

	var pos data.PaperPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, item.StockCode).First(&pos).Error)
	require.Equal(t, int64(1000), pos.Volume)
	require.InDelta(t, 10.05, pos.AvgCost, 1e-9)

	var after data.PaperAccount
	require.NoError(t, db.Dao.First(&after, acc.ID).Error)
	amount := 10.05 * 1000
	require.InDelta(t, cashBefore-(amount+res.Fill.Commission), after.Cash, 1e-6)
}

func TestPaperFillEngine_SlippageDoesNotPolluteLimitPrice(t *testing.T) {
	setupCutoverTestDB(t)
	acc := seedCashForFillEngine(t, 1_000_000)

	item := frozenSpecItem()
	limitBefore := item.LimitPrice
	volBefore := item.TargetVolume

	eng := NewPaperFillEngine(NewPaperBroker(nil))
	res, err := eng.ExecuteFullFillFromSpec(context.Background(), item, PaperFillOpts{
		AccountID: acc.ID,
		Slippage:  0.2,
	})
	require.NoError(t, err)
	require.InDelta(t, 10.2, res.Fill.FillPrice, 1e-9)
	require.InDelta(t, 0.2, res.Fill.Slippage, 1e-9)

	// In-memory Spec fields unchanged.
	require.Equal(t, limitBefore, item.LimitPrice)
	require.Equal(t, volBefore, item.TargetVolume)
	require.Equal(t, limitBefore, res.LimitPrice)
	require.Equal(t, limitBefore, res.Order.Price, "paper order keeps Spec limit; fill differs")
	require.NotEqual(t, res.Order.Price, res.Fill.FillPrice)
}

func TestPaperFillEngine_FillLeavesTradePlanSpecUntouched(t *testing.T) {
	setupCutoverTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	acc := seedCashForFillEngine(t, 1_000_000)

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:      "2026-08-03",
		GeneratedAt:    now,
		Status:         models.TradePlanStatusReady,
		PlanVersion:    1,
		AmountPerStock: 100_000,
		Side:           "buy",
		EnableExecute:  true,
		ApprovedAt:     &now,
		ApprovedBy:     "fill-engine-test",
		FreezeAt:       &now,
		FreezeBy:       "fill-engine-test",
	}
	item := frozenSpecItem()
	item.IntentStatus = "priced"
	item.RefPrice = 9.8
	item.EntryRule = "limit"
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{item}))

	before, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Len(t, before.Items, 1)
	bi := before.Items[0]
	snap := struct {
		LimitPrice   float64
		TargetVolume int64
		IntentStatus string
		RefPrice     float64
		EntryRule    string
		Status       string
	}{bi.LimitPrice, bi.TargetVolume, bi.IntentStatus, bi.RefPrice, bi.EntryRule, bi.Status}

	eng := NewPaperFillEngine(NewPaperBroker(nil))
	_, err = eng.ExecuteFullFillFromSpec(context.Background(), bi, PaperFillOpts{
		AccountID: acc.ID,
		Slippage:  0.1,
		Reason:    "no-plan-mutation",
	})
	require.NoError(t, err)

	after, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	ai := after.Items[0]
	require.Equal(t, snap.LimitPrice, ai.LimitPrice, "limit_price must stay Frozen")
	require.Equal(t, snap.TargetVolume, ai.TargetVolume, "target_volume must stay Frozen")
	require.Equal(t, snap.IntentStatus, ai.IntentStatus)
	require.Equal(t, snap.RefPrice, ai.RefPrice)
	require.Equal(t, snap.EntryRule, ai.EntryRule)
	require.Equal(t, snap.Status, ai.Status, "engine must not UpdateItemExecution")
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.FreezeAt.Unix(), after.FreezeAt.Unix())
}

func TestComputeFillPrice_AndSlippage(t *testing.T) {
	p, err := ComputeFillPrice("buy", 10, 0.05)
	require.NoError(t, err)
	require.InDelta(t, 10.05, p, 1e-9)
	require.InDelta(t, 0.05, ComputeSlippage("buy", 10, 10.05), 1e-9)

	p, err = ComputeFillPrice("sell", 10, 0.05)
	require.NoError(t, err)
	require.InDelta(t, 9.95, p, 1e-9)
	require.InDelta(t, 0.05, ComputeSlippage("sell", 10, 9.95), 1e-9)

	_, err = ComputeFillPrice("buy", 0, 0)
	require.Error(t, err)
}

func TestMapPaperLogicToOMS(t *testing.T) {
	require.Equal(t, "", MapPaperLogicToOMS(PaperLogicCreated))
	require.Equal(t, data.PaperOrderStatusPending, MapPaperLogicToOMS(PaperLogicSubmitted))
	require.Equal(t, data.PaperOrderStatusFilled, MapPaperLogicToOMS(PaperLogicFilled))
}
