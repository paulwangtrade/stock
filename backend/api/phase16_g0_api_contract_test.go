package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/tradeplanorigin"

	"github.com/stretchr/testify/require"
)

// Phase16-G0 contract tests: cross-API wire semantics frozen by PHASE16_API_CONTRACT_AUDIT.md.

func assertJSONNoMissingSentinel(t *testing.T, raw []byte) {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal(raw, &v))
	walkJSONStrings(v, func(s string) {
		require.NotEqual(t, tradeplanorigin.Missing, s,
			"forbidden %q sentinel in response JSON", tradeplanorigin.Missing)
	})
}

func walkJSONStrings(v any, fn func(string)) {
	switch x := v.(type) {
	case map[string]any:
		for _, val := range x {
			walkJSONStrings(val, fn)
		}
	case []any:
		for _, val := range x {
			walkJSONStrings(val, fn)
		}
	case string:
		fn(x)
	}
}

func g0ContractMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	api.RegisterPortfolioDashboardRoutes(mux)
	api.RegisterTradePlansRoutes(mux)
	return mux
}

func TestPhase16G0_Contract_StockCodeNormalization(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 41, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	mux := g0ContractMux(t)
	for _, code := range []string{"301125.SZ", "sz301125", "SZ301125", "301125"} {
		code := code
		t.Run("outcomes/"+code, func(t *testing.T) {
			body := getOutcomesJSON(t, mux, fmt.Sprintf("stock_code=%s&status=OPEN", code))
			items := body["items"].([]any)
			require.Len(t, items, 1)
			require.Equal(t, "sz301125", items[0].(map[string]any)["stock_code"])
			assertJSONNoMissingSentinel(t, []byte(g0MustJSON(t, body)))
		})
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/positions/301125.SZ/provenance", nil))
	require.Equal(t, http.StatusNotFound, rec.Code, "no position seeded for provenance")

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tradeplans/%d/origin", plan.ID), nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var originBody map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &originBody))
	items := originBody["items"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, "sz301125", items[0].(map[string]any)["stock_code"])
	assertJSONNoMissingSentinel(t, rec2.Body.Bytes())
}

func TestPhase16G0_Contract_SingleStockNotFound404(t *testing.T) {
	setupOutcomesAPITestDB(t)
	seedOutcomeAPIAccount(t)
	mux := g0ContractMux(t)

	for _, spec := range []struct {
		name string
		path string
	}{
		{"projections", "/api/opportunities/projections?stock_code=sh999999&trade_date=2026-09-02"},
		{"outcomes", "/api/opportunities/outcomes?stock_code=sh999999"},
		{"provenance", "/api/portfolio/positions/sh999999/provenance"},
	} {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, spec.path, nil))
			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.False(t, body["ok"].(bool))
		})
	}
}

func TestPhase16G0_Contract_ListFilterEmpty200(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 41, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	mux := g0ContractMux(t)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125&status=CLOSED")
	require.Empty(t, body["items"].([]any))
	require.NotNil(t, body["items"])
}

func TestPhase16G0_Contract_OriginPresentBlocks(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	now := time.Now()
	plan := &models.TradePlan{TradeDate: "2026-08-29", GeneratedAt: now, Status: models.TradePlanStatusDraft, PlanVersion: 1}
	planItems := []models.TradePlanItem{
		{TradeDate: "2026-08-29", StockCode: "sh600000", Side: "buy", Status: models.TradePlanItemPending},
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tradeplans/%d/origin", plan.ID), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assertJSONNoMissingSentinel(t, rec.Body.Bytes())

	item := body["items"].([]any)[0].(map[string]any)
	signal := item["signal"].(map[string]any)
	require.Equal(t, false, signal["present"])
	_, hasTag := signal["signal_tag"]
	require.False(t, hasTag)
	_, hasFlatTag := item["signal_tag"]
	require.False(t, hasFlatTag)
}

func TestPhase16G0_Contract_OutcomeMetadataBlock(t *testing.T) {
	setupOutcomesAPITestDB(t)
	seedOutcomeAPIAccount(t)
	seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")

	mux := g0ContractMux(t)
	body := getOutcomesJSON(t, mux, "stock_code=sz301125&trade_date=2026-09-02&status="+outcome.OutcomeStatusNoTrade)
	row := body["items"].([]any)[0].(map[string]any)
	meta := row["metadata"].(map[string]any)
	require.NotEmpty(t, meta["source_type"])
	require.NotEmpty(t, meta["as_of"])
	signal := row["signal"].(map[string]any)
	require.True(t, signal["present"].(bool))
	assertJSONNoMissingSentinel(t, []byte(g0MustJSON(t, body)))
}

func TestPhase16G0_Contract_ProjectionSignalPresent(t *testing.T) {
	setupProjectionsAPITestDB(t)
	snapID := seedProjectionSignal301125(t, "2026-09-01")
	seedProjectionPool301125(t, "2026-09-02", snapID)

	mux := g0ContractMux(t)
	body := getProjectionsJSON(t, mux, "trade_date=2026-09-02&stock_code=sz301125")
	row := body["items"].([]any)[0].(map[string]any)
	signal := row["signal"].(map[string]any)
	require.True(t, signal["present"].(bool))
	require.NotEmpty(t, signal["signal_tag"])
	assertJSONNoMissingSentinel(t, []byte(g0MustJSON(t, body)))
}

func g0MustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

func TestPhase16G0_Contract_NoMissingSubstringInSuccessBodies(t *testing.T) {
	setupOutcomesAPITestDB(t)
	acc := seedOutcomeAPIAccount(t)
	pool := seedOutcomeAPISignalPool301125(t, "2026-09-01", "2026-09-02")
	plan, item := seedOutcomeAPIBuyPlan(t, pool, "2026-09-02")
	seedOutcomeAPIBuyFill(t, acc, plan, item, "2026-09-02", 41, 500, time.Date(2026, 9, 2, 9, 31, 0, 0, time.UTC))

	mux := g0ContractMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/opportunities/outcomes?stock_code=sz301125&status=OPEN", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, strings.Contains(rec.Body.String(), `"missing"`))
}
