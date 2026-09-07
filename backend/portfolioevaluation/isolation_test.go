package portfolioevaluation_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsolation_NoTradePlanExecutionOrPnL(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	forbiddenImports := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/models",
		"go-stock/backend/data",
		"go-stock/backend/papertrading",
		"go-stock/backend/approvegate",
		"go-stock/backend/tradingautomation",
	}
	forbiddenSrc := []string{
		"CreatePlanWithItems",
		"BuildDraftTradePlan",
		"FreezeTradePlan",
		"TryBeginExecute",
		"ComputePnL",
		"TotalReturn",
		"sharpe_ratio",
		"SharpeRatio",
		"AutoTune(",
		"gorm.",
	}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, e.Name()))
		require.NoError(t, err, e.Name())
		text := string(src)
		for _, bad := range forbiddenSrc {
			require.NotContains(t, text, bad, e.Name())
		}
		// JSON field / identifier hygiene: forbid return metrics keys, not NotSharpe flags.
		low := strings.ToLower(text)
		require.NotContains(t, low, "sharpe_ratio", e.Name())
		require.NotContains(t, low, "\"pnl\":", e.Name())
		require.NotContains(t, low, "backtest_return", e.Name())
		require.NotContains(t, low, "autotune(", e.Name())

		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err, e.Name())
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p, e.Name())
			}
		}
	}
}

func TestIsolation_ProductionFilesDoNotImportEvaluation(t *testing.T) {
	targets := []string{
		"../strategy/build_draft_trade_plan.go",
		"../strategy/controlled_draft.go",
		"../strategy/freeze_trade_plan.go",
		"../execution/port.go",
	}
	for _, rel := range targets {
		path := filepath.Clean(filepath.Join(".", rel))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(src), "portfolioevaluation", path)
	}
}
