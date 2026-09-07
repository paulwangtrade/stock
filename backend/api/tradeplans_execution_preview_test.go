package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// Phase10-T: Execution Preview fields must appear on upcoming + plan-by-id JSON
// without changing existing keys. AfterClose zeros for limit/volume are normal.
func TestTradePlansUpcoming_ExecutionPreviewJSONFields(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	today := "2026-08-08"
	slip := 0.03
	require.NoError(t, repo.CreatePlanWithItems(&models.TradePlan{
		TradeDate: "2026-08-10", GeneratedAt: time.Now(), PoolID: 91,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
		PricingStage:  "after_close_intent",
	}, []models.TradePlanItem{
		{
			StockCode: "sh600363", StockName: "联创光电", Side: "buy",
			Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
			RefPrice: 12.5, RefSource: "prev_close", EntryRule: "LIMIT_REF_PLUS_SLIP",
			MaxSlippage: &slip, IntentStatus: "selected",
			LimitPrice: 0, TargetVolume: 0, // AfterClose normal
		},
		{
			StockCode: "sz301677", StockName: "欣兴工具", Side: "buy",
			Priority: 2, TargetAmount: 100_000, Status: models.TradePlanItemPending,
			RefPrice: 28.0, EntryRule: "LIMIT_REF_PLUS_SLIP", IntentStatus: "priced",
			LimitPrice: 28.84, TargetVolume: 3400,
		},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/upcoming?trade_date="+today, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.EqualValues(t, 0, envelope["code"])

	plan, ok := envelope["plan"].(map[string]any)
	require.True(t, ok)
	items, ok := plan["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)

	first := items[0].(map[string]any)
	// Existing keys still present
	require.Contains(t, first, "stock_code")
	require.Contains(t, first, "target_amount")
	require.Contains(t, first, "status")
	// Preview keys
	require.Contains(t, first, "ref_price")
	require.Contains(t, first, "limit_price")
	require.Contains(t, first, "target_volume")
	require.Contains(t, first, "entry_rule")
	require.Contains(t, first, "intent_status")

	require.Equal(t, "sh600363", first["stock_code"])
	require.InDelta(t, 12.5, first["ref_price"].(float64), 1e-9)
	require.InDelta(t, 0.0, first["limit_price"].(float64), 1e-9)
	require.EqualValues(t, 0, first["target_volume"])
	require.Equal(t, "LIMIT_REF_PLUS_SLIP", first["entry_rule"])
	require.Equal(t, "selected", first["intent_status"])

	second := items[1].(map[string]any)
	require.Equal(t, "sz301677", second["stock_code"])
	require.InDelta(t, 28.84, second["limit_price"].(float64), 1e-9)
	require.EqualValues(t, 3400, second["target_volume"])
	require.Equal(t, "priced", second["intent_status"])
}

func TestTradePlansPlanByID_ExecutionPreviewJSONFields(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	repo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-08-11", GeneratedAt: time.Now(), PoolID: 92,
		Status: models.TradePlanStatusDraft, PlanVersion: 2,
		SourceSession: models.TradePlanSourceAfterClose,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{
			StockCode: "sz000001", StockName: "平安银行", Side: "buy",
			Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
			RefPrice: 10.1, LimitPrice: 0, TargetVolume: 0,
			EntryRule: "LIMIT_REF_PLUS_SLIP", IntentStatus: "",
		},
	}))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/api/tradeplans/plan?plan_id="+strconv.FormatUint(uint64(plan.ID), 10),
		nil,
	))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.UpcomingTradePlanResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.NotNil(t, resp.Plan)
	require.Len(t, resp.Plan.Items, 1)
	it := resp.Plan.Items[0]
	require.Equal(t, "sz000001", it.StockCode)
	require.InDelta(t, 10.1, it.RefPrice, 1e-9)
	require.Equal(t, float64(0), it.LimitPrice)
	require.Equal(t, int64(0), it.TargetVolume)
	require.Equal(t, "LIMIT_REF_PLUS_SLIP", it.EntryRule)
	require.Equal(t, "", it.IntentStatus)
}
