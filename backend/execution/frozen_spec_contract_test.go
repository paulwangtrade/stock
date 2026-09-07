package execution

// Phase6.5.7.2.1 Frozen Spec Contract Tests.
// Tests only — does not modify Execution / Freeze / Strategy production logic.
//
// Contract (PHASE6_5_7_2_EXECUTION_SPEC_SOURCE_BOUNDARY_DESIGN.md):
//   Freeze 后 Order Spec（symbol/side/limit_price/target_volume）+ Intent 只读；
//   SubmitIntent 必须从 Frozen Spec 组装；禁止 live 重算回写 Spec。

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

// buildSubmitIntentFromFrozenSpec is the contract builder under test (test-local).
// Production open-buy path must converge to this rule in 6.5.7.2.2.
func buildSubmitIntentFromFrozenSpec(item models.TradePlanItem) SubmitIntent {
	side := item.Side
	if side == "" {
		side = "buy"
	}
	return SubmitIntent{
		StockCode:   item.StockCode,
		StockName:   item.StockName,
		Side:        side,
		Price:       item.LimitPrice,
		Volume:      item.TargetVolume,
		Reason:      item.Reason,
		StrategyTag: models.PaperStrategyTagTradePlan,
		AutoFill:    true,
	}
}

// mirrorCalcOpenBuyVolume mirrors data.calcOpenBuyVolume (unexported) for mismatch detection.
func mirrorCalcOpenBuyVolume(amount, price float64) int64 {
	if amount <= 0 || price <= 0 {
		return 0
	}
	return int64(math.Floor(amount/price/100) * 100)
}

type frozenSpecSnap struct {
	StockCode    string
	Side         string
	LimitPrice   float64
	TargetVolume int64
	IntentStatus string
	RefPrice     float64
	EntryRule    string
}

func snapFrozenSpec(item models.TradePlanItem) frozenSpecSnap {
	side := item.Side
	if side == "" {
		side = "buy"
	}
	return frozenSpecSnap{
		StockCode:    item.StockCode,
		Side:         side,
		LimitPrice:   item.LimitPrice,
		TargetVolume: item.TargetVolume,
		IntentStatus: item.IntentStatus,
		RefPrice:     item.RefPrice,
		EntryRule:    item.EntryRule,
	}
}

func seedFrozenSpecPlan(t *testing.T) (*models.TradePlan, frozenSpecSnap) {
	t.Helper()
	setupCutoverTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    false,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:            "2026-08-03",
		GeneratedAt:          now,
		Status:               models.TradePlanStatusDraft,
		PlanVersion:          1,
		AmountPerStock:       100_000,
		Side:                 "buy",
		PricingPolicyVersion: 1,
		PricingStage:         "morning_materialized",
		ApprovedAt:           &now,
		ApprovedBy:           "contract",
		ApprovedSource:       "contract_test",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate:    "2026-08-03",
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		Priority:     1,
		TargetAmount: 100_000,
		LimitPrice:   10.0,
		TargetVolume: 10_000, // Spec volume (≠ live calc at price 12.5)
		IntentStatus: "priced",
		RefPrice:     9.8,
		EntryRule:    "limit",
		Status:       models.TradePlanItemPending,
	}}))

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	before := snapFrozenSpec(got.Items[0])

	frozen, err := strategy.FreezeTradePlan(got, "contract-6721", "spec contract freeze")
	require.NoError(t, err)
	require.True(t, frozen.IsFrozen())

	after, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Len(t, after.Items, 1)
	return after, before
}

func TestFrozenSpecContract_ItemImmutableAfterFreeze(t *testing.T) {
	plan, before := seedFrozenSpecPlan(t)
	got := snapFrozenSpec(plan.Items[0])

	require.Equal(t, before.StockCode, got.StockCode)
	require.Equal(t, before.Side, got.Side)
	require.Equal(t, before.LimitPrice, got.LimitPrice)
	require.Equal(t, before.TargetVolume, got.TargetVolume)
	require.Equal(t, before.IntentStatus, got.IntentStatus)
	require.Equal(t, before.RefPrice, got.RefPrice)
	require.Equal(t, before.EntryRule, got.EntryRule)

	require.Equal(t, "sz000001", got.StockCode)
	require.Equal(t, "buy", got.Side)
	require.Equal(t, 10.0, got.LimitPrice)
	require.Equal(t, int64(10_000), got.TargetVolume)
}

func TestFrozenSpecContract_SubmitIntentReadsFrozenSpec(t *testing.T) {
	plan, _ := seedFrozenSpecPlan(t)
	item := plan.Items[0]

	intent := buildSubmitIntentFromFrozenSpec(item)
	require.Equal(t, item.StockCode, intent.StockCode)
	require.Equal(t, "buy", intent.Side)
	require.Equal(t, item.LimitPrice, intent.Price, "SubmitIntent.Price must be Frozen limit_price")
	require.Equal(t, item.TargetVolume, intent.Volume, "SubmitIntent.Volume must be Frozen target_volume")

	port := &stubPort{order: &broker.TradeOrder{ID: "1", Status: data.PaperOrderStatusFilled, StockCode: item.StockCode}}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	})
	_, err := svc.ExecutePlanItem(context.Background(), item, ExecutePlanItemOpts{
		Price:    intent.Price,
		Volume:   intent.Volume,
		AutoFill: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, port.calls)
	require.Equal(t, item.LimitPrice, port.lastIntent.Price)
	require.Equal(t, item.TargetVolume, port.lastIntent.Volume)
	require.Equal(t, item.StockCode, port.lastIntent.StockCode)
}

