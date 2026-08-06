package papertrading_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestRunExecution_FrozenPlan_WritesPaperSimOnly(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	// Legacy paper_* tables exist so we can assert zero writes.
	require.NoError(t, db.Dao.AutoMigrate(
		&data.PaperAccount{}, &data.PaperPosition{}, &data.PaperOrder{}, &data.PaperFill{},
	))

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05}}}

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerManual,
		Actor:            "test:c2a",
		Price:            price,
		SkipWeekdayCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionEntryGateway, res.Entry)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)
	require.Equal(t, plan.ID, res.PlanID)

	var simOrders, simFills, simRuns, legacyOrders, legacyFills int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&simOrders).Error)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimFill{}).Count(&simFills).Error)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimRun{}).Count(&simRuns).Error)
	require.NoError(t, db.Dao.Model(&data.PaperOrder{}).Count(&legacyOrders).Error)
	require.NoError(t, db.Dao.Model(&data.PaperFill{}).Count(&legacyFills).Error)

	require.Equal(t, int64(1), simOrders)
	require.Equal(t, int64(1), simFills)
	require.Equal(t, int64(1), simRuns)
	require.Equal(t, int64(0), legacyOrders, "gateway must not write paper_*")
	require.Equal(t, int64(0), legacyFills, "gateway must not write paper_*")
}

func TestRunExecution_ManualAndCron_SameEntry(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	planCron := seedFrozenPlan(t, "2026-07-28", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	planManual := seedFrozenPlan(t, "2026-07-29", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}

	cronRes, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: planCron.TradeDate, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: price, SkipWeekdayCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionEntryGateway, cronRes.Entry)
	require.Equal(t, papertrading.TriggerCron, cronRes.Trigger)

	manualRes, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: planManual.TradeDate, Trigger: papertrading.TriggerManual, Actor: "ui:future",
		Price: price, SkipWeekdayCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionEntryGateway, manualRes.Entry)
	require.Equal(t, papertrading.TriggerManual, manualRes.Trigger)
	require.Equal(t, "ui:future", manualRes.Actor)
}

func TestPaperTradingAssetMiddleware_RunUsesGateway(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	papertrading.SetQuoteFetcherForTest(func(codes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{Code: codes[0], Open: "10.05", Price: "10.05"}}, nil
	})
	t.Cleanup(func() { papertrading.SetQuoteFetcherForTest(nil) })

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	h := api.PaperTradingAssetMiddleware(next)

	body, _ := json.Marshal(map[string]any{
		"actor": "dev", "trigger": "manual", "plan_id": plan.ID, "trade_date": plan.TradeDate,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/run", bytes.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.PaperTradingRunResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.NotNil(t, resp.Result)
	require.Equal(t, papertrading.ExecutionEntryGateway, resp.Result.Entry)
}

func TestPhase10C2A_MainWiring_Markers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	mainBody, err := os.ReadFile(filepath.Join(root, "main.go"))
	require.NoError(t, err)
	require.Contains(t, string(mainBody), "PaperTradingAssetMiddleware")
	require.NotContains(t, string(mainBody), "PaperObservationAssetMiddleware",
		"C.2-A replaces Observation MW with Trading MW (which still serves observation paths)")

	winBody, err := os.ReadFile(filepath.Join(root, "app_windows.go"))
	require.NoError(t, err)
	require.Contains(t, string(winBody), "InitPaperTradingJobs()")

	// Cron open path must call RunExecution, not the unexported job.
	cronBody, err := os.ReadFile(filepath.Join(root, "app_paper_trading.go"))
	require.NoError(t, err)
	require.Contains(t, string(cronBody), "RunExecution")
	require.False(t, strings.Contains(string(cronBody), "paperTradingJob("))
}
