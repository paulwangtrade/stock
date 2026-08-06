package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupObservationMetricsDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
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
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true})
	t.Cleanup(papertrading.ResetConfigCache)
}

func TestObservationMetricsAPI_LegacyExcluded(t *testing.T) {
	setupObservationMetricsDB(t)
	filledAt := time.Date(2026, 8, 5, 15, 14, 20, 0, time.Local)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimRun{
		ID: 2, ExecutionID: "exec-legacy", TradeDate: "2026-08-05", PlanID: 26,
		Trigger: "manual", Actor: "verify:b1-paper", Status: papertrading.RunStatusCompleted,
		StartedAt: filledAt, FilledCount: 5,
	}).Error)
	for id := uint(6); id <= 10; id++ {
		require.NoError(t, db.Dao.Create(&papertrading.PaperSimOrder{
			ID: id, PlanID: 26, PlanItemID: id, TradeDate: "2026-08-05",
			StockCode: fmt.Sprintf("code%d", id), Side: "buy", Status: papertrading.OrderStatusFilled,
			OrderTime: filledAt,
		}).Error)
		require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
			ID: id, OrderID: id, PlanID: 26, PlanItemID: id,
			StockCode: fmt.Sprintf("code%d", id), Side: "buy", Price: 10, Volume: 100,
			FillReason: papertrading.FillReasonMarketOpen, FilledAt: filledAt,
		}).Error)
	}

	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/metrics?trade_date=2026-08-05", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])
	legacy := body["legacy"].(map[string]any)
	require.Greater(t, legacy["legacyBaselineCount"].(float64), 0.0)
	fillPolicy := body["fillPolicy"].(map[string]any)
	require.Equal(t, float64(0), fillPolicy["bWindowOpenFills"])
}
