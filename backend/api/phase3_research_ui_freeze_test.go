package api_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR3-B：CandidatePool UI 必须走 ResearchTradeIntent API，禁止旁路 SubmitPaperOrder。

func TestPhase3_CandidatePoolUIUsesResearchTradeIntentAPI(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	vuePath := filepath.Join(root, "frontend", "src", "components", "CandidatePool.vue")
	vueBytes, err := os.ReadFile(vuePath)
	require.NoError(t, err)
	vue := string(vueBytes)

	require.NotContains(t, vue, "SubmitPaperOrder")
	require.NotContains(t, vue, "PaperTrading")
	require.NotContains(t, vue, "wailsjs/go/main/App")
	require.Contains(t, vue, "createResearchTradeIntent")
	require.Contains(t, vue, "confirmResearchTradeIntent")
	require.Contains(t, vue, "executeResearchTradeIntent")

	apiPath := filepath.Join(root, "frontend", "src", "api", "researchTrade.ts")
	apiBytes, err := os.ReadFile(apiPath)
	require.NoError(t, err)
	apiBody := string(apiBytes)
	require.Contains(t, apiBody, "/api/research_trade/intent")
	require.Contains(t, apiBody, "createResearchTradeIntent")
	require.Contains(t, apiBody, "confirmResearchTradeIntent")
	require.Contains(t, apiBody, "executeResearchTradeIntent")
	require.True(t, strings.Contains(apiBody, "intent/confirm") || strings.Contains(apiBody, "/confirm"))
	require.True(t, strings.Contains(apiBody, "intent/execute") || strings.Contains(apiBody, "/execute"))
}
