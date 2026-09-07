package execution

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR4-B：手工交易 façade 不得旁路到 PaperTrading 会计层。

func TestPhase3_ManualTradeFacadeSourceHasNoPaperTradingBypass(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "manual_trade_facade.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)

	forbidden := []string{
		"SubmitPaperOrder",
		"FillPaperOrder",
		"NewPaperTradingApi",
		"PaperTradingApi",
		"research_source",
		"candidate_snapshot",
		"signal_score",
		"signal_tag",
		"BuildTradePlan(",
		"BuildCandidatePool(",
	}
	for _, needle := range forbidden {
		require.NotContains(t, body, needle,
			"manual_trade_facade.go 不得出现 %q", needle)
	}
	require.Contains(t, body, "ExecutePlanItem(")
	require.Contains(t, body, "ErrManualTradeIntentNotExecutable")
	require.Contains(t, body, "ManualTradeStrategyTag")
}
