package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/readmodel"

	"github.com/stretchr/testify/require"
)

func p5gMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPortfolioDashboardRoutes(mux)
	return mux
}

func p5gGET(t *testing.T, mux http.Handler, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool), rec.Body.String())
	return body
}

func seedP5GSim(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000,
		Cash: 700_000, Equity: 1, MarketValue: 1, UnrealizedPnl: 1,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1500, AvailableVolume: 1000, LockedVolume: 500,
		AvgCost: 10, MarkPrice: 12,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimDailyReport{
		AccountID: acc.ID, ReportDate: "2026-08-18", Equity: 700_000, Cash: 700_000,
	}).Error)
	return acc
}

// pageAssets is what PortfolioDashboard binds from Snapshot (no include_display).
func pageAssets(snap map[string]any) (cash, equity, marketValue float64, pos map[string]any) {
	cash = snap["cash"].(float64)
	equity = snap["equity"].(float64)
	marketValue = snap["market_value"].(float64)
	rows, _ := snap["positions"].([]any)
	if len(rows) > 0 {
		pos = rows[0].(map[string]any)
	}
	return cash, equity, marketValue, pos
}

func TestPortfolio_P5G_CaseA_CashMatchesSnapshot(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5GSim(t)

	mux := p5gMux(t)
	snap := p5gGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	cash, _, _, _ := pageAssets(snap)
	require.Equal(t, true, snap["found"])
	require.InDelta(t, 700_000.0, cash, 1e-6)
	require.InDelta(t, snap["cash"].(float64), cash, 1e-9)
}

func TestPortfolio_P5G_CaseB_EquityMatchesSnapshotNotAccountColumn(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5GSim(t)

	mux := p5gMux(t)
	snap := p5gGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	dash := p5gGET(t, mux, "/api/portfolio/dashboard?trade_date=2026-08-19")["dashboard"].(map[string]any)
	_, equity, mv, pos := pageAssets(snap)
	require.InDelta(t, 18_000.0, mv, 1e-6)
	require.InDelta(t, 718_000.0, equity, 1e-6)
	require.InDelta(t, snap["equity"].(float64), equity, 1e-9)

	summary := dash["summary"].(map[string]any)
	require.InDelta(t, 18_000.0, summary["daily_pnl"].(float64), 1e-6) // 718000-700000
	require.InDelta(t, 3_000.0, pos["pnl"].(float64), 1e-6)            // (12-10)*1500
	require.NotEqual(t, summary["daily_pnl"], pos["pnl"])
}

func TestPortfolio_P5G_CaseC_OverlayDoesNotChangePageAssets(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := seedP5GSim(t)
	var posRow papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ?", acc.ID).First(&posRow).Error)

	svc := readmodel.NewService(nil)
	svc.SetQuoteServiceForTest(&stubSnapshotQuoteService{quotes: []marketdata.Quote{{
		Code: "sh600000", Price: 20,
	}}})
	mux := http.NewServeMux()
	mux.Handle("/api/portfolio/snapshot", api.NewPortfolioSnapshotHandler(svc))
	api.RegisterPortfolioDashboardHandler(mux, nil)

	plain := p5gGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	disp := p5gGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19&include_display=1")["snapshot"].(map[string]any)
	// Page does not send include_display; assets must equal the plain snapshot.
	require.InDelta(t, plain["equity"].(float64), 718_000.0, 1e-6)
	require.InDelta(t, plain["equity"].(float64), disp["equity"].(float64), 1e-9)
	require.InDelta(t, plain["cash"].(float64), disp["cash"].(float64), 1e-9)
	require.InDelta(t, plain["market_value"].(float64), disp["market_value"].(float64), 1e-9)

	row := disp["positions"].([]any)[0].(map[string]any)
	require.InDelta(t, 12.0, row["mark_price"].(float64), 1e-6)
	require.InDelta(t, 20.0, row["display_price"].(float64), 1e-6)
	_, hasDisplay := plain["positions"].([]any)[0].(map[string]any)["display_price"]
	require.False(t, hasDisplay)

	var mark float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Where("id = ?", posRow.ID).Pluck("mark_price", &mark).Error)
	require.InDelta(t, 12.0, mark, 1e-9)
}

func TestPortfolio_P5G_CaseD_QtyMatchesSnapshotTotalQty(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5GSim(t)

	mux := p5gMux(t)
	snap := p5gGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	_, _, _, pos := pageAssets(snap)
	require.Equal(t, float64(1500), pos["total_qty"])
	require.Equal(t, "sh600000", pos["stock_code"])
	require.Equal(t, "浦发", pos["stock_name"])
	require.InDelta(t, 10.0, pos["avg_cost"].(float64), 1e-9)
	require.InDelta(t, 12.0, pos["mark_price"].(float64), 1e-9)
}

func TestPortfolio_P5G_CaseE_OldDashboardAPIUnchanged(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5GSim(t)

	mux := p5gMux(t)
	body := p5gGET(t, mux, "/api/portfolio/dashboard?trade_date=2026-08-19")
	require.Contains(t, body, "dashboard")
	_, hasSnap := body["snapshot"]
	require.False(t, hasSnap)
	dash := body["dashboard"].(map[string]any)
	require.Equal(t, true, dash["found"])
	summary := dash["summary"].(map[string]any)
	require.Contains(t, summary, "equity")
	require.Contains(t, summary, "cash")
	require.Contains(t, summary, "daily_pnl")
	row := dash["positions"].([]any)[0].(map[string]any)
	require.Equal(t, float64(1500), row["quantity"])
	require.Contains(t, row, "market_price")
	require.Contains(t, row, "unrealized_pnl")
	require.Contains(t, dash, "trades")
}
