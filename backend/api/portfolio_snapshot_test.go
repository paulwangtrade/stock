package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/readmodel"

	"github.com/stretchr/testify/require"
)

func getPortfolioSnapshot(t *testing.T, query string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPortfolioDashboardRoutes(mux)
	path := "/api/portfolio/snapshot"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	snap, ok := body["snapshot"].(map[string]any)
	require.True(t, ok)
	return snap
}

type stubSnapshotQuoteService struct {
	quotes []marketdata.Quote
}

func (s *stubSnapshotQuoteService) GetQuote(code string) (*marketdata.Quote, error) {
	for i := range s.quotes {
		if s.quotes[i].Code == code {
			q := s.quotes[i]
			return &q, nil
		}
	}
	return nil, nil
}

func (s *stubSnapshotQuoteService) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	return s.quotes, nil
}

func TestPortfolioSnapshotAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPortfolioSnapshotRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/portfolio/snapshot", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// Case C: empty account → found=false, no INSERT.
func TestPortfolioSnapshotAPI_CaseC_EmptyAccountFoundFalse(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	snap := getPortfolioSnapshot(t, "")
	require.Equal(t, false, snap["found"])
	require.Equal(t, float64(0), snap["cash"])
	require.Equal(t, float64(0), snap["equity"])
	require.Equal(t, float64(0), snap["market_value"])
	pos, _ := snap["positions"].([]any)
	require.Empty(t, pos)
	require.Contains(t, snap, "updated_at")

	var n int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

// Case A: paper_sim has a position; paper_* has a different (or any) legacy row — API returns only sim.
func TestPortfolioSnapshotAPI_CaseA_SimNotLegacy(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, data.MigratePaperTrading(db.Dao))

	legacyAcc := &data.PaperAccount{Name: "默认模拟账户", Cash: 50_000, InitialCash: 50_000, Equity: 80_000}
	require.NoError(t, db.Dao.Create(legacyAcc).Error)
	require.NoError(t, db.Dao.Create(&data.PaperPosition{
		AccountID: legacyAcc.ID, StockCode: "sz000002", StockName: "万科A",
		Volume: 9999, Sellable: 9999, AvgCost: 1, MarkPrice: 2,
	}).Error)

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 700_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1000, AvailableVolume: 1000, LockedVolume: 0, AvgCost: 10, MarkPrice: 12,
	}).Error)

	snap := getPortfolioSnapshot(t, "")
	require.Equal(t, true, snap["found"])
	require.InDelta(t, 700_000.0, snap["cash"].(float64), 1e-6)
	require.InDelta(t, 12_000.0, snap["market_value"].(float64), 1e-6)
	require.InDelta(t, 712_000.0, snap["equity"].(float64), 1e-6)

	positions := snap["positions"].([]any)
	require.Len(t, positions, 1)
	row := positions[0].(map[string]any)
	require.Equal(t, "sh600000", row["stock_code"])
	require.Equal(t, "浦发", row["stock_name"])
	require.Equal(t, float64(1000), row["total_qty"])
	require.InDelta(t, 10.0, row["avg_cost"].(float64), 1e-6)
	require.InDelta(t, 12.0, row["mark_price"].(float64), 1e-6)
	require.InDelta(t, 2000.0, row["pnl"].(float64), 1e-6)
	require.InDelta(t, 0.2, row["pnl_percent"].(float64), 1e-6)
	require.NotContains(t, row, "sz000002")

	for _, p := range positions {
		m := p.(map[string]any)
		require.NotEqual(t, "sz000002", m["stock_code"])
		require.NotEqual(t, float64(9999), m["total_qty"])
	}
}

