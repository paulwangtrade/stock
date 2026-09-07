package provenance_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/provenance"
	"go-stock/backend/tradeplanorigin"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProvenanceTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:prov_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, testDB.AutoMigrate(&models.SignalScanSnapshot{}))
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(func() {
		db.Dao = original
		papertrading.ResetConfigCache()
		_ = sqlDB.Close()
	})
}

func seedAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedSignalSnapshot(t *testing.T) uint {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE: "301125.SZ", Tag: "强", SignalTime: "2026-08-31",
				SignalPrice: 14.07, SignalPriceStatus: models.SignalPriceStatusFrozen,
			},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{TradeDate: "2026-08-31", Session: "close", Status: "done", ResultJSON: string(raw)}
	require.NoError(t, db.Dao.Create(snap).Error)
	return snap.ID
}

func seedFullProvenanceChain(t *testing.T, accID uint, snapID uint) (*models.TradePlan, *models.TradePlanItem, *papertrading.PaperSimFill) {
	t.Helper()
	now := time.Date(2026, 8, 29, 9, 31, 2, 0, time.Local)
	pool := &models.CandidatePool{
		TradeDate: "2026-08-28", GeneratedAt: now, Source: models.CandidatePoolSourceStrategyRun,
		Status: models.CandidatePoolStatusReady,
	}
	poolItems := []models.CandidatePoolItem{
		{
			StockCode: "sz301125", StockName: "腾亚精工", Rank: 3, Score: 0.82,
			StrategyName: "冰点超跌·出坑买点", SignalTag: "强", SignalSnapshotID: snapID,
			Reason: models.CandidatePoolSourceStrategyRun,
		},
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, poolItems))

	plan := &models.TradePlan{
		TradeDate: "2026-08-29", GeneratedAt: now, PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
	}
	planItems := []models.TradePlanItem{
		{
			TradeDate: "2026-08-29", StockCode: "sz301125", StockName: "腾亚精工", Side: "buy",
			Score: 0.82, StrategyName: "冰点超跌·出坑买点", Status: models.TradePlanItemPending,
			Reason: models.CandidatePoolSourceStrategyRun,
		},
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))
	item := &planItems[0]
	item.PlanID = plan.ID

	order := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: plan.TradeDate,
		StockCode: "sz301125", StockName: "腾亚精工", Side: "buy",
		Quantity: 9000, OrderPrice: 10.65, Status: papertrading.OrderStatusFilled,
		FilledPrice: 10.65, FilledVolume: 9000, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: "sz301125", StockName: "腾亚精工", Side: "buy",
		Price: 10.65, Volume: 9000, FillReason: papertrading.FillReasonMarketOpen, FilledAt: now,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: accID, StockCode: "sz301125", StockName: "腾亚精工",
		TotalVolume: 9000, AvailableVolume: 0, LockedVolume: 9000,
		AvgCost: 10.63, MarkPrice: 11.0,
	}).Error)
	return plan, item, fill
}

func TestEvaluate_FullProvenance(t *testing.T) {
	setupProvenanceTestDB(t)
	acc := seedAccount(t)
	snapID := seedSignalSnapshot(t)
	plan, item, fill := seedFullProvenanceChain(t, acc.ID, snapID)

	view, err := provenance.Evaluate("sz301125", provenance.EvaluateOptions{})
	require.NoError(t, err)
	require.Equal(t, "sz301125", view.StockCode)
	require.Equal(t, "腾亚精工", view.StockName)
	require.InDelta(t, 9000, view.Position.Quantity, 1e-6)
	require.InDelta(t, 10.63, view.Position.AvgCost, 1e-6)
	require.Len(t, view.Trades, 1)
	require.Equal(t, plan.ID, view.Trades[0].PlanID)
	require.Equal(t, item.ID, view.Trades[0].PlanItemID)
	require.Equal(t, fill.ID, view.Trades[0].FillID)
	require.InDelta(t, 10.65, view.Trades[0].FillPrice, 1e-6)
	require.Equal(t, int64(9000), view.Trades[0].FillVolume)
	require.NotNil(t, view.Trades[0].FilledAt)
	require.Len(t, view.Origins, 1)
	require.Equal(t, plan.ID, view.Origins[0].PlanID)
	require.Equal(t, "冰点超跌·出坑买点", view.Origins[0].Strategy)
	require.True(t, view.Origins[0].Signal.Present)
	require.Equal(t, "强", view.Origins[0].Signal.Tag)
	require.Equal(t, snapID, view.Origins[0].Signal.SnapshotID)
	require.True(t, view.Origins[0].Reason.Present)
	require.NotEqual(t, tradeplanorigin.Missing, view.Origins[0].Reason.Source)
	require.Equal(t, papertrading.ReconcileStatusMatched, view.Reconcile.Status)
}

