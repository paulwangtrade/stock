package papertrading_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/approvegate"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	sellE2ETradeDate = "2026-08-18"
	sellE2ECode      = "sz000001"
	sellE2EQty       = int64(100)
	sellE2ESeedVol   = int64(1000)
	sellE2ECash      = 1_000_000.0
	sellE2EAvgCost   = 10.0
	sellE2EFillPx    = 10.5
)

func setupSellExecutionE2EDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:sell_e2e_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: sellE2ECash})
	t.Cleanup(papertrading.ResetConfigCache)
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    true,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))
	t.Cleanup(data.ResetPaperOpenBuyConfigCache)
}

func seedPaperSimDefaultSellable(t *testing.T, code string, total, avail, locked int64) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: sellE2ECash, Cash: sellE2ECash, Equity: sellE2ECash,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	pos := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: code, StockName: "平安银行",
		TotalVolume: total, AvailableVolume: avail, LockedVolume: locked,
		AvgCost: sellE2EAvgCost, MarkPrice: sellE2EAvgCost, UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(&pos).Error)
	return acc
}

func requireSellablePS(t *testing.T, code string, wantAvail, wantLocked int64) positionstate.PositionStateView {
	t.Helper()
	view := findPS(t, code)
	require.Greater(t, view.AvailableQty, int64(0))
	require.Equal(t, wantAvail, view.AvailableQty)
	require.Equal(t, wantLocked, view.LockedQty)
	require.True(t, view.CanSell)
	return view
}

func findPS(t *testing.T, code string) positionstate.PositionStateView {
	t.Helper()
	bundle := positionstate.NewService(nil).Evaluate(positionstate.Query{
		TradeDate: sellE2ETradeDate, AsOf: time.Now(),
	})
	for _, p := range bundle.Positions {
		if p.Symbol == code {
			return p
		}
	}
	t.Fatalf("position-state missing symbol %s", code)
	return positionstate.PositionStateView{}
}

func findSimPosition(t *testing.T, accID uint, code string) papertrading.PaperSimPosition {
	t.Helper()
	positions, err := papertrading.GetPositions(accID)
	require.NoError(t, err)
	for _, p := range positions {
		if p.StockCode == code {
			return p
		}
	}
	t.Fatalf("paper_sim_positions missing %s", code)
	return papertrading.PaperSimPosition{}
}

func draftApproveFreezeSell(t *testing.T, qty int64) *models.TradePlan {
	t.Helper()
	plan, err := strategy.BuildDraftTSellTradePlan(strategy.TSellDraftRequest{
		TradeDate: sellE2ETradeDate,
		StockCode: sellE2ECode,
		Quantity:  qty,
		Actor:     "e2e",
	})
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, "sell", plan.Side)
	require.Equal(t, qty, plan.Items[0].TargetVolume)
	require.Nil(t, plan.ApprovedAt)
	require.Nil(t, plan.FreezeAt)

	rd := readiness.EvaluateExecutionIntentReadiness(plan, nil)
	require.True(t, rd.Ready, "blockers=%+v", rd.Blockers)

	approve, err := approvegate.ApproveTradePlanByID(plan.ID, "e2e", "unit_test", nil)
	require.NoError(t, err)
	require.True(t, approve.OK, "approve blockers=%+v", approve.Blockers)
	require.Equal(t, models.TradePlanStatusDraft, approve.Plan.Status)
	require.NotNil(t, approve.Plan.ApprovedAt)
	require.Nil(t, approve.Plan.FreezeAt)

	frozen, err := strategy.FreezeTradePlan(approve.Plan, "e2e", "e2e freeze")
	require.NoError(t, err)
	require.True(t, frozen.IsFrozen())
	require.Equal(t, models.TradePlanStatusReady, frozen.Status)
	require.Equal(t, "sell", frozen.Side)
	require.Equal(t, qty, frozen.Items[0].TargetVolume)
	require.Equal(t, models.TradePlanItemPending, frozen.Items[0].Status)
	return frozen
}

func runSellExecution(t *testing.T, planID uint) *papertrading.ExecutionResult {
	t.Helper()
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: sellE2ETradeDate,
		PlanID:    planID,
		Trigger:   papertrading.TriggerManual,
		Actor:     "e2e",
		Price: papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{sellE2ECode: {Open: sellE2EFillPx}},
		},
		SkipWeekdayCheck: true,
		Now:              time.Date(2026, 8, 18, 10, 0, 0, 0, time.Local),
	})
	require.NoError(t, err)
	return res
}

