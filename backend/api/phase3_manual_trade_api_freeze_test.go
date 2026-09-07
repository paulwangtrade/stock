package api_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase3-PR4-C：ManualTrade HTTP API 不得旁路到纸面会计层。

func TestPhase3_ManualTradeAPISourceHasNoPaperTradingBypass(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "manual_trade.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)

	forbidden := []string{
		"SubmitPaperOrder",
		"FillPaperOrder",
		"NewPaperTradingApi",
		"PaperTradingApi",
	}
	for _, needle := range forbidden {
		require.NotContains(t, body, needle,
			"manual_trade.go 不得出现 %q；唯一出口为 ManualTradeFacade", needle)
	}
	require.Contains(t, body, "ConfirmAndExecuteManualTrade")
	require.Contains(t, body, "ManualTradeAssetMiddleware")
}
