package execution

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR2：研究买入 façade 不得旁路到 PaperTrading 会计层。

func TestPhase3_ResearchTradeFacadeSourceHasNoPaperTradingBypass(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "research_trade_facade.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)

	forbidden := []string{
		"SubmitPaperOrder(",
		"FillPaperOrder(",
		"NewPaperTradingApi(",
		"BuildTradePlan(",
		"BuildCandidatePool(",
	}
	for _, needle := range forbidden {
		require.NotContains(t, body, needle,
			"research_trade_facade.go 不得出现 %q；唯一出口为 ExecutionService.ExecutePlanItem", needle)
	}
	require.Contains(t, body, "ExecutePlanItem(")
	require.Contains(t, body, "ErrResearchTradeIntentNotExecutable")
}
