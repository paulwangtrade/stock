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

func p5fMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	api.RegisterPortfolioDashboardRoutes(mux)
	return mux
}

func p5fGET(t *testing.T, mux http.Handler, path string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool), rec.Body.String())
	return body
}

func seedP5FSim(t *testing.T, staleEquity float64) *papertrading.PaperSimAccount {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000,
		Cash: 700_000, Equity: staleEquity, MarketValue: 1, UnrealizedPnl: 1,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1500, AvailableVolume: 1000, LockedVolume: 500,
		AvgCost: 10, MarkPrice: 12,
	}).Error)
	return acc
}

func observationAssetView(snap map[string]any) (cash, equity, marketValue, ledgerPnl float64, pos map[string]any) {
	cash = snap["cash"].(float64)
	equity = snap["equity"].(float64)
	marketValue = snap["market_value"].(float64)
	rows, _ := snap["positions"].([]any)
	if len(rows) == 0 {
		return cash, equity, marketValue, 0, nil
	}
	pos = rows[0].(map[string]any)
	if p, ok := pos["pnl"].(float64); ok {
		ledgerPnl = p
	}
	return cash, equity, marketValue, ledgerPnl, pos
}

// Case A: page cash/equity = Snapshot (not paper_sim_accounts.equity column).
func TestObservation_P5F_CaseA_CashMatchesSnapshotNotAccountColumn(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5FSim(t, 1) // stale persisted equity

	mux := p5fMux(t)
	snap := p5fGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	today := p5fGET(t, mux, "/api/papertrading/dashboard/today?trade_date=2026-08-19")["today"].(map[string]any)

	cash, equity, mv, ledgerPnl, _ := observationAssetView(snap)
	require.InDelta(t, 700_000.0, cash, 1e-6)
	require.InDelta(t, 18_000.0, mv, 1e-6)      // 12 × 1500
	require.InDelta(t, 718_000.0, equity, 1e-6) // 700000 + 18000
	require.InDelta(t, 3_000.0, ledgerPnl, 1e-6)
	require.Equal(t, float64(1), snap["position_count"])

	// Old today API still returns persisted column; page must NOT use it as equity authority.
	require.InDelta(t, 1.0, today["equity"].(float64), 1e-6)
	require.NotEqual(t, today["equity"], snap["equity"])
	require.InDelta(t, snap["cash"].(float64), cash, 1e-6)
}

// Case B: Snapshot total_qty = available + locked (fixture); page reads those fields, does not re-sum.
func TestObservation_P5F_CaseB_TotalQtyEqualsAvailablePlusLocked(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5FSim(t, 1)

	mux := p5fMux(t)
	snap := p5fGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	_, _, _, _, pos := observationAssetView(snap)
	require.NotNil(t, pos)
	total := pos["total_qty"].(float64)
	avail := pos["available_qty"].(float64)
	locked := pos["locked_qty"].(float64)
	require.Equal(t, float64(1500), total)
	require.Equal(t, float64(1000), avail)
	require.Equal(t, float64(500), locked)
	require.Equal(t, avail+locked, total)
}

// Case C: quote overlay changes display_* only; Snapshot equity unchanged; DB mark/equity columns unchanged.
func TestObservation_P5F_CaseC_OverlayDoesNotChangeEquity(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := seedP5FSim(t, 1)
	var posRow papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ?", acc.ID).First(&posRow).Error)

	svc := readmodel.NewService(nil)
	svc.SetQuoteServiceForTest(&stubSnapshotQuoteService{quotes: []marketdata.Quote{{
		Code: "sh600000", Price: 20,
	}}})
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	mux.Handle("/api/portfolio/snapshot", api.NewPortfolioSnapshotHandler(svc))

	plain := p5fGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19")["snapshot"].(map[string]any)
	disp := p5fGET(t, mux, "/api/portfolio/snapshot?trade_date=2026-08-19&include_display=1")["snapshot"].(map[string]any)

	require.InDelta(t, plain["equity"].(float64), disp["equity"].(float64), 1e-9)
	require.InDelta(t, 718_000.0, disp["equity"].(float64), 1e-6)

	row := disp["positions"].([]any)[0].(map[string]any)
	require.InDelta(t, 12.0, row["mark_price"].(float64), 1e-6)
	require.InDelta(t, 3_000.0, row["pnl"].(float64), 1e-6)
	require.InDelta(t, 20.0, row["display_price"].(float64), 1e-6)
	require.InDelta(t, 15_000.0, row["display_pnl"].(float64), 1e-6) // (20-10)*1500
	require.NotEqual(t, row["pnl"], row["display_pnl"])

	var mark float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Where("id = ?", posRow.ID).Pluck("mark_price", &mark).Error)
	require.InDelta(t, 12.0, mark, 1e-9)
	var storedEquity float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Where("id = ?", acc.ID).Pluck("equity", &storedEquity).Error)
	require.InDelta(t, 1.0, storedEquity, 1e-9)
}

// Case D: Observation metrics envelope unchanged (not merged into Snapshot).
func TestObservation_P5F_CaseD_MetricsUnchanged(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5FSim(t, 1)

	mux := p5fMux(t)
	body := p5fGET(t, mux, "/api/papertrading/observation/metrics?trade_date=2026-08-19")
	require.Contains(t, body, "metrics")
	m := body["metrics"].(map[string]any)
	require.Contains(t, m, "totalRuns")
	require.Contains(t, m, "sessionDistribution")
	require.Contains(t, m, "fillPolicy")
	require.Contains(t, m, "quality")
	require.Contains(t, m, "legacy")
	require.Contains(t, m, "pricePolicyCompliance")
	_, hasSnap := m["equity"]
	require.False(t, hasSnap)
	snap := p5fGET(t, mux, "/api/portfolio/snapshot")["snapshot"].(map[string]any)
	_, hasRuns := snap["totalRuns"]
	require.False(t, hasRuns)
}

// Case E: old observation dashboard APIs still work.
func TestObservation_P5F_CaseE_OldAPIsCompatible(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedP5FSim(t, 1)

	mux := p5fMux(t)

	todayBody := p5fGET(t, mux, "/api/papertrading/dashboard/today?trade_date=2026-08-19")
	today := todayBody["today"].(map[string]any)
	require.Contains(t, today, "cash")
	require.Contains(t, today, "equity")
	require.Contains(t, today, "ordersTotal")
	require.Contains(t, today, "filledCount")
	require.Contains(t, today, "runStatus")

	posBody := p5fGET(t, mux, "/api/papertrading/dashboard/positions")
	pos := posBody["positions"].(map[string]any)
	require.Contains(t, pos, "cash")
	require.Contains(t, pos, "observationEquity")
	rows := pos["positions"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	require.Equal(t, float64(1500), row["totalVolume"])
	require.Contains(t, row, "availableVolume")
	require.Contains(t, row, "lockedVolume")

	attr := p5fGET(t, mux, "/api/papertrading/observation/positions/attribution")
	require.Contains(t, attr, "attribution")
}
