package api_test

import (
	"bytes"
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

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPaperAPITestDB(t *testing.T) {
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
	require.NoError(t, data.EnsureTradePlanTables())
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
}

func TestPaperTradingStatus_ReadOnly(t *testing.T) {
	setupPaperAPITestDB(t)
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: now, Status: models.TradePlanStatusReady,
		FreezeAt: &now, FreezeBy: "test", PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", StockName: "平安银行", Side: "buy", Status: models.TradePlanItemPending, TargetVolume: 1000, LimitPrice: 10},
	}))

	papertrading.SetQuoteFetcherForTest(func(codes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{Code: codes[0], Open: "10.05", Price: "10.10"}}, nil
	})
	t.Cleanup(func() { papertrading.SetQuoteFetcherForTest(nil) })

	_, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerManual, Actor: "dev",
		SkipWeekdayCheck: true,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	api.RegisterPaperTradingRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/status?trade_date=2026-07-30", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])

	// POST without actor fails
	rec = httptest.NewRecorder()
	raw, _ := json.Marshal(map[string]any{"trigger": "manual"})
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/run", bytes.NewReader(raw)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
