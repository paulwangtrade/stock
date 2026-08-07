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
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/marketdata"
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
		Now:              sessionANow(),
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
		Price: price, SkipWeekdayCheck: true, Now: sessionANow(),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionEntryGateway, cronRes.Entry)
	require.Equal(t, papertrading.TriggerCron, cronRes.Trigger)

	manualRes, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: planManual.TradeDate, Trigger: papertrading.TriggerManual, Actor: "ui:future",
		Price: price, SkipWeekdayCheck: true, Now: sessionANow(),
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
	papertrading.SetExecutionNowForTest(func() time.Time {
		return time.Date(2026, 8, 5, 10, 0, 0, 0, time.Local)
	})
	t.Cleanup(func() { papertrading.SetExecutionNowForTest(nil) })

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
	// Phase10-C.4-A: Session B exclusive cron spec + key present; still via Gateway.
	require.Contains(t, string(cronBody), "paper_trading_session_b")
	require.Contains(t, string(cronBody), "0 10 15 * * 1-5")
	require.Contains(t, string(cronBody), "runPaperTradingSessionBJob")
	require.Contains(t, string(cronBody), "FillCronExclusive")
}

func TestRunExecution_SessionB_UsesCloseFill(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})

	day := time.Date(2026, 8, 5, 15, 0, 0, 0, time.Local)
	stub := &stubCloseKline{bars: []marketdata.Bar{
		{Time: day, TimeText: "2026-08-05", Open: 10.05, Close: 12.34},
	}}
	papertrading.SetCloseKlineServiceForTest(func() marketdata.KlineService { return stub })
	t.Cleanup(func() { papertrading.SetCloseKlineServiceForTest(nil) })

	// Cron-style DefaultOpen must be overridden by SelectFillProvider on B.
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerCron, Actor: "cron",
		Price:            papertrading.DefaultOpenPriceProvider(),
		SkipWeekdayCheck: true,
		Now:              atClock(15, 10, 0),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionB, res.Session)
	require.Equal(t, papertrading.PolicyAllow, res.Decision)
	require.Equal(t, papertrading.PriceModeClose, res.PriceMode)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Len(t, st.Fills, 1)
	require.InDelta(t, 12.34, st.Fills[0].Price, 1e-9)
	require.Equal(t, papertrading.FillReasonMarketClose, st.Fills[0].FillReason)
	require.NotEqual(t, 10.05, st.Fills[0].Price)
}

func TestRunExecution_SessionB_MissingClose_NoFill(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-08-05", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})

	papertrading.SetCloseKlineServiceForTest(func() marketdata.KlineService {
		return &stubCloseKline{bars: nil}
	})
	t.Cleanup(func() { papertrading.SetCloseKlineServiceForTest(nil) })

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID,
		Trigger: papertrading.TriggerManual, Actor: "test",
		Price:            papertrading.DefaultOpenPriceProvider(),
		SkipWeekdayCheck: true,
		Now:              atClock(15, 10, 0),
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.SessionB, res.Session)
	require.Equal(t, papertrading.PriceModeClose, res.PriceMode)
	require.Equal(t, 0, res.FilledCount)
	require.Equal(t, 1, res.RejectCount)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, len(st.Fills))
	require.Equal(t, 1, len(st.Orders))
	require.Equal(t, papertrading.RejectMissingClosePrice, st.Orders[0].RejectReason)
}
