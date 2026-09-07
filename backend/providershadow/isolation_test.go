package providershadow

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
	"go-stock/backend/data",
	"go-stock/backend/papertrading",
	"go-stock/backend/approvegate",
	"go-stock/backend/tradingautomation",
	"go-stock/backend/portfoliosim",
	"go-stock/backend/portfolioreplay",
}

var forbiddenSource = []string{
	"NewTradePlanRepo",
	"CreatePlanWithItems",
	"FreezeTradePlan",
	"MaterializeMorning",
	"RunMorningPlanPreparation",
	"ProposeDefault",
	"provider_mode",
	"CONTROLLED",
}

func TestPackage_DoesNotTouchTradingWriteChain(t *testing.T) {
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
		for _, bad := range forbiddenSource {
			require.NotContains(t, text, bad, e.Name())
		}
		require.NotContains(t, text, "PlanFilter")
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
