package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/home"

	"github.com/stretchr/testify/require"
)

func getInvestmentHomeJSON(t *testing.T, query string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterInvestmentHomeRoutes(mux)
	api.RegisterPortfolioDashboardRoutes(mux)
	path := "/api/investment/home"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	h, ok := body["home"].(map[string]any)
	require.True(t, ok)
	return h
}

func getSnapshotJSON(t *testing.T, mux http.Handler, query string) map[string]any {
	t.Helper()
	path := "/api/portfolio/snapshot"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body["snapshot"].(map[string]any)
}

func seedSimPositionHome(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 700_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		// total ≠ available+locked：证明 Home 读 Snapshot.total_qty，而不是前端加总。
		TotalVolume: 1500, AvailableVolume: 1000, LockedVolume: 400, AvgCost: 10, MarkPrice: 12,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimDailyReport{
		AccountID: acc.ID, ReportDate: "2026-08-18", Equity: 700_000, Cash: 700_000,
	}).Error)
	return acc
}

func TestInvestmentHome_P5D_CaseA_QtyMatchesSnapshot(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedSimPositionHome(t)

	mux := http.NewServeMux()
	api.RegisterInvestmentHomeRoutes(mux)
	api.RegisterPortfolioDashboardRoutes(mux)

	homeBody := getInvestmentHomeJSON(t, "trade_date=2026-08-19")
	snap := getSnapshotJSON(t, mux, "trade_date=2026-08-19")

	ps := homeBody["portfolio_summary"].(map[string]any)
	require.Equal(t, true, ps["found"])
	require.Equal(t, snap["found"], ps["found"])
	require.InDelta(t, snap["cash"].(float64), ps["cash"].(float64), 1e-6)
	require.InDelta(t, snap["equity"].(float64), ps["equity"].(float64), 1e-6)
	require.InDelta(t, snap["market_value"].(float64), ps["market_value"].(float64), 1e-6)
	require.Equal(t, snap["position_count"], ps["position_count"])

	homePos := homeBody["position_states"].(map[string]any)["positions"].([]any)
	snapPos := snap["positions"].([]any)
	require.Len(t, homePos, 1)
	require.Len(t, snapPos, 1)
	hp := homePos[0].(map[string]any)
	sp := snapPos[0].(map[string]any)
	require.Equal(t, sp["total_qty"], hp["total_qty"])
	require.Equal(t, sp["available_qty"], hp["available_qty"])
	require.Equal(t, sp["locked_qty"], hp["locked_qty"])
	require.Equal(t, float64(1500), hp["total_qty"])
	require.NotEqual(t, float64(1000)+float64(400), hp["total_qty"])
}

func TestInvestmentHome_P5D_CaseB_AvailableLocked(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedSimPositionHome(t)

	h := getInvestmentHomeJSON(t, "trade_date=2026-08-19")
	row := h["position_states"].(map[string]any)["positions"].([]any)[0].(map[string]any)
	require.Equal(t, float64(1500), row["total_qty"])
	require.Equal(t, float64(1000), row["available_qty"])
	require.Equal(t, float64(400), row["locked_qty"])
}

func TestInvestmentHome_P5D_CaseC_CashEquityAndDailyPnLNotUnrealized(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedSimPositionHome(t)

	mux := http.NewServeMux()
	api.RegisterInvestmentHomeRoutes(mux)
	api.RegisterPortfolioDashboardRoutes(mux)

	homeBody := getInvestmentHomeJSON(t, "trade_date=2026-08-19")
	snap := getSnapshotJSON(t, mux, "trade_date=2026-08-19")
	ps := homeBody["portfolio_summary"].(map[string]any)
	require.InDelta(t, 700_000.0, ps["cash"].(float64), 1e-6)
	require.InDelta(t, 718_000.0, ps["equity"].(float64), 1e-6) // 700000 + 12*1500
	require.InDelta(t, snap["equity"].(float64), ps["equity"].(float64), 1e-6)

	// unrealized = (12-10)*1500 = 3000; daily_pnl = 718000 - 700000 = 18000
	require.InDelta(t, 18000.0, ps["daily_pnl"].(float64), 1e-6)
	row := snap["positions"].([]any)[0].(map[string]any)
	require.InDelta(t, 3000.0, row["pnl"].(float64), 1e-6)
	require.NotEqual(t, row["pnl"], ps["daily_pnl"])

	dashRec := httptest.NewRecorder()
	mux.ServeHTTP(dashRec, httptest.NewRequest(http.MethodGet, "/api/portfolio/dashboard?trade_date=2026-08-19", nil))
	require.Equal(t, http.StatusOK, dashRec.Code)
}

func TestInvestmentHome_P5D_CaseD_TradePlanStatusUnchanged(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedSimPositionHome(t)

	mux := http.NewServeMux()
	api.RegisterInvestmentHomeRoutes(mux)
	api.RegisterTradingDayMonitorRoutes(mux)

	homeRec := httptest.NewRecorder()
	mux.ServeHTTP(homeRec, httptest.NewRequest(http.MethodGet, "/api/investment/home?trade_date=2026-08-19", nil))
	require.Equal(t, http.StatusOK, homeRec.Code)
	var homeBody map[string]any
	require.NoError(t, json.Unmarshal(homeRec.Body.Bytes(), &homeBody))
	h := homeBody["home"].(map[string]any)
	ts := h["trading_status"].(map[string]any)

	monRec := httptest.NewRecorder()
	mux.ServeHTTP(monRec, httptest.NewRequest(http.MethodGet, "/api/trading/day-monitor?trade_date=2026-08-19", nil))
	require.Equal(t, http.StatusOK, monRec.Code)
	var monBody map[string]any
	require.NoError(t, json.Unmarshal(monRec.Body.Bytes(), &monBody))
	mon := monBody["monitor"].(map[string]any)
	morning := mon["morning"].(map[string]any)
	exec := mon["execution"].(map[string]any)
	settle := mon["settlement"].(map[string]any)

	require.Equal(t, morning["materialize"].(map[string]any)["status"], ts["materialize_status"])
	require.Equal(t, morning["approve"].(map[string]any)["status"], ts["approve_status"])
	require.Equal(t, morning["freeze"].(map[string]any)["status"], ts["freeze_status"])
	require.Equal(t, exec["status"], ts["execution_status"])
	require.Equal(t, settle["status"], ts["settlement_status"])
	require.NotEqual(t, ts["execution_status"], h["portfolio_summary"].(map[string]any)["equity"])
}

func TestInvestmentHome_P5D_CaseE_EmptyAccount(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	h := getInvestmentHomeJSON(t, "trade_date=2026-08-19")
	ps := h["portfolio_summary"].(map[string]any)
	require.Equal(t, false, ps["found"])
	require.Equal(t, float64(0), ps["cash"])
	require.Equal(t, float64(0), ps["equity"])
	pos := h["position_states"].(map[string]any)["positions"].([]any)
	require.Empty(t, pos)

	var n int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestInvestmentHome_P5D_ServiceMatchesReadModel(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedSimPositionHome(t)

	view := home.NewService(nil).Build(home.Query{TradeDate: "2026-08-19"})
	require.True(t, view.PortfolioSummary.Found)
	require.InDelta(t, 700_000, view.PortfolioSummary.Cash, 1e-9)
	require.InDelta(t, 718_000, view.PortfolioSummary.Equity, 1e-9)
	require.Equal(t, int64(1500), view.PositionStates.Positions[0].TotalQty)
}