func TestFrozenSpecContract_PaperAndRealPortSameSpecInput(t *testing.T) {
	plan, _ := seedFrozenSpecPlan(t)
	item := plan.Items[0]
	intent := buildSubmitIntentFromFrozenSpec(item)

	paperPort := &stubPort{order: &broker.TradeOrder{ID: "p1", Status: data.PaperOrderStatusPending, StockCode: item.StockCode}}
	realPort := &stubPort{order: &broker.TradeOrder{ID: "r1", Status: data.PaperOrderStatusPending, StockCode: item.StockCode}}

	opts := ExecutePlanItemOpts{Price: intent.Price, Volume: intent.Volume, AutoFill: false}
	snap := func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	}

	_, err := NewExecutionService(paperPort).withSnapshotLoader(snap).
		ExecutePlanItem(context.Background(), item, opts)
	require.NoError(t, err)

	_, err = NewExecutionService(realPort).withSnapshotLoader(snap).
		ExecutePlanItem(context.Background(), item, opts)
	require.NoError(t, err)

	require.Equal(t, paperPort.lastIntent.Price, realPort.lastIntent.Price)
	require.Equal(t, paperPort.lastIntent.Volume, realPort.lastIntent.Volume)
	require.Equal(t, paperPort.lastIntent.StockCode, realPort.lastIntent.StockCode)
	require.Equal(t, paperPort.lastIntent.Side, realPort.lastIntent.Side)
	require.Equal(t, item.LimitPrice, paperPort.lastIntent.Price)
	require.Equal(t, item.TargetVolume, paperPort.lastIntent.Volume)
}

func TestFrozenSpecContract_DetectsLiveVolumeMismatchRisk(t *testing.T) {
	plan, _ := seedFrozenSpecPlan(t)
	item := plan.Items[0]

	// Live quote that yields a different lot size than Frozen Spec volume.
	livePrice := 12.5
	liveVol := mirrorCalcOpenBuyVolume(item.TargetAmount, livePrice)
	require.NotEqual(t, int64(0), liveVol)
	require.NotEqual(t, item.TargetVolume, liveVol,
		"fixture must create Spec vs live mismatch (Spec=%d live=%d)", item.TargetVolume, liveVol)

	contractIntent := buildSubmitIntentFromFrozenSpec(item)
	require.Equal(t, item.TargetVolume, contractIntent.Volume)
	require.NotEqual(t, liveVol, contractIntent.Volume,
		"contract SubmitIntent must NOT adopt live-recalculated volume")

	// Phase6.5.7.2.2: production open-buy must not live-recalc or overwrite Spec.
	src, err := os.ReadFile(filepath.Join("..", "data", "paper_open_buy.go"))
	require.NoError(t, err)
	require.NotContains(t, string(src), "calcOpenBuyVolume(",
		"sentinel: open-buy must not call calcOpenBuyVolume")
	require.NotContains(t, string(src), "planItem.TargetVolume = vol",
		"sentinel: open-buy must not overwrite Spec target_volume")
	require.Contains(t, string(src), "planItem.LimitPrice",
		"open-buy must read Frozen limit_price")
	require.Contains(t, string(src), "planItem.TargetVolume",
		"open-buy must read Frozen target_volume")
}

func TestFrozenSpecContract_UpdateItemExecutionDoesNotWriteTargetVolume(t *testing.T) {
	// Phase6.5.7.2.2: UpdateItemExecution must not persist Order Spec fields.
	// Morning materialization (UpdateItemMorningTargetVolume) may still write target_volume.
	src, err := os.ReadFile(filepath.Join("..", "data", "trade_plan_repo.go"))
	require.NoError(t, err)
	s := string(src)
	const marker = "func (r *TradePlanRepo) UpdateItemExecution"
	start := strings.Index(s, marker)
	require.GreaterOrEqual(t, start, 0, "UpdateItemExecution not found")
	rest := s[start+len(marker):]
	next := strings.Index(rest, "\nfunc (")
	require.GreaterOrEqual(t, next, 0, "next method after UpdateItemExecution not found")
	body := rest[:next]
	require.NotContains(t, body, `"target_volume"`,
		"UpdateItemExecution must not write target_volume (Frozen Spec immutable)")
	require.NotContains(t, body, `"limit_price"`,
		"UpdateItemExecution must not write limit_price")
}

func TestFrozenSpecContract_GuardBlocksUnfrozenSubmitPath(t *testing.T) {
	setupCutoverTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	draft := &models.TradePlan{
		TradeDate: "2026-08-03", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		AmountPerStock: 100_000, Side: "buy",
		PricingPolicyVersion: 1,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(draft, []models.TradePlanItem{{
		TradeDate: "2026-08-03", StockCode: "sz000001", Side: "buy",
		LimitPrice: 10, TargetVolume: 1000, Status: models.TradePlanItemPending,
		TargetAmount: 100_000,
	}}))
	got, err := data.NewTradePlanRepo().GetByID(draft.ID)
	require.NoError(t, err)

	guard := models.RequireFrozenReadyTradePlan(got)
	require.False(t, guard.Allowed)
	require.Equal(t, models.ReasonPlanNotFrozen, guard.Reason)

	// Naked ready without freeze_at also blocked.
	got.Status = models.TradePlanStatusReady
	got.FreezeAt = nil
	guard2 := models.RequireFrozenReadyTradePlan(got)
	require.False(t, guard2.Allowed)

	plan, _ := seedFrozenSpecPlan(t)
	guardOK := models.RequireFrozenReadyTradePlan(plan)
	require.True(t, guardOK.Allowed)
}
