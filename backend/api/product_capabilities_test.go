package api_test

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

	"github.com/stretchr/testify/require"
)

func TestProductCapabilities_FeatureGate_FreeVsPro(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/product/feature-gate?tier=free", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var freeBody map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &freeBody))
	require.True(t, freeBody["ok"].(bool))
	require.Equal(t, "free", freeBody["tier"])
	feats := freeBody["features"].([]any)
	require.NotEmpty(t, feats)
	for _, raw := range feats {
		f := raw.(map[string]any)
		require.False(t, f["allowed"].(bool), "free must deny %v", f["feature"])
	}

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/product/feature-gate?tier=pro", nil))
	require.Equal(t, http.StatusOK, rr2.Code)
	var proBody map[string]any
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &proBody))
	require.Equal(t, "pro", proBody["tier"])
	proAllow := map[string]bool{}
	for _, raw := range proBody["features"].([]any) {
		f := raw.(map[string]any)
		proAllow[f["feature"].(string)] = f["allowed"].(bool)
	}
	require.True(t, proAllow["AdvancedRisk"])
	require.True(t, proAllow["AIAnalysis"])
	require.True(t, proAllow["AdvancedObservation"])
	require.True(t, proAllow["Backtest"])
	require.False(t, proAllow["RealtimeSignal"])
	require.False(t, proAllow["MultiAccount"])
}

