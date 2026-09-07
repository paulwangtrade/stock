package api_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR4-E：交易相关 UI 旁路冻结（普通单 / 研究买入）。

func TestPhase3_TradeUIFreeze_NoDirectSubmitPaperOrder(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	candidate := filepath.Join(root, "frontend", "src", "components", "CandidatePool.vue")
	cb, err := os.ReadFile(candidate)
	require.NoError(t, err)
	cbody := string(cb)
	require.NotContains(t, cbody, "SubmitPaperOrder")
	require.NotContains(t, cbody, "PaperTradingApi")
	require.NotContains(t, cbody, "wailsjs/go/main/App")
	require.Contains(t, cbody, "createResearchTradeIntent")
	require.Contains(t, cbody, "confirmResearchTradeIntent")
	require.Contains(t, cbody, "executeResearchTradeIntent")

	panel := filepath.Join(root, "frontend", "src", "components", "PaperTradingPanel.vue")
	pb, err := os.ReadFile(panel)
	require.NoError(t, err)
	pbody := string(pb)
	require.NotContains(t, pbody, "WailsApp.SubmitPaperOrder")
	require.NotContains(t, pbody, "SubmitPaperOrder(")
	require.NotContains(t, pbody, "PaperTradingApi")
	require.Contains(t, pbody, "createManualTradeIntent")
	require.Contains(t, pbody, "confirmManualTradeIntent")
	require.Contains(t, pbody, "executeManualTradeIntent")
	// 两融 / 券商确认仍允许
	require.Contains(t, pbody, "SubmitPaperMarginOrder")
	require.Contains(t, pbody, "ConfirmBrokerOrderPlan")
}

func TestPhase3_IntentModelIsolationFreeze(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	modelsDir := filepath.Join(filepath.Dir(thisFile), "..", "models")

	research, err := os.ReadFile(filepath.Join(modelsDir, "research_trade_intent.go"))
	require.NoError(t, err)
	rbody := string(research)
	for _, need := range []string{"ResearchSource", "CandidateSnapshotID", "SignalScore", "SignalTag"} {
		require.Contains(t, rbody, need)
	}

	manual, err := os.ReadFile(filepath.Join(modelsDir, "manual_trade_intent.go"))
	require.NoError(t, err)
	mbody := string(manual)
	for _, forbid := range []string{
		"ResearchSource", "CandidateSnapshotID", "SignalScore", "SignalTag",
		"research_source", "candidate_snapshot_id", "signal_score", "signal_tag",
	} {
		require.NotContains(t, mbody, forbid)
	}
	require.Contains(t, mbody, "ManualSource")
	require.Contains(t, mbody, "OrderKind")
	require.True(t, strings.Contains(mbody, "Manual execution intent") ||
		strings.Contains(mbody, "模拟盘人工交易意图") ||
		strings.Contains(mbody, "Not a research"))
}
