package portfoliosim

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var forbiddenImports = []string{
	"go-stock/backend/strategy",
	"go-stock/backend/execution",
	"go-stock/backend/models",
	"go-stock/backend/positionsizing",
	"go-stock/backend/papertrading",
	"go-stock/backend/approvegate",
	"go-stock/backend/tradingautomation",
	"go-stock/backend/data",
}

var tradingChainFiles = []string{
	"backend/risk/plan_filter.go",
	"backend/strategy/plan_risk_bridge.go",
	"backend/strategy/build_draft_trade_plan.go",
	"backend/strategy/plan_amount_sizing.go",
	"backend/strategy/morning_position_materialize.go",
	"backend/strategy/freeze_trade_plan.go",
	"backend/strategy/morning_plan_preparation.go",
	"backend/execution/port.go",
	"backend/positionsizing/fixed_amount.go",
}

var forbiddenSourceSnippets = []string{
	"FreezeTradePlan",
	"MaterializeMorningTargetVolumes",
	"ProposeDefault",
	"FixedAmountSizer",
	"ExecutePlanItem",
	"NewTradePlanRepo",
	"INSERT",
	"BuildDraftTradePlan",
	"CreatePlanWithItems",
}

func TestPackage_DoesNotImportTradingChainOrDB(t *testing.T) {
	t.Parallel()
	root := packageDir(t)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, e.Name()))
		require.NoError(t, err)
		text := string(src)
		if !strings.HasSuffix(e.Name(), "_test.go") {
			for _, snip := range forbiddenSourceSnippets {
				require.NotContains(t, text, snip, "%s must not contain %s", e.Name(), snip)
			}
		}
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, path, "%s imports %s", e.Name(), bad)
			}
		}
	}
}

func TestTradingChain_DoesNotImportPortfolioSim(t *testing.T) {
	t.Parallel()
	repo := repoRoot(t)
	for _, rel := range tradingChainFiles {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		b, err := os.ReadFile(p)
		require.NoError(t, err, rel)
		require.NotContains(t, string(b), "go-stock/backend/portfoliosim", "%s must not import portfoliosim", rel)
		require.NotContains(t, string(b), "portfoliosim.", "%s must not call portfoliosim", rel)
	}
}

func packageDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return wd
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Dir(filepath.Dir(wd))
}
