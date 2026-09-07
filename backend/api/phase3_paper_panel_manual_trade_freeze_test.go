package api_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR4-D：PaperTradingPanel 普通单必须走 ManualTrade API，禁止 SubmitPaperOrder。

func TestPhase3_PaperTradingPanelNormalPathUsesManualTradeAPI(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	vuePath := filepath.Join(root, "frontend", "src", "components", "PaperTradingPanel.vue")
	vueBytes, err := os.ReadFile(vuePath)
	require.NoError(t, err)
	vue := string(vueBytes)

	require.NotContains(t, vue, "WailsApp.SubmitPaperOrder")
	require.NotContains(t, vue, "SubmitPaperOrder(")
	require.NotContains(t, vue, "PaperTradingApi")
	require.NotContains(t, vue, ".SubmitPaperOrder")
	require.Contains(t, vue, "createManualTradeIntent")
	require.Contains(t, vue, "confirmManualTradeIntent")
	require.Contains(t, vue, "executeManualTradeIntent")
	// 两融 / 券商确认路径仍允许
	require.Contains(t, vue, "SubmitPaperMarginOrder")
	require.Contains(t, vue, "ConfirmBrokerOrderPlan")
	require.NotContains(t, vue, "候选池模拟买入")
	require.True(t, strings.Contains(vue, "模拟交易") || strings.Contains(vue, "模拟执行台"))

	apiPath := filepath.Join(root, "frontend", "src", "api", "manualTrade.ts")
	apiBytes, err := os.ReadFile(apiPath)
	require.NoError(t, err)
	apiBody := string(apiBytes)
	require.Contains(t, apiBody, "/api/manual_trade/intent")
	require.Contains(t, apiBody, "createManualTradeIntent")
	require.Contains(t, apiBody, "confirmManualTradeIntent")
	require.Contains(t, apiBody, "executeManualTradeIntent")
}
