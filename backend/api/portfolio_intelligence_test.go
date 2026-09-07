package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupIntelAPITestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:intel_api_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
}

func TestPortfolioIntelligenceAPI_Holdings(t *testing.T) {
	setupIntelAPITestDB(t)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", Cash: 500_000, InitialCash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 11,
	}).Error)

	mux := http.NewServeMux()
	api.RegisterPortfolioIntelligenceRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/intelligence", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	pos := body["positions"].([]any)
	require.Len(t, pos, 1)
	row := pos[0].(map[string]any)
	require.Equal(t, "sz000001", row["stock_code"])
	require.Equal(t, intelligence.StatusHoldingProfit, row["position_status"])
}

func TestPortfolioIntelligenceAPI_NoPositionExtra(t *testing.T) {
	setupIntelAPITestDB(t)
	mux := http.NewServeMux()
	api.RegisterPortfolioIntelligenceRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/intelligence?stock_codes=sz399001", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	pos := body["positions"].([]any)
	require.NotEmpty(t, pos)
	row := pos[0].(map[string]any)
	require.Equal(t, intelligence.StatusNoPosition, row["position_status"])
}

func TestPortfolioIntelligenceAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPortfolioIntelligenceRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/portfolio/intelligence", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPortfolioIntelligence_BuildUsesSnapshotOnly(t *testing.T) {
	// Ensure package path compile: snapshot → intelligence without paper_sim writes.
	snap := &portfolio.Snapshot{Found: false}
	b := intelligence.Build(intelligence.Options{Snapshot: snap, ExtraCodes: []string{"x"}})
	require.Equal(t, intelligence.StatusNoPosition, b.Positions[0].PositionStatus)
}
