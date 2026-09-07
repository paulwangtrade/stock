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
	"go-stock/backend/opportunity/projection"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProjectionsAPITestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:proj_api_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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

func seedProjectionSignal301125(t *testing.T, tradeDate string) uint {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE: "301125.SZ", SECURITY_NAME_ABBR: "腾亚精工",
				Tag: "突", StatusText: "突破+放量",
				SignalTime: tradeDate + "T15:00:00+08:00", SignalPrice: 42.15,
				SignalPriceStatus: models.SignalPriceStatusFrozen,
			},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{
		TradeDate: tradeDate, Session: models.SignalScanSessionClose, Status: "done",
		StrategyID: "trend_breakout", ResultJSON: string(raw), HitTotal: 1,
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	return snap.ID
}

func seedProjectionSignal600519(t *testing.T, tradeDate string) {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE: "600519.SH", SECURITY_NAME_ABBR: "贵州茅台",
				Tag: "强", StatusText: "趋势增强",
				SignalTime: tradeDate + "T15:00:00+08:00", SignalPrice: 1800,
				SignalPriceStatus: models.SignalPriceStatusFrozen, RSI: 28,
			},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{
		TradeDate: tradeDate, Session: models.SignalScanSessionClose, Status: "done",
		ResultJSON: string(raw), HitTotal: 1,
	}
	require.NoError(t, db.Dao.Create(snap).Error)
}

func seedProjectionPool301125(t *testing.T, tradeDate string, snapID uint) {
	t.Helper()
	pool := &models.CandidatePool{
		TradeDate: tradeDate, GeneratedAt: time.Now(),
		Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{{
		StockCode: "sz301125", StockName: "腾亚精工", Rank: 3, Score: 0.86,
		StrategyName: "trend_breakout", StrategyVersion: "run:1",
		SignalTag: "突", SignalSnapshotID: snapID,
	}}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
}

func getProjectionsJSON(t *testing.T, mux *http.ServeMux, query string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	path := "/api/opportunities/projections"
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

// Case1: API returns Signal + Pool, WATCH (no plan).
func TestOpportunitiesProjectionsAPI_Case1_SignalAndPool(t *testing.T) {
	setupProjectionsAPITestDB(t)
	snapID := seedProjectionSignal301125(t, "2026-09-01")
	seedProjectionPool301125(t, "2026-09-02", snapID)

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	body := getProjectionsJSON(t, mux, "trade_date=2026-09-02&stock_code=sz301125")
	require.EqualValues(t, 1, body["total"])
	items := body["items"].([]any)
	require.Len(t, items, 1)
	row := items[0].(map[string]any)
	signal := row["signal"].(map[string]any)
	require.True(t, signal["present"].(bool))
	require.Equal(t, "突", signal["signal_tag"])
	opp := row["opportunity"].(map[string]any)
	require.True(t, opp["present"].(bool))
	require.InDelta(t, 0.86, opp["score"].(float64), 1e-6)
	decision := row["decision"].(map[string]any)
	require.Equal(t, projection.DecisionStatusWatch, decision["decision_status"])
}

// Case2: full loop Signal → Pool → TradePlan → Position.
func TestOpportunitiesProjectionsAPI_Case2_FullLoop(t *testing.T) {
	setupProjectionsAPITestDB(t)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	snapID := seedProjectionSignal301125(t, "2026-09-01")
	seedProjectionPool301125(t, "2026-09-02", snapID)

	plan := &models.TradePlan{
		TradeDate: "2026-09-02", GeneratedAt: time.Now(), MaxNames: 5,
		Status: models.TradePlanStatusDraft, Side: "buy",
		DecisionProvider: models.TradePlanDecisionProviderFixed,
	}
	planItems := []models.TradePlanItem{{
		TradeDate: "2026-09-02", StockCode: "sz301125", Side: "buy",
		TargetAmount: 100_000, Score: 0.86, Status: models.TradePlanItemPending,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz301125", StockName: "腾亚精工",
		TotalVolume: 500, AvailableVolume: 500, AvgCost: 41, MarkPrice: 43,
	}).Error)

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	body := getProjectionsJSON(t, mux, "trade_date=2026-09-02&stock_code=sz301125")
	row := body["items"].([]any)[0].(map[string]any)
	decision := row["decision"].(map[string]any)
	require.Equal(t, projection.DecisionStatusBuyCandidate, decision["decision_status"])
	portfolio := row["portfolio"].(map[string]any)
	require.Equal(t, projection.HoldingStatusHeld, portfolio["holding_status"])
}

// Case3: Research only — no Pool, not BUY_CANDIDATE.
func TestOpportunitiesProjectionsAPI_Case3_ResearchOnly(t *testing.T) {
	setupProjectionsAPITestDB(t)
	tradeDate := "2026-09-01"
	seedProjectionSignal600519(t, tradeDate)

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	body := getProjectionsJSON(t, mux, "trade_date="+tradeDate+"&stock_code=sh600519")
	row := body["items"].([]any)[0].(map[string]any)
	opp := row["opportunity"].(map[string]any)
	require.False(t, opp["present"].(bool))
	decision := row["decision"].(map[string]any)
	require.NotEqual(t, projection.DecisionStatusBuyCandidate, decision["decision_status"])
	require.NotNil(t, row["research"])
	require.True(t, row["signal"].(map[string]any)["present"].(bool))
}

// Case4: unknown stock → 404.
func TestOpportunitiesProjectionsAPI_Case4_NotFound(t *testing.T) {
	setupProjectionsAPITestDB(t)
	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/opportunities/projections?stock_code=sh999999&trade_date=2026-09-02", nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOpportunitiesProjectionsAPI_StatusFilter(t *testing.T) {
	setupProjectionsAPITestDB(t)
	snapID := seedProjectionSignal301125(t, "2026-09-01")
	seedProjectionPool301125(t, "2026-09-02", snapID)

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	body := getProjectionsJSON(t, mux, "trade_date=2026-09-02&status=WATCH")
	require.EqualValues(t, 1, body["total"])
	body2 := getProjectionsJSON(t, mux, "trade_date=2026-09-02&status=BUY_CANDIDATE")
	require.EqualValues(t, 0, body2["total"])
}
