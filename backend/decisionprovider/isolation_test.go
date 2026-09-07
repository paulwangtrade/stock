package decisionprovider

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
	"go-stock/backend/risk",
	"go-stock/backend/portfoliosim",
	"go-stock/backend/portfolioreplay",
	"go-stock/backend/papertrading",
	"go-stock/backend/approvegate",
	"go-stock/backend/tradingautomation",
}

func TestPackage_DoesNotImportTradingChain(t *testing.T) {
	t.Parallel()
	wd, err := os.Getwd()
	require.NoError(t, err)
	entries, err := os.ReadDir(wd)
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(wd, e.Name()))
		require.NoError(t, err)
		text := string(src)
		require.NotContains(t, text, "PlanFilter", e.Name())
		require.NotContains(t, text, "BuildDraftTradePlan", e.Name())
		require.NotContains(t, text, "FreezeTradePlan", e.Name())
		if e.Name() == "legacy.go" || e.Name() == "doc.go" {
			require.NotContains(t, text, "SelectPortfolio", e.Name())
			require.NotContains(t, text, "go-stock/backend/portfoliolayer", e.Name())
		}
		if e.Name() == "portfolio.go" {
			require.NotContains(t, text, "ProposeDefault", e.Name())
			require.NotContains(t, text, "FixedAmountSizer", e.Name())
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