func getPositionStateAPI(t *testing.T, tradeDate, code string) positionstate.PositionStateView {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPositionStateRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/portfolio/position-state?trade_date="+tradeDate, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		OK             bool `json:"ok"`
		PositionStates struct {
			Positions []positionstate.PositionStateView `json:"positions"`
		} `json:"position_states"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.True(t, envelope.OK)
	for _, p := range envelope.PositionStates.Positions {
		if p.Symbol == code {
			return p
		}
	}
	t.Fatalf("position-state API missing %s", code)
	return positionstate.PositionStateView{}
}

func TestSellExecutionE2E_DraftApproveFreezeFill_PaperSim(t *testing.T) {
	setupSellExecutionE2EDB(t)
	acc := seedPaperSimDefaultSellable(t, sellE2ECode, sellE2ESeedVol, sellE2ESeedVol, 0)
	requireSellablePS(t, sellE2ECode, sellE2ESeedVol, 0)

	beforePos := findSimPosition(t, acc.ID, sellE2ECode)
	beforeTotal := beforePos.TotalVolume
	beforeAvail := beforePos.AvailableVolume
	beforeAvg := beforePos.AvgCost
	accBefore, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	beforeCash := accBefore.Cash
	require.Equal(t, sellE2ESeedVol, beforeTotal)
	require.Equal(t, sellE2ESeedVol, beforeAvail)
	require.Equal(t, sellE2ECash, beforeCash)
	require.LessOrEqual(t, sellE2EQty, beforeAvail)

	frozen := draftApproveFreezeSell(t, sellE2EQty)
	frozenQty := frozen.Items[0].TargetVolume

	res := runSellExecution(t, frozen.ID)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)
	require.Equal(t, 0, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(frozen.ID)
	require.NoError(t, err)
	require.Len(t, status.Fills, 1)
	require.Equal(t, "sell", status.Fills[0].Side)
	require.Equal(t, frozenQty, status.Fills[0].Volume)
	require.InDelta(t, sellE2EFillPx, status.Fills[0].Price, 1e-9)

	gotPlan, err := data.NewTradePlanRepo().GetByID(frozen.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDone, gotPlan.Status)
	require.Equal(t, models.TradePlanItemFilled, gotPlan.Items[0].Status)
	require.Equal(t, frozenQty, gotPlan.Items[0].TargetVolume, "execution must not rewrite Frozen Spec")

	afterPos := findSimPosition(t, acc.ID, sellE2ECode)
	require.Equal(t, beforeTotal-sellE2EQty, afterPos.TotalVolume)
	require.Equal(t, beforeAvail-sellE2EQty, afterPos.AvailableVolume)
	require.Equal(t, int64(0), afterPos.LockedVolume)
	require.InDelta(t, beforeAvg, afterPos.AvgCost, 1e-9)

	accAfter, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	require.InDelta(t, beforeCash+sellE2EFillPx*float64(sellE2EQty), accAfter.Cash, 1e-6)

	ps := getPositionStateAPI(t, sellE2ETradeDate, sellE2ECode)
	require.Equal(t, afterPos.AvailableVolume, ps.AvailableQty)
	require.Equal(t, afterPos.TotalVolume, ps.TotalQty)
	require.True(t, ps.CanSell)
	require.Equal(t, positionstate.S4ReducedAvailable, ps.State)
}

func TestSellExecutionE2E_CaseA_InsufficientAvailable_RejectsNoFill(t *testing.T) {
	setupSellExecutionE2EDB(t)
	acc := seedPaperSimDefaultSellable(t, sellE2ECode, sellE2ESeedVol, sellE2ESeedVol, 0)
	frozen := draftApproveFreezeSell(t, sellE2EQty)

	// After freeze, available drops below Frozen Spec — draft gate already passed.
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).
		Where("account_id = ? AND stock_code = ?", acc.ID, sellE2ECode).
		Updates(map[string]any{"available_volume": int64(50), "updated_at": time.Now()}).Error)

	beforePos := findSimPosition(t, acc.ID, sellE2ECode)
	accBefore, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	beforeCash := accBefore.Cash

	res := runSellExecution(t, frozen.ID)
	require.Equal(t, papertrading.RunStatusCompletedWithRejects, res.Status)
	require.Equal(t, 0, res.FilledCount)
	require.Equal(t, 1, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(frozen.ID)
	require.NoError(t, err)
	require.Empty(t, status.Fills)
	require.Len(t, status.Orders, 1)
	require.Equal(t, papertrading.OrderStatusRejected, status.Orders[0].Status)
	require.Equal(t, papertrading.RejectInsufficientAvailable, status.Orders[0].RejectReason)

	afterPos := findSimPosition(t, acc.ID, sellE2ECode)
	require.Equal(t, beforePos.TotalVolume, afterPos.TotalVolume)
	require.Equal(t, beforePos.AvailableVolume, afterPos.AvailableVolume)
	require.InDelta(t, beforePos.AvgCost, afterPos.AvgCost, 1e-9)

	accAfter, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	require.InDelta(t, beforeCash, accAfter.Cash, 1e-6)

	gotPlan, err := data.NewTradePlanRepo().GetByID(frozen.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusFailed, gotPlan.Status)
	require.Equal(t, sellE2EQty, gotPlan.Items[0].TargetVolume)
}

func TestSellExecutionE2E_CaseB_FrozenTargetVolumeNotRecomputed(t *testing.T) {
	setupSellExecutionE2EDB(t)
	seedPaperSimDefaultSellable(t, sellE2ECode, sellE2ESeedVol, sellE2ESeedVol, 0)
	frozen := draftApproveFreezeSell(t, sellE2EQty)
	frozenQty := frozen.Items[0].TargetVolume

	// After freeze, rewrite buy-style sizing inputs. Sell fill must still use Frozen TargetVolume.
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Where("id = ?", frozen.Items[0].ID).Updates(map[string]any{
		"target_amount": 1_000_000.0,
		"limit_price":   5.0,
		"updated_at":    time.Now(),
	}).Error)

	res := runSellExecution(t, frozen.ID)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)

	status, err := papertrading.GetPlanPaperStatus(frozen.ID)
	require.NoError(t, err)
	require.Len(t, status.Fills, 1)
	require.Equal(t, "sell", status.Fills[0].Side)
	require.Equal(t, frozenQty, status.Fills[0].Volume)

	gotPlan, err := data.NewTradePlanRepo().GetByID(frozen.ID)
	require.NoError(t, err)
	require.Equal(t, frozenQty, gotPlan.Items[0].TargetVolume)
}