func TestProductCapabilities_StrategyExplanation_RequiresIDs(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/product/strategy-explanation?tier=pro", nil))
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProductCapabilities_StrategyExplanation_GatedForFree(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet,
		"/api/product/strategy-explanation?tier=free&plan_id=1&plan_item_id=1", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	expl := body["explanation"].(map[string]any)
	require.Equal(t, "gated", expl["status"])
}

func TestProductCapabilities_RiskReport_GatedForFree(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/product/risk-report?tier=free", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	report := body["report"].(map[string]any)
	require.Equal(t, "gated", report["status"])
}

func TestProductCapabilities_Usage_RecordsOpenedAndViewed(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	payload := []byte(`{
		"tier":"pro",
		"feature":"AdvancedObservation",
		"event":"opened",
		"usage_key":"strategy_explanation_opened",
		"scene":"strategy_explanation"
	}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/product/usage", bytes.NewReader(payload)))
	require.Equal(t, http.StatusOK, rr.Code)
	var body1 map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body1))
	require.True(t, body1["ok"].(bool))
	ev1 := body1["event"].(map[string]any)
	require.Equal(t, "opened", ev1["event_type"])
	meta1 := ev1["metadata"].(map[string]any)
	require.Equal(t, "strategy_explanation_opened", meta1["usage_key"])

	payload2 := []byte(`{
		"tier":"pro",
		"feature":"AdvancedRisk",
		"event":"viewed",
		"usage_key":"risk_report_viewed",
		"scene":"advanced_risk_report"
	}`)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodPost, "/api/product/usage", bytes.NewReader(payload2)))
	require.Equal(t, http.StatusOK, rr2.Code)
	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &body2))
	require.True(t, body2["ok"].(bool))
	ev2 := body2["event"].(map[string]any)
	require.Equal(t, "viewed", ev2["event_type"])
	meta2 := ev2["metadata"].(map[string]any)
	require.Equal(t, "risk_report_viewed", meta2["usage_key"])
}

func TestProductCapabilities_AssetMiddleware_RegisteredInMain(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	mainPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "main.go"))
	b, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	body := string(b)
	require.Contains(t, body, "ProductCapabilitiesAssetMiddleware")
	require.Contains(t, body, "ChainAssetMiddleware")
}

func TestProductCapabilities_AssetMiddleware_RoutesAndPassesThrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("next"))
	})
	h := api.ProductCapabilitiesAssetMiddleware(next)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/product/feature-gate?tier=free", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.True(t, strings.Contains(rr.Body.String(), `"ok":true`))

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/other", nil))
	require.Equal(t, http.StatusTeapot, rr2.Code)
}

func TestProductCapabilities_UsageAnalytics_SummaryAndCount(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	seed := []byte(`{
		"tier":"pro",
		"feature":"AdvancedRisk",
		"event":"opened",
		"usage_key":"risk_report_opened",
		"scene":"advanced_risk_report"
	}`)
	rrSeed := httptest.NewRecorder()
	mux.ServeHTTP(rrSeed, httptest.NewRequest(http.MethodPost, "/api/product/usage", bytes.NewReader(seed)))
	require.Equal(t, http.StatusOK, rrSeed.Code)

	rrSum := httptest.NewRecorder()
	mux.ServeHTTP(rrSum, httptest.NewRequest(http.MethodGet, "/api/product/usage/summary?tier=pro", nil))
	require.Equal(t, http.StatusOK, rrSum.Code)
	var sumBody map[string]any
	require.NoError(t, json.Unmarshal(rrSum.Body.Bytes(), &sumBody))
	require.True(t, sumBody["ok"].(bool))
	require.Equal(t, "shell:pro", sumBody["user_id"])
	require.NotNil(t, sumBody["summary"])

	rrCnt := httptest.NewRecorder()
	mux.ServeHTTP(rrCnt, httptest.NewRequest(http.MethodGet, "/api/product/usage/count?feature=AdvancedRisk&event_type=opened", nil))
	require.Equal(t, http.StatusOK, rrCnt.Code)
	var cntBody map[string]any
	require.NoError(t, json.Unmarshal(rrCnt.Body.Bytes(), &cntBody))
	require.True(t, cntBody["ok"].(bool))
	require.GreaterOrEqual(t, int(cntBody["count"].(float64)), 1)

	rrAn := httptest.NewRecorder()
	mux.ServeHTTP(rrAn, httptest.NewRequest(http.MethodGet, "/api/product/usage/analytics", nil))
	require.Equal(t, http.StatusOK, rrAn.Code)
	var anBody map[string]any
	require.NoError(t, json.Unmarshal(rrAn.Body.Bytes(), &anBody))
	require.True(t, anBody["ok"].(bool))
	feats := anBody["commercial_features"].([]any)
	require.Contains(t, feats, "MultiAccount")
	require.Contains(t, feats, "AIAnalysis")
}

func TestProductCapabilities_UsageSummary_RequiresUser(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/product/usage/summary", nil))
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestProductCapabilities_PlansAndComparison(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterProductCapabilitiesRoutes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/product/plans?tier=free", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var plansBody map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &plansBody))
	require.True(t, plansBody["ok"].(bool))
	require.Equal(t, false, plansBody["payment_enabled"])
	plans := plansBody["plans"].([]any)
	require.Len(t, plans, 3)
	codes := map[string]bool{}
	for _, raw := range plans {
		p := raw.(map[string]any)
		codes[p["code"].(string)] = true
	}
	require.True(t, codes["free"] && codes["pro"] && codes["enterprise"])

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/product/feature-comparison?tier=free", nil))
	require.Equal(t, http.StatusOK, rr2.Code)
	var cmpBody map[string]any
	require.NoError(t, json.Unmarshal(rr2.Body.Bytes(), &cmpBody))
	require.True(t, cmpBody["ok"].(bool))
	rows := cmpBody["rows"].([]any)
	require.GreaterOrEqual(t, len(rows), 6)

	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodPost, "/api/product/upgrade-demo",
		bytes.NewReader([]byte(`{"target":"pro"}`))))
	require.Equal(t, http.StatusOK, rr3.Code)
	var upBody map[string]any
	require.NoError(t, json.Unmarshal(rr3.Body.Bytes(), &upBody))
	require.True(t, upBody["ok"].(bool))
	require.True(t, upBody["accepted"].(bool))
	require.Equal(t, false, upBody["payment_enabled"])

	rr4 := httptest.NewRecorder()
	mux.ServeHTTP(rr4, httptest.NewRequest(http.MethodPost, "/api/product/upgrade-demo",
		bytes.NewReader([]byte(`{"target":"enterprise"}`))))
	require.Equal(t, http.StatusOK, rr4.Code)
	var entBody map[string]any
	require.NoError(t, json.Unmarshal(rr4.Body.Bytes(), &entBody))
	require.True(t, entBody["ok"].(bool))
	require.Equal(t, false, entBody["accepted"])
}
