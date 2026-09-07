package api_test

import (
	"encoding/json"
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

func TestPositionAttributionAPI_MultiPlanSameStock(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)

	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	now := time.Now()
	mkPlanItem := func(date, code string) (*models.TradePlan, *models.TradePlanItem) {
		p := &models.TradePlan{TradeDate: date, GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1}
		require.NoError(t, db.Dao.Create(p).Error)
		it := &models.TradePlanItem{
			PlanID: p.ID, TradeDate: date, StockCode: code, StockName: "联创光电",
			Side: "buy", Status: models.TradePlanItemPending,
		}
		require.NoError(t, db.Dao.Create(it).Error)
		return p, it
	}
	mkFill := func(plan *models.TradePlan, item *models.TradePlanItem, vol int64) {
		o := &papertrading.PaperSimOrder{
			AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: plan.TradeDate,
			StockCode: item.StockCode, Side: "buy", Quantity: vol, Status: papertrading.OrderStatusFilled,
			FilledVolume: vol, OrderTime: now,
		}
		require.NoError(t, db.Dao.Create(o).Error)
		require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
			AccountID: acc.ID, OrderID: o.ID, PlanID: plan.ID, PlanItemID: item.ID,
			StockCode: item.StockCode, Side: "buy", Price: 10, Volume: vol, FilledAt: now,
		}).Error)
	}

	p32, i32 := mkPlanItem("2026-08-01", "sh600363")
	p35, i35 := mkPlanItem("2026-08-03", "sh600363")
	mkFill(p32, i32, 3000)
	mkFill(p35, i35, 2000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 5000, MarkPrice: 11, AvgCost: 10.4,
	}).Error)

	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/positions/attribution", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	attr, ok := body["attribution"].(map[string]any)
	require.True(t, ok)
	positions, ok := attr["positions"].([]any)
	require.True(t, ok)
	require.Len(t, positions, 1)
	row := positions[0].(map[string]any)
	require.Equal(t, "sh600363", row["stock_code"])
	lots := row["lots"].([]any)
	require.Len(t, lots, 2)
	planIDs := map[float64]bool{}
	for _, raw := range lots {
		lot := raw.(map[string]any)
		planIDs[lot["plan_id"].(float64)] = true
		require.NotZero(t, lot["fill_id"])
		require.NotZero(t, lot["order_id"])
	}
	require.True(t, planIDs[float64(p32.ID)])
	require.True(t, planIDs[float64(p35.ID)])
	recMap := row["reconcile"].(map[string]any)
	require.Equal(t, "matched", recMap["status"])
}

func TestPositionAttributionAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/positions/attribution", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
