package tests

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	betaV04StockCode  = "sz301125"
	betaV04SignalDate = "2026-09-01"
	betaV04TradeDate  = "2026-09-02"
	betaV04SignalTag  = "突"
	betaV04PoolRank   = 3
	betaV04PoolScore  = 0.86
)

// betaV04Fixtures holds cross-layer golden keys for closed-loop assertions.
type betaV04Fixtures struct {
	PlanID    uint
	PoolID    uint
	SnapID    uint
	BuyFillID uint
	BuyPrice  float64
	BuyVolume int64
}

func setupBetaV04TestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:beta_v04_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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

func seedBetaV04SignalPool301125(t *testing.T, signalDate, poolDate string) *models.CandidatePool {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{
			SECUCODE: "301125.SZ", SECURITY_NAME_ABBR: "腾亚精工",
			Tag: betaV04SignalTag, SignalTime: signalDate + "T15:00:00+08:00", SignalPrice: 42.15,
		}},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{
		TradeDate: signalDate, Session: models.SignalScanSessionClose, Status: "done",
		ResultJSON: string(raw), HitTotal: 1,
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	pool := &models.CandidatePool{
		TradeDate: poolDate, GeneratedAt: time.Now(),
		Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{{
		StockCode: betaV04StockCode, StockName: "腾亚精工", Rank: betaV04PoolRank, Score: betaV04PoolScore,
		StrategyName: "trend_breakout", SignalTag: betaV04SignalTag, SignalSnapshotID: snap.ID,
	}}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	return pool
}

func seedBetaV04Account(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedBetaV04BuyPlan(t *testing.T, pool *models.CandidatePool, tradeDate string) (*models.TradePlan, *models.TradePlanItem) {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: time.Now(), PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusDone, Side: "buy",
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: betaV04StockCode, StockName: "腾亚精工", Side: "buy",
		TargetAmount: 100_000, Score: betaV04PoolScore, Status: models.TradePlanItemFilled,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return reloaded, &reloaded.Items[0]
}

func seedBetaV04BuyFill(t *testing.T, acc *papertrading.PaperSimAccount, plan *models.TradePlan, item *models.TradePlanItem, tradeDate string, price float64, vol int64, at time.Time) {
	t.Helper()
	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: betaV04StockCode, Side: "buy", Quantity: vol,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: at,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: betaV04StockCode, Side: "buy", Price: price, Volume: vol, FilledAt: at,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
}

func seedBetaV04SellFill(t *testing.T, acc *papertrading.PaperSimAccount, tradeDate string, price float64, vol int64, at time.Time) {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: time.Now(), Status: models.TradePlanStatusDone, Side: "sell",
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: betaV04StockCode, Side: "sell",
		TargetVolume: vol, Status: models.TradePlanItemFilled,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	item := plan.Items[0]
	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: betaV04StockCode, Side: "sell", Quantity: vol,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: at,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: betaV04StockCode, Side: "sell", Price: price, Volume: vol, FilledAt: at,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
}

func seedBetaV04OpenFixtures(t *testing.T, buyPrice float64, buyVolume int64) betaV04Fixtures {
	t.Helper()
	setupBetaV04TestDB(t)
	acc := seedBetaV04Account(t)
	pool := seedBetaV04SignalPool301125(t, betaV04SignalDate, betaV04TradeDate)
	plan, item := seedBetaV04BuyPlan(t, pool, betaV04TradeDate)
	buyAt := time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC)
	seedBetaV04BuyFill(t, acc, plan, item, betaV04TradeDate, buyPrice, buyVolume, buyAt)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: betaV04StockCode, StockName: "腾亚精工",
		TotalVolume: buyVolume, AvailableVolume: buyVolume, AvgCost: buyPrice, MarkPrice: buyPrice + 2,
	}).Error)

	var fill papertrading.PaperSimFill
	require.NoError(t, db.Dao.Where("stock_code = ? AND side = ?", betaV04StockCode, "buy").First(&fill).Error)

	var snap models.SignalScanSnapshot
	require.NoError(t, db.Dao.Order("id desc").First(&snap).Error)

	return betaV04Fixtures{
		PlanID:    plan.ID,
		PoolID:    pool.ID,
		SnapID:    snap.ID,
		BuyFillID: fill.ID,
		BuyPrice:  buyPrice,
		BuyVolume: buyVolume,
	}
}

func seedBetaV04ClosedFixtures(t *testing.T) (betaV04Fixtures, float64, float64) {
	t.Helper()
	const buyPrice = 40.0
	const sellPrice = 44.0
	const buyVolume int64 = 500
	fx := seedBetaV04OpenFixtures(t, buyPrice, buyVolume)
	sellAt := time.Date(2026, 9, 15, 9, 31, 0, 0, time.UTC)
	acc, _ := papertrading.GetDefaultAccount()
	require.NotNil(t, acc)
	seedBetaV04SellFill(t, acc, "2026-09-15", sellPrice, buyVolume, sellAt)
	return fx, buyPrice, sellPrice
}

func seedBetaV04NoTradeFixtures(t *testing.T) {
	t.Helper()
	setupBetaV04TestDB(t)
	seedBetaV04Account(t)
	seedBetaV04SignalPool301125(t, betaV04SignalDate, betaV04TradeDate)
}