func TestEvaluate_PositionWithoutFills(t *testing.T) {
	setupProvenanceTestDB(t)
	acc := seedAccount(t)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600519", StockName: "贵州茅台",
		TotalVolume: 100, AvgCost: 1800, MarkPrice: 1800,
	}).Error)

	view, err := provenance.Evaluate("sh600519", provenance.EvaluateOptions{})
	require.NoError(t, err)
	require.Empty(t, view.Trades)
	require.Empty(t, view.Origins)
	require.Equal(t, papertrading.ReconcileStatusUnattributed, view.Reconcile.Status)
	require.Equal(t, int64(100), view.Reconcile.PositionVolume)
	require.Equal(t, int64(0), view.Reconcile.AttributedVolume)
}

func TestEvaluate_MultipleFills(t *testing.T) {
	setupProvenanceTestDB(t)
	acc := seedAccount(t)
	now := time.Now()

	plan1 := &models.TradePlan{TradeDate: "2026-08-01", GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1}
	require.NoError(t, db.Dao.Create(plan1).Error)
	item1 := &models.TradePlanItem{
		PlanID: plan1.ID, TradeDate: "2026-08-01", StockCode: "sh600363", Side: "buy",
		Status: models.TradePlanItemPending, StrategyName: "plan-a",
	}
	require.NoError(t, db.Dao.Create(item1).Error)

	plan2 := &models.TradePlan{TradeDate: "2026-08-03", GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1}
	require.NoError(t, db.Dao.Create(plan2).Error)
	item2 := &models.TradePlanItem{
		PlanID: plan2.ID, TradeDate: "2026-08-03", StockCode: "sh600363", Side: "buy",
		Status: models.TradePlanItemPending, StrategyName: "plan-b",
	}
	require.NoError(t, db.Dao.Create(item2).Error)

	t1 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.Local)
	t2 := time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local)
	for _, spec := range []struct {
		plan *models.TradePlan
		item *models.TradePlanItem
		at   time.Time
		vol  int64
		px   float64
	}{
		{plan1, item1, t1, 3000, 10.0},
		{plan2, item2, t2, 2000, 12.0},
	} {
		order := &papertrading.PaperSimOrder{
			AccountID: acc.ID, PlanID: spec.plan.ID, PlanItemID: spec.item.ID, TradeDate: spec.plan.TradeDate,
			StockCode: "sh600363", Side: "buy", Quantity: spec.vol, Status: papertrading.OrderStatusFilled,
			FilledPrice: spec.px, FilledVolume: spec.vol, OrderTime: spec.at,
		}
		require.NoError(t, db.Dao.Create(order).Error)
		require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
			AccountID: acc.ID, OrderID: order.ID, PlanID: spec.plan.ID, PlanItemID: spec.item.ID,
			StockCode: "sh600363", Side: "buy", Price: spec.px, Volume: spec.vol, FilledAt: spec.at,
		}).Error)
	}

	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", TotalVolume: 5000, AvgCost: 10.8, MarkPrice: 11.5,
	}).Error)

	view, err := provenance.Evaluate("sh600363", provenance.EvaluateOptions{})
	require.NoError(t, err)
	require.Len(t, view.Trades, 2)
	require.Len(t, view.Origins, 2)
	require.Equal(t, plan2.ID, view.Trades[0].PlanID, "newer fill first")
	require.Equal(t, plan1.ID, view.Trades[1].PlanID)
	require.Equal(t, papertrading.ReconcileStatusMatched, view.Reconcile.Status)
}

func TestEvaluate_NotFound(t *testing.T) {
	setupProvenanceTestDB(t)
	seedAccount(t)
	_, err := provenance.Evaluate("sh999999", provenance.EvaluateOptions{})
	require.ErrorIs(t, err, provenance.ErrNotFound)
}

func TestEvaluate_NormalizesSecucode(t *testing.T) {
	setupProvenanceTestDB(t)
	acc := seedAccount(t)
	snapID := seedSignalSnapshot(t)
	seedFullProvenanceChain(t, acc.ID, snapID)

	view, err := provenance.Evaluate("301125.SZ", provenance.EvaluateOptions{})
	require.NoError(t, err)
	require.Equal(t, "sz301125", view.StockCode)
}
