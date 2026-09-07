package outcome_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/opportunity/projection"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOutcomeTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:outcome_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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

func seedDefaultAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedSignalPool301125(t *testing.T, signalDate, poolDate string) (*models.CandidatePool, uint) {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE: "301125.SZ", SECURITY_NAME_ABBR: "腾亚精工",
				Tag: "突", StatusText: "突破+放量",
				SignalTime: signalDate + "T15:00:00+08:00", SignalPrice: 42.15,
				SignalPriceStatus: models.SignalPriceStatusFrozen,
			},
		},
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
		StockCode: "sz301125", StockName: "腾亚精工", Rank: 3, Score: 0.86,
		StrategyName: "trend_breakout", SignalTag: "突", SignalSnapshotID: snap.ID,
	}}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	return pool, snap.ID
}

func seedBuyPlan(t *testing.T, pool *models.CandidatePool, tradeDate string) (*models.TradePlan, *models.TradePlanItem) {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: time.Now(), PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusDone, Side: "buy",
		DecisionProvider: models.TradePlanDecisionProviderFixed,
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: "sz301125", StockName: "腾亚精工", Side: "buy",
		TargetAmount: 100_000, Score: 0.86, Status: models.TradePlanItemFilled,
		StrategyName: "trend_breakout",
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return reloaded, &reloaded.Items[0]
}

func seedBuyFill(t *testing.T, acc *papertrading.PaperSimAccount, plan *models.TradePlan, item *models.TradePlanItem, tradeDate string, price float64, vol int64, at time.Time) *papertrading.PaperSimFill {
	t.Helper()
	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: "sz301125", StockName: "腾亚精工", Side: "buy", Quantity: vol,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol,
		OrderTime: at,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: "sz301125", StockName: "腾亚精工", Side: "buy",
		Price: price, Volume: vol, FilledAt: at,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	require.NoError(t, db.Dao.Model(item).Updates(map[string]any{
		"fill_id": fill.ID, "order_id": order.ID, "filled_price": price, "filled_volume": vol,
		"status": models.TradePlanItemFilled,
	}).Error)
	return fill
}

func seedSellFill(t *testing.T, acc *papertrading.PaperSimAccount, tradeDate string, price float64, vol int64, at time.Time, reason string) *papertrading.PaperSimFill {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: time.Now(), MaxNames: 1,
		Status: models.TradePlanStatusDone, Side: "sell",
		SourceSession: models.TradePlanSourceExitReview,
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: "sz301125", StockName: "腾亚精工", Side: "sell",
		TargetVolume: vol, Status: models.TradePlanItemFilled, Reason: reason,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	item := plan.Items[0]
	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: "sz301125", Side: "sell", Quantity: vol,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol,
		OrderTime: at,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: "sz301125", Side: "sell", Price: price, Volume: vol, FilledAt: at,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	return fill
}

// Case1: buy only → OPEN
func TestOutcomeProjection_Case1_BuyOnly_Open(t *testing.T) {
	setupOutcomeTestDB(t)
	acc := seedDefaultAccount(t)
	pool, _ := seedSignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedBuyPlan(t, pool, "2026-09-02")
	seedBuyFill(t, acc, plan, item, "2026-09-02", 41.0, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	rows, err := outcome.ProjectOne("sz301125", outcome.ProjectOptions{TradeDate: "2026-09-02"})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	require.Equal(t, outcome.OutcomeStatusOpen, row.OutcomeStatus)
	require.True(t, row.Entry.Present)
	require.InDelta(t, 41.0, row.Entry.EntryPrice, 1e-6)
	require.Equal(t, int64(500), row.Entry.EntryQty)
	require.False(t, row.Exit.Present)
	require.Nil(t, row.Performance.RealizedReturnPct)
	require.Equal(t, int64(500), row.Performance.Quantity)
	require.True(t, row.Signal.Present)
	require.True(t, row.Opportunity.Present)
	require.Equal(t, projection.DecisionStatusBuyCandidate, row.Decision.DecisionStatus)
}

// Case2: buy + sell → CLOSED with realized return
func TestOutcomeProjection_Case2_BuySell_Closed(t *testing.T) {
	setupOutcomeTestDB(t)
	acc := seedDefaultAccount(t)
	pool, _ := seedSignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedBuyPlan(t, pool, "2026-09-02")
	seedBuyFill(t, acc, plan, item, "2026-09-02", 40.0, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))
	seedSellFill(t, acc, "2026-09-15", 44.0, 500, time.Date(2026, 9, 15, 9, 31, 0, 0, time.UTC), "exit_review:TIME_REVIEW")

	rows, err := outcome.ProjectOne("sz301125", outcome.ProjectOptions{})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	require.Equal(t, outcome.OutcomeStatusClosed, row.OutcomeStatus)
	require.True(t, row.Exit.Present)
	require.InDelta(t, 44.0, row.Exit.ExitPrice, 1e-6)
	require.NotNil(t, row.Performance.RealizedReturnPct)
	require.InDelta(t, 10.0, *row.Performance.RealizedReturnPct, 0.01) // (44-40)/40*100
	require.Equal(t, 13, row.Performance.HoldingDays)                   // Sep 2 → Sep 15
	require.Equal(t, models.TradePlanSourceExitReview, row.Exit.ExitChannel)
}

// Case3: multi-buy FIFO + partial sell
func TestOutcomeProjection_Case3_MultiBuyFIFO(t *testing.T) {
	setupOutcomeTestDB(t)
	acc := seedDefaultAccount(t)
	pool, _ := seedSignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedBuyPlan(t, pool, "2026-09-02")

	seedBuyFill(t, acc, plan, item, "2026-09-01", 40.0, 300, time.Date(2026, 9, 1, 9, 31, 0, 0, time.UTC))
	seedBuyFill(t, acc, plan, item, "2026-09-05", 42.0, 200, time.Date(2026, 9, 5, 9, 31, 0, 0, time.UTC))
	seedSellFill(t, acc, "2026-09-10", 45.0, 400, time.Date(2026, 9, 10, 9, 31, 0, 0, time.UTC), "t_sell")

	rows, err := outcome.ProjectOne("sz301125", outcome.ProjectOptions{})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	var closed, open int
	for _, row := range rows {
		switch row.OutcomeStatus {
		case outcome.OutcomeStatusClosed:
			closed++
			require.NotNil(t, row.Performance.RealizedReturnPct)
		case outcome.OutcomeStatusOpen:
			open++
			require.Nil(t, row.Performance.RealizedReturnPct)
			require.Equal(t, int64(100), row.Performance.Quantity)
			require.InDelta(t, 42.0, row.Performance.EntryPrice, 1e-6)
		}
	}
	require.Equal(t, 2, closed)
	require.Equal(t, 1, open)
}

// Case4: signal + pool, no fills → NO_TRADE
func TestOutcomeProjection_Case4_NoTrade(t *testing.T) {
	setupOutcomeTestDB(t)
	seedDefaultAccount(t)
	seedSignalPool301125(t, "2026-09-01", "2026-09-02")

	rows, err := outcome.ProjectOne("sz301125", outcome.ProjectOptions{
		TradeDate:      "2026-09-02",
		IncludeNoTrade: true,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	require.Equal(t, outcome.OutcomeStatusNoTrade, row.OutcomeStatus)
	require.False(t, row.Entry.Present)
	require.False(t, row.Exit.Present)
	require.True(t, row.Signal.Present)
	require.True(t, row.Opportunity.Present)
	require.Equal(t, projection.DecisionStatusWatch, row.Decision.DecisionStatus)
}
