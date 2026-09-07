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
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func provenanceMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPortfolioDashboardRoutes(mux)
	return mux
}

func seedProvenanceAPIFixtures(t *testing.T) (planID uint, snapID uint) {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)

	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "301125.SZ", Tag: "强", SignalTime: "2026-08-31", SignalPrice: 14.07, SignalPriceStatus: models.SignalPriceStatusFrozen},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{TradeDate: "2026-08-31", Session: "close", Status: "done", ResultJSON: string(raw)}
	require.NoError(t, db.Dao.Create(snap).Error)
	snapID = snap.ID

	now := time.Date(2026, 8, 29, 9, 31, 2, 0, time.Local)
	pool := &models.CandidatePool{TradeDate: "2026-08-28", GeneratedAt: now, Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady}
	poolItems := []models.CandidatePoolItem{
		{StockCode: "sz301125", Rank: 3, Score: 0.82, StrategyName: "冰点超跌·出坑买点", SignalTag: "强", SignalSnapshotID: snapID, Reason: models.CandidatePoolSourceStrategyRun},
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, poolItems))

	plan := &models.TradePlan{TradeDate: "2026-08-29", GeneratedAt: now, PoolID: pool.ID, MaxNames: 5, Status: models.TradePlanStatusReady, PlanVersion: 1}
	planItems := []models.TradePlanItem{
		{TradeDate: "2026-08-29", StockCode: "sz301125", StockName: "腾亚精工", Side: "buy", Score: 0.82, StrategyName: "冰点超跌·出坑买点", Status: models.TradePlanItemPending, Reason: models.CandidatePoolSourceStrategyRun},
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))
	planID = plan.ID
	item := planItems[0]
	item.PlanID = plan.ID

	order := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: plan.TradeDate,
		StockCode: "sz301125", StockName: "腾亚精工", Side: "buy", Quantity: 9000,
		Status: papertrading.OrderStatusFilled, FilledPrice: 10.65, FilledVolume: 9000, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: "sz301125", Side: "buy", Price: 10.65, Volume: 9000, FilledAt: now,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz301125", StockName: "腾亚精工",
		TotalVolume: 9000, AvgCost: 10.63, MarkPrice: 11.0,
	}).Error)
	return planID, snapID
}

func TestPortfolioProvenanceHandler_OK(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}))
	planID, snapID := seedProvenanceAPIFixtures(t)

	mux := provenanceMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/positions/sz301125/provenance", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	prov := body["provenance"].(map[string]any)
	require.Equal(t, "sz301125", prov["stock_code"])
	pos := prov["position"].(map[string]any)
	require.InDelta(t, 9000, pos["quantity"].(float64), 1e-6)
	require.InDelta(t, 10.63, pos["avg_cost"].(float64), 1e-6)

	trades := prov["trades"].([]any)
	require.Len(t, trades, 1)
	tr := trades[0].(map[string]any)
	require.Equal(t, float64(planID), tr["plan_id"].(float64))
	require.InDelta(t, 10.65, tr["fill_price"].(float64), 1e-6)
	require.NotEmpty(t, tr["filled_at"])

	origins := prov["origins"].([]any)
	require.Len(t, origins, 1)
	origin := origins[0].(map[string]any)
	require.Equal(t, "冰点超跌·出坑买点", origin["strategy"])
	signal := origin["signal"].(map[string]any)
	require.Equal(t, "强", signal["tag"])
	require.Equal(t, float64(snapID), signal["snapshot_id"].(float64))

	reconcile := prov["reconcile"].(map[string]any)
	require.Equal(t, "matched", reconcile["status"])
}

func TestPortfolioProvenanceHandler_NoProvenanceHolding(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600519", StockName: "贵州茅台",
		TotalVolume: 100, AvgCost: 1800, MarkPrice: 1800,
	}).Error)

	mux := provenanceMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/positions/sh600519/provenance", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	prov := body["provenance"].(map[string]any)
	require.Empty(t, prov["trades"].([]any))
	require.Empty(t, prov["origins"].([]any))
	reconcile := prov["reconcile"].(map[string]any)
	require.Equal(t, "unattributed", reconcile["status"])
}

func TestPortfolioProvenanceHandler_MultipleFills(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	now := time.Now()
	for i, td := range []string{"2026-08-01", "2026-08-03"} {
		plan := &models.TradePlan{TradeDate: td, GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1}
		require.NoError(t, db.Dao.Create(plan).Error)
		item := &models.TradePlanItem{PlanID: plan.ID, TradeDate: td, StockCode: "sh600363", Side: "buy", Status: models.TradePlanItemPending}
		require.NoError(t, db.Dao.Create(item).Error)
		at := time.Date(2026, 8, 1+i*2, 10, 0, 0, 0, time.Local)
		vol := int64(3000 - i*1000)
		order := &papertrading.PaperSimOrder{
			AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: td,
			StockCode: "sh600363", Side: "buy", Quantity: vol, Status: papertrading.OrderStatusFilled,
			FilledPrice: 10 + float64(i), FilledVolume: vol, OrderTime: at,
		}
		require.NoError(t, db.Dao.Create(order).Error)
		require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
			AccountID: acc.ID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
			StockCode: "sh600363", Side: "buy", Price: 10 + float64(i), Volume: vol, FilledAt: at,
		}).Error)
	}
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", TotalVolume: 5000, AvgCost: 10.8, MarkPrice: 11.5,
	}).Error)

	mux := provenanceMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/positions/sh600363/provenance", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	trades := body["provenance"].(map[string]any)["trades"].([]any)
	require.Len(t, trades, 2)
}

func TestPortfolioProvenanceHandler_NotFound(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	mux := provenanceMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/positions/sh999999/provenance", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.False(t, body["ok"].(bool))
}

func TestPortfolioProvenanceHandler_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPortfolioProvenanceRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/portfolio/positions/sz000001/provenance", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPortfolioDashboardAssetMiddleware_ProvenanceRoute(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", TotalVolume: 100, AvgCost: 10, MarkPrice: 10,
	}).Error)

	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.PortfolioDashboardAssetMiddleware(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/portfolio/positions/sz000001/provenance"), nil))
	require.False(t, nextHit)
	require.Equal(t, http.StatusOK, rec.Code)
}