// Case B: available / locked come from paper_sim_positions.
func TestPortfolioSnapshotAPI_CaseB_AvailableLocked(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 800_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安",
		TotalVolume: 1500, AvailableVolume: 1000, LockedVolume: 500, AvgCost: 10, MarkPrice: 11,
	}).Error)

	snap := getPortfolioSnapshot(t, "trade_date=2026-08-19")
	require.Equal(t, true, snap["found"])
	row := snap["positions"].([]any)[0].(map[string]any)
	require.Equal(t, float64(1500), row["total_qty"])
	require.Equal(t, float64(1000), row["available_qty"])
	require.Equal(t, float64(500), row["locked_qty"])
	ps := row["position_state"].(map[string]any)
	require.Equal(t, "S3_PARTIAL_LOCKED", ps["state"])
	require.Equal(t, float64(1000), ps["available_qty"])
	require.Equal(t, float64(500), ps["locked_qty"])
}

// Case D: include_display fills display_* and must not change accounting fields or DB mark.
func TestPortfolioSnapshotAPI_CaseD_IncludeDisplayDoesNotChangeAccounting(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 700_000, Equity: 999_999,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	pos := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 12,
	}
	require.NoError(t, db.Dao.Create(&pos).Error)

	svc := readmodel.NewService(nil)
	svc.SetQuoteServiceForTest(&stubSnapshotQuoteService{quotes: []marketdata.Quote{{
		Code: "sh600000", Price: 20,
	}}})
	mux := http.NewServeMux()
	mux.Handle("/api/portfolio/snapshot", api.NewPortfolioSnapshotHandler(svc))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/snapshot?include_display=1", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	snap := body["snapshot"].(map[string]any)

	require.InDelta(t, 700_000.0, snap["cash"].(float64), 1e-6)
	require.InDelta(t, 12_000.0, snap["market_value"].(float64), 1e-6)
	require.InDelta(t, 712_000.0, snap["equity"].(float64), 1e-6)

	row := snap["positions"].([]any)[0].(map[string]any)
	require.InDelta(t, 12.0, row["mark_price"].(float64), 1e-6)
	require.InDelta(t, 10.0, row["avg_cost"].(float64), 1e-6)
	require.InDelta(t, 2000.0, row["pnl"].(float64), 1e-6)
	require.InDelta(t, 20.0, row["display_price"].(float64), 1e-6)
	require.Equal(t, "live", row["display_quote_source"])
	require.InDelta(t, 10_000.0, row["display_pnl"].(float64), 1e-6)

	var mark float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Where("id = ?", pos.ID).Pluck("mark_price", &mark).Error)
	require.InDelta(t, 12.0, mark, 1e-9)
	var cash float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Where("id = ?", acc.ID).Pluck("cash", &cash).Error)
	require.InDelta(t, 700_000, cash, 1e-9)
	var storedEquity float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Where("id = ?", acc.ID).Pluck("equity", &storedEquity).Error)
	require.InDelta(t, 999_999, storedEquity, 1e-9)
}

// Case E: old dashboard API still returns its envelope (quantity / market_price / unrealized_pnl).
func TestPortfolioSnapshotAPI_CaseE_DashboardRegression(t *testing.T) {
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

	mux := http.NewServeMux()
	api.RegisterPortfolioDashboardRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/dashboard?trade_date=2026-08-19", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	dash := body["dashboard"].(map[string]any)
	require.Equal(t, true, dash["found"])
	summary := dash["summary"].(map[string]any)
	require.InDelta(t, 712_000.0, summary["equity"].(float64), 1e-6)
	row := dash["positions"].([]any)[0].(map[string]any)
	require.Equal(t, float64(1000), row["quantity"])
	require.InDelta(t, 12.0, row["market_price"].(float64), 1e-6)
	require.InDelta(t, 2000.0, row["unrealized_pnl"].(float64), 1e-6)
	_, hasSnapshotKey := body["snapshot"]
	require.False(t, hasSnapshotKey)

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/portfolio/snapshot", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &body2))
	require.Contains(t, body2, "snapshot")
	_, hasDash := body2["dashboard"]
	require.False(t, hasDash)
}

func TestPortfolioSnapshotAssetMiddleware_ExplicitRoute(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))

	calledNext := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.PortfolioDashboardAssetMiddleware(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/snapshot", nil))
	require.False(t, calledNext)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Contains(t, body, "snapshot")
	require.Equal(t, false, body["snapshot"].(map[string]any)["found"])
}
