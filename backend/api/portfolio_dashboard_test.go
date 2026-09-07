package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func getPortfolioDashboard(t *testing.T, query string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPortfolioDashboardRoutes(mux)
	path := "/api/portfolio/dashboard"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	dash, ok := body["dashboard"].(map[string]any)
	require.True(t, ok)
	return dash
}

func TestPortfolioDashboardAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPortfolioDashboardRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/portfolio/dashboard", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPortfolioDashboardAPI_EmptyAccount(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	dash := getPortfolioDashboard(t, "trade_date=2026-08-17")
	require.Equal(t, false, dash["found"])
	summary := dash["summary"].(map[string]any)
	require.Equal(t, float64(0), summary["equity"])
	require.Equal(t, float64(0), summary["position_count"])
	require.Nil(t, summary["daily_pnl"])
	pos, _ := dash["positions"].([]any)
	require.Empty(t, pos)
	trades := dash["trades"].(map[string]any)
	fills, _ := trades["fills"].([]any)
	require.Empty(t, fills)

	var n int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestPortfolioDashboardAPI_WithPositions(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 700_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 12,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimDailyReport{
		AccountID: acc.ID, ReportDate: "2026-08-14", Equity: 710_000, Cash: 700_000, MarketValue: 10_000,
	}).Error)

	dash := getPortfolioDashboard(t, "trade_date=2026-08-17")
	require.Equal(t, true, dash["found"])
	summary := dash["summary"].(map[string]any)
	require.InDelta(t, 712_000.0, summary["equity"].(float64), 1e-6)
	require.InDelta(t, 700_000.0, summary["cash"].(float64), 1e-6)
	require.InDelta(t, 12_000.0, summary["market_value"].(float64), 1e-6)
	require.Equal(t, float64(1), summary["position_count"])
	require.InDelta(t, 2000.0, summary["daily_pnl"].(float64), 1e-6)

	positions := dash["positions"].([]any)
	require.Len(t, positions, 1)
	row := positions[0].(map[string]any)
	require.Equal(t, "sh600000", row["stock_code"])
	require.Equal(t, "浦发", row["stock_name"])
	require.Equal(t, float64(1000), row["quantity"])
	require.InDelta(t, 10.0, row["avg_cost"].(float64), 1e-6)
	require.InDelta(t, 12.0, row["market_price"].(float64), 1e-6)
	require.InDelta(t, 2000.0, row["unrealized_pnl"].(float64), 1e-6)

	risk := dash["risk"].(map[string]any)
	require.Contains(t, risk, "max_position_ratio")
	require.Contains(t, risk, "risk_level")

	// API must not mutate cash
	var cash float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Where("id = ?", acc.ID).Pluck("cash", &cash).Error)
	require.InDelta(t, 700_000, cash, 1e-9)
}

func TestPortfolioDashboardAPI_WithFills(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安",
		TotalVolume: 100, AvgCost: 10, MarkPrice: 10.5,
	}).Error)
	order := papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 3, PlanItemID: 1, TradeDate: "2026-08-17",
		StockCode: "sz000001", StockName: "平安", Side: "buy", Quantity: 100,
		Status: papertrading.OrderStatusFilled, FilledPrice: 10, FilledVolume: 100,
		OrderTime: time.Date(2026, 8, 17, 9, 31, 0, 0, time.Local),
	}
	require.NoError(t, db.Dao.Create(&order).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: 3, PlanItemID: 1,
		StockCode: "sz000001", StockName: "平安", Side: "buy", Price: 10, Volume: 100,
		FillReason: papertrading.FillReasonMarketOpen,
		FilledAt:   time.Date(2026, 8, 17, 9, 31, 5, 0, time.Local),
	}).Error)

	dash := getPortfolioDashboard(t, "trade_date=2026-08-17")
	require.Equal(t, true, dash["found"])
	trades := dash["trades"].(map[string]any)
	require.Equal(t, "2026-08-17", trades["trade_date"])
	fills := trades["fills"].([]any)
	require.Len(t, fills, 1)
	f := fills[0].(map[string]any)
	require.Equal(t, "sz000001", f["stock_code"])
	require.Equal(t, float64(100), f["volume"])
	require.InDelta(t, 10.0, f["price"].(float64), 1e-6)
}

func TestPortfolioDashboardAssetMiddleware_Routes(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	calledNext := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.PortfolioDashboardAssetMiddleware(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/dashboard", nil))
	require.False(t, calledNext)
	require.Equal(t, http.StatusOK, rec.Code)

	calledNext = false
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/other", nil))
	require.True(t, calledNext)
	require.Equal(t, http.StatusTeapot, rec.Code)
}
