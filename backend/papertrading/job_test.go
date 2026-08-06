package papertrading_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestPaperTradingJob_FlagOff_NoOrders(t *testing.T) {
	setupTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: false})

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		PlanID:           plan.ID,
		Trigger:          papertrading.TriggerCron,
		Actor:            "cron",
		Price:            papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05}}},
		SkipWeekdayCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusSkippedDisabled, res.Status)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, st.OrdersTotal)
}

func TestPaperTradingJob_FrozenPlan_CreatesOrders(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        plan.TradeDate,
		Trigger:          papertrading.TriggerCron,
		Actor:            "cron",
		Price:            papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05, LimitUp: 11}}},
		SkipWeekdayCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusCompleted, res.Status)
	require.Equal(t, 1, res.FilledCount)

	st, err := papertrading.GetTodayStatus(plan.TradeDate)
	require.NoError(t, err)
	require.Equal(t, 1, st.FilledCount)
	require.Len(t, st.Orders, 1)
	require.Len(t, st.Positions, 1)
	require.Equal(t, int64(1000), st.Positions[0].LockedVolume)
	require.Equal(t, int64(0), st.Positions[0].AvailableVolume)
}

func TestPaperTradingJob_Idempotent_NoDuplicateOrders(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}
	req := papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: price, SkipWeekdayCheck: true,
	}
	res1, err := papertrading.RunExecution(req)
	require.NoError(t, err)
	require.Equal(t, 1, res1.OrdersTotal)

	res2, err := papertrading.RunExecution(req)
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusSkippedAlreadyRun, res2.Status)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, st.OrdersTotal, "must not duplicate orders on re-run")
}

func TestPaperTradingJob_MissingPrice_Rejected(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000003", "国农科技", 1000)})
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: papertrading.MissingPriceProvider{}, SkipWeekdayCheck: true,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.RunStatusCompletedWithRejects, res.Status)
	require.Equal(t, 1, res.RejectCount)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.RejectMissingOpenPrice, st.Orders[0].RejectReason)
}

func TestPaperTradingJob_NotFrozen_Rejected(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := &models.TradePlan{
		TradeDate: "2026-07-30", GeneratedAt: time.Now(), Status: models.TradePlanStatusDraft,
		PlanVersion: 1, Side: "buy", AmountPerStock: 100_000,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)}))

	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, PlanID: plan.ID, Trigger: papertrading.TriggerManual, Actor: "tester",
		Price: papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10}}},
		SkipWeekdayCheck: true,
	})
	require.Error(t, err)
	require.Equal(t, papertrading.RunStatusFailed, res.Status)
}

func TestSettlementJob_DoesNotUnlockSameDay(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)

	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	price := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}}}
	_, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate: plan.TradeDate, Trigger: papertrading.TriggerCron, Actor: "cron",
		Price: price, SkipWeekdayCheck: true,
	})
	require.NoError(t, err)

	settle, err := papertrading.SettlementJob(plan.TradeDate, price, false)
	require.NoError(t, err)
	require.Greater(t, settle.LockedVolumeTotal, int64(0))

	acc, err := papertrading.GetDefaultAccount()
	require.NoError(t, err)
	pos, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), pos[0].LockedVolume)
	require.Equal(t, int64(0), pos[0].AvailableVolume)
}

func TestPaperTradingAPI_RunRequiresActorAndManualTrigger(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})

	mux := http.NewServeMux()
	api.RegisterPaperTradingRoutes(mux)

	// missing actor
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{"trigger": "manual", "plan_id": plan.ID, "trade_date": plan.TradeDate})
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/run", bytes.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// wrong trigger
	rec = httptest.NewRecorder()
	body, _ = json.Marshal(map[string]any{"actor": "dev", "trigger": "cron", "plan_id": plan.ID})
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/run", bytes.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// ok
	papertrading.SetQuoteFetcherForTest(func(codes ...string) (*[]data.StockInfo, error) {
		return &[]data.StockInfo{{Code: codes[0], Open: "10.05", Price: "10.05"}}, nil
	})
	t.Cleanup(func() { papertrading.SetQuoteFetcherForTest(nil) })

	// Use Static via Job path indirectly — API uses DefaultOpenPriceProvider.
	// Inject quote fetcher so realtime provider works.
	rec = httptest.NewRecorder()
	body, _ = json.Marshal(map[string]any{
		"actor": "dev", "trigger": "manual", "plan_id": plan.ID, "trade_date": plan.TradeDate,
	})
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/run", bytes.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.PaperTradingRunResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.Equal(t, papertrading.TriggerManual, resp.Result.Trigger)
	require.Equal(t, papertrading.ExecutionEntryGateway, resp.Result.Entry)
}

func TestBroker_IdempotentCreate_SameItem(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	plan := seedFrozenPlan(t, "2026-07-30", []models.TradePlanItem{buyItem("sz000001", "平安银行", 1000)})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0}},
	})
	r1, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, r1.OrdersTotal)

	r2, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, r2.OrdersTotal)
	require.Equal(t, 1, r2.SkippedAlready)

	st, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, st.OrdersTotal)
}
