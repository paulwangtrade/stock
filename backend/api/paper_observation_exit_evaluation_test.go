package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedExitEvalAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedExitEvalFill(
	t *testing.T,
	accID uint,
	tradeDate, code, name string,
	price float64,
	vol int64,
) (*models.TradePlan, *papertrading.PaperSimFill) {
	t.Helper()
	return seedExitEvalFillWithMeta(t, accID, tradeDate, code, name, price, vol, models.TradePlanStatusReady, "", "", "", "")
}

func seedExitEvalFillWithMeta(
	t *testing.T,
	accID uint,
	tradeDate, code, name string,
	price float64,
	vol int64,
	planStatus, strategy, reason, entryRule, intentStatus string,
) (*models.TradePlan, *papertrading.PaperSimFill) {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{TradeDate: tradeDate, GeneratedAt: now, Status: planStatus, PlanVersion: 1}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: tradeDate, StockCode: code, StockName: name,
		Side: "buy", Status: models.TradePlanItemPending,
		StrategyName: strategy, Reason: reason, EntryRule: entryRule, IntentStatus: intentStatus,
	}
	require.NoError(t, db.Dao.Create(item).Error)
	order := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: code, StockName: name, Side: "buy", Quantity: vol, OrderPrice: price,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: code, StockName: name, Side: "buy", Price: price, Volume: vol, FilledAt: now,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	return plan, fill
}

func getExitEvaluation(t *testing.T) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/holdings/exit-evaluation", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	ev, ok := body["exit_evaluation"].(map[string]any)
	require.True(t, ok)
	return ev
}

func TestExitEvaluationAPI_Normal(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	// buy date = today → holding_days 0, profit → NORMAL
	today := time.Now().Format("2006-01-02")
	_, fill := seedExitEvalFill(t, acc.ID, today, "sh600363", "联创光电", 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 11,
	}).Error)

	ev := getExitEvaluation(t)
	holdings := ev["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	eval := row["evaluation"].(map[string]any)
	require.Equal(t, "NORMAL", eval["state"])
	lots := row["lots"].([]any)
	require.Len(t, lots, 1)
	require.Equal(t, float64(fill.ID), lots[0].(map[string]any)["fill_id"])
}

func TestExitEvaluationAPI_TimeReview(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	_, _ = seedExitEvalFill(t, acc.ID, "2026-01-01", "sh600105", "永鼎股份", 30.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600105", StockName: "永鼎股份",
		TotalVolume: 1000, AvgCost: 30, MarkPrice: 35,
	}).Error)

	ev := getExitEvaluation(t)
	row := ev["holdings"].([]any)[0].(map[string]any)
	eval := row["evaluation"].(map[string]any)
	require.Equal(t, "WATCH", eval["state"])
	codes := eval["reason_codes"].([]any)
	require.Contains(t, codes, "TIME_REVIEW")
}

func TestExitEvaluationAPI_LossReview(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	today := time.Now().Format("2006-01-02")
	_, _ = seedExitEvalFill(t, acc.ID, today, "sz000001", "平安银行", 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 8.5, // -15%
	}).Error)

	ev := getExitEvaluation(t)
	eval := ev["holdings"].([]any)[0].(map[string]any)["evaluation"].(map[string]any)
	require.Equal(t, "REVIEW_REQUIRED", eval["state"])
	require.Contains(t, eval["reason_codes"].([]any), "LOSS_REVIEW")
}

func TestExitEvaluationAPI_MultiLot(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	p32, f32 := seedExitEvalFill(t, acc.ID, "2026-08-01", "sh600363", "联创光电", 10.0, 3000)
	p35, f35 := seedExitEvalFill(t, acc.ID, "2026-08-03", "sh600363", "联创光电", 12.0, 2000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 5000, AvgCost: 10.8, MarkPrice: 11.5,
	}).Error)

	ev := getExitEvaluation(t)
	lots := ev["holdings"].([]any)[0].(map[string]any)["lots"].([]any)
	require.Len(t, lots, 2)
	plans := map[float64]bool{}
	fills := map[float64]bool{}
	for _, raw := range lots {
		lot := raw.(map[string]any)
		plans[lot["plan_id"].(float64)] = true
		fills[lot["fill_id"].(float64)] = true
	}
	require.True(t, plans[float64(p32.ID)])
	require.True(t, plans[float64(p35.ID)])
	require.True(t, fills[float64(f32.ID)])
	require.True(t, fills[float64(f35.ID)])
}

func TestExitEvaluationAPI_MissingPrice(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	today := time.Now().Format("2006-01-02")
	_, _ = seedExitEvalFill(t, acc.ID, today, "sz000002", "万科A", 8.0, 500)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000002", StockName: "万科A",
		TotalVolume: 500, AvgCost: 8, MarkPrice: 0,
	}).Error)

	ev := getExitEvaluation(t)
	eval := ev["holdings"].([]any)[0].(map[string]any)["evaluation"].(map[string]any)
	require.Equal(t, "NORMAL", eval["state"])
	codes, _ := eval["reason_codes"].([]any)
	for _, c := range codes {
		require.NotEqual(t, "LOSS_REVIEW", c)
	}
}

func TestExitEvaluationAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/holdings/exit-evaluation", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestExitEvaluationAPI_LazyOutcomeTable(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	today := time.Now().Format("2006-01-02")
	_, _ = seedExitEvalFill(t, acc.ID, today, "sh600363", "联创光电", 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 11,
	}).Error)

	require.True(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))
	require.NoError(t, db.Dao.Migrator().DropTable(&papertrading.ExitReviewOutcome{}))
	require.False(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))

	ev := getExitEvaluation(t)
	require.NotNil(t, ev)
	require.True(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))
}

func TestExitEvaluationAPI_EntryContextAndPlanReview(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	today := time.Now().Format("2006-01-02")
	plan, fill := seedExitEvalFillWithMeta(
		t, acc.ID, today, "sh600363", "联创光电", 10.0, 1000,
		models.TradePlanStatusSuperseded,
		"momentum_v1", "突破20日高", "open_ge_ref", "ready",
	)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 10.5,
	}).Error)

	ev := getExitEvaluation(t)
	row := ev["holdings"].([]any)[0].(map[string]any)
	eval := row["evaluation"].(map[string]any)
	require.Equal(t, "WATCH", eval["state"])
	require.Contains(t, eval["reason_codes"].([]any), "PLAN_REVIEW")

	lot := row["lots"].([]any)[0].(map[string]any)
	require.Equal(t, float64(fill.ID), lot["fill_id"])
	ctx := lot["context"].(map[string]any)
	entry := ctx["entry"].(map[string]any)
	require.Equal(t, "momentum_v1", entry["strategy_name"])
	require.Equal(t, "突破20日高", entry["entry_reason"])
	require.Equal(t, "open_ge_ref", entry["entry_rule"])
	require.Equal(t, "ready", entry["intent_status"])
	planCtx := ctx["plan"].(map[string]any)
	require.Equal(t, float64(plan.ID), planCtx["plan_id"])
	require.Equal(t, today, planCtx["trade_date"])
	require.Equal(t, models.TradePlanStatusSuperseded, planCtx["plan_status"])
}
