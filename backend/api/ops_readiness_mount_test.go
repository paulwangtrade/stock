package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/execution/reconcile"
	"go-stock/backend/recovery"

	"github.com/stretchr/testify/require"
)

// productionAssetChain mirrors main.go ChainAssetMiddleware order (first match wins).
func productionAssetChain(next http.Handler) http.Handler {
	return api.ChainAssetMiddleware(
		api.StrategyIntentsAssetMiddleware,
		api.StrategySchemasAssetMiddleware,
		api.ResearchCandidatesAssetMiddleware,
		api.CandidatePoolAssetMiddleware,
		api.RealOrdersAssetMiddleware,
		api.TradePlansAssetMiddleware,
		api.PaperTradingAssetMiddleware,
		api.PortfolioDashboardAssetMiddleware,
		api.TradingDayMonitorAssetMiddleware,
		api.DailyInvestmentSummaryAssetMiddleware,
		api.OpsTradingDayAssetMiddleware,
		api.RecoveryReadinessAssetMiddleware,
		api.BrokerReconcileAssetMiddleware,
		api.ProductCapabilitiesAssetMiddleware,
	)(next)
}

func TestOpsReadinessRoutes_RegisteredInMain(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	mainPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "main.go"))
	b, err := os.ReadFile(mainPath)
	require.NoError(t, err, mainPath)
	body := string(b)
	require.Contains(t, body, "ChainAssetMiddleware")
	require.Contains(t, body, "PortfolioDashboardAssetMiddleware",
		"main.go ChainAssetMiddleware 必须注册 PortfolioDashboardAssetMiddleware")
	require.Contains(t, body, "TradingDayMonitorAssetMiddleware",
		"main.go ChainAssetMiddleware 必须注册 TradingDayMonitorAssetMiddleware")
	require.Contains(t, body, "DailyInvestmentSummaryAssetMiddleware",
		"main.go ChainAssetMiddleware 必须注册 DailyInvestmentSummaryAssetMiddleware")
	require.Contains(t, body, "RecoveryReadinessAssetMiddleware",
		"main.go ChainAssetMiddleware 必须注册 RecoveryReadinessAssetMiddleware")
	require.Contains(t, body, "BrokerReconcileAssetMiddleware",
		"main.go ChainAssetMiddleware 必须注册 BrokerReconcileAssetMiddleware")

	// Order: existing APIs first; recovery/broker after OpsTradingDay (append-only mount).
	opsIdx := strings.Index(body, "OpsTradingDayAssetMiddleware")
	recIdx := strings.Index(body, "RecoveryReadinessAssetMiddleware")
	brkIdx := strings.Index(body, "BrokerReconcileAssetMiddleware")
	require.Greater(t, opsIdx, 0)
	require.Greater(t, recIdx, opsIdx)
	require.Greater(t, brkIdx, recIdx)
}

func TestOpsReadinessRoutes_ChainGET_NotStatic404(t *testing.T) {
	staticHits := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staticHits++
		http.NotFound(w, r) // simulate AssetServer static 404
	})
	h := productionAssetChain(next)

	recReq := httptest.NewRequest(http.MethodGet, "/api/recovery/readiness", nil)
	recRec := httptest.NewRecorder()
	h.ServeHTTP(recRec, recReq)
	require.Equal(t, http.StatusOK, recRec.Code, "recovery readiness must not fall through to static 404")
	require.Equal(t, 0, staticHits)
	var readiness api.RecoveryReadinessView
	require.NoError(t, json.Unmarshal(recRec.Body.Bytes(), &readiness))
	require.NotEmpty(t, readiness.Status)
	require.Contains(t, []string{
		recovery.StatusReady, recovery.StatusDegraded, recovery.StatusBlocked,
	}, readiness.Status)

	brkReq := httptest.NewRequest(http.MethodGet, "/api/execution/broker-reconcile", nil)
	brkRec := httptest.NewRecorder()
	h.ServeHTTP(brkRec, brkReq)
	require.Equal(t, http.StatusOK, brkRec.Code, "broker reconcile must not fall through to static 404")
	require.Equal(t, 0, staticHits)
	var brk reconcile.BrokerReconcileView
	require.NoError(t, json.Unmarshal(brkRec.Body.Bytes(), &brk))
	require.Equal(t, reconcile.ViewStatusReady, brk.Status)

	// Unrelated path still reaches static layer.
	staticReq := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	staticRec := httptest.NewRecorder()
	h.ServeHTTP(staticRec, staticReq)
	require.Equal(t, http.StatusNotFound, staticRec.Code)
	require.Equal(t, 1, staticHits)
}
