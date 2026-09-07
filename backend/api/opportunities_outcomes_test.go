package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOutcomesAPITestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:outcome_api_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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

func getOutcomesJSON(t *testing.T, mux *http.ServeMux, query string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	path := "/api/opportunities/outcomes"
	if query != "" {
		path += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	return body
}

func seedOutcomeAPISignalPool301125(t *testing.T, signalDate, poolDate string) *models.CandidatePool {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{
			SECUCODE: "301125.SZ", SECURITY_NAME_ABBR: "腾亚精工",
			Tag: "突", SignalTime: signalDate + "T15:00:00+08:00", SignalPrice: 42.15,
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
		StockCode: "sz301125", StockName: "腾亚精工", Rank: 3, Score: 0.86,
		StrategyName: "trend_breakout", SignalTag: "突", SignalSnapshotID: snap.ID,
	}}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	return pool
}

func seedOutcomeAPIAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedOutcomeAPIBuyPlan(t *testing.T, pool *models.CandidatePool, tradeDate string) (*models.TradePlan, *models.TradePlanItem) {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: time.Now(), PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusDone, Side: "buy",
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: "sz301125", StockName: "腾亚精工", Side: "buy",
		TargetAmount: 100_000, Score: 0.86, Status: models.TradePlanItemFilled,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return reloaded, &reloaded.Items[0]
}

func seedOutcomeAPIBuyFill(t *testing.T, acc *papertrading.PaperSimAccount, plan *models.TradePlan, item *models.TradePlanItem, tradeDate string, price float64, vol int64, at time.Time) {
	t.Helper()
	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: "sz301125", Side: "buy", Quantity: vol,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: at,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: "sz301125", Side: "buy", Price: price, Volume: vol, FilledAt: at,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
}

func seedOutcomeAPISellFill(t *testing.T, acc *papertrading.PaperSimAccount, tradeDate string, price float64, vol int64, at time.Time) {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: time.Now(), Status: models.TradePlanStatusDone, Side: "sell",
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: "sz301125", Side: "sell",
		TargetVolume: vol, Status: models.TradePlanItemFilled,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	item := plan.Items[0]
	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: "sz301125", Side: "sell", Quantity: vol,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: at,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: "sz301125", Side: "sell", Price: price, Volume: vol, FilledAt: at,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
}

// Case1: buy only → OPEN
func TestOpportunitiesOutcomesAPI_Case1_Open(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 41, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125&status=OPEN")
	items := body["items"].([]any)
	require.Len(t, items, 1)
	row := items[0].(map[string]any)
	require.Equal(t, outcome.OutcomeStatusOpen, row["outcome_status"])
}

// Case2: buy + sell → CLOSED
func TestOpportunitiesOutcomesAPI_Case2_Closed(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 40, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))
	seedOutcomeAPISellFill(t, acc, "2026-09-15", 44, 500, time.Date(2026, 9, 15, 9, 31, 0, 0, time.UTC))

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125&status=CLOSED")
	items := body["items"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, outcome.OutcomeStatusClosed, items[0].(map[string]any)["outcome_status"])
}

// Case3: multi-buy FIFO
func TestOpportunitiesOutcomesAPI_Case3_FIFO(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-01", 40, 300, time.Date(2026, 9, 1, 9, 31, 0, 0, time.UTC))
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-05", 42, 200, time.Date(2026, 9, 5, 9, 31, 0, 0, time.UTC))
	seedOutcomeAPISellFill(t, acc, "2026-09-10", 45, 400, time.Date(2026, 9, 10, 9, 31, 0, 0, time.UTC))

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125")
	items := body["items"].([]any)
	require.Len(t, items, 3)
}

// Case4: NO_TRADE
func TestOpportunitiesOutcomesAPI_Case4_NoTrade(t *testing.T) {
	setupOutcomesAPITestDB(t)
	seedOutcomeAPIAccount(t)
	seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125&trade_date=2026-09-02&status=NO_TRADE")
	items := body["items"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, outcome.OutcomeStatusNoTrade, items[0].(map[string]any)["outcome_status"])
}

func TestOpportunitiesOutcomesAPI_StatusFilterEmpty(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 41, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125&status=CLOSED")
	require.Empty(t, body["items"].([]any))
}

func TestOpportunitiesOutcomesAPI_SecucodeStockCode(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 41, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	for _, query := range []string{
		"stock_code=301125.SZ&status=OPEN",
		"stock_code=301125&status=OPEN",
	} {
		body := getOutcomesJSON(t, mux, query)
		items := body["items"].([]any)
		require.Len(t, items, 1, query)
		require.Equal(t, "sz301125", items[0].(map[string]any)["stock_code"], query)
	}
}

func TestOpportunitiesOutcomesAPI_UnknownStockNotFound(t *testing.T) {
	setupOutcomesAPITestDB(t)
	seedOutcomeAPIAccount(t)

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities/outcomes?stock_code=sh999999", nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.False(t, body["ok"].(bool))
	require.EqualValues(t, 404, body["code"])
	require.Equal(t, "outcome not found", body["message"])
	require.NotNil(t, body["items"])
	require.Empty(t, body["items"].([]any))
}
