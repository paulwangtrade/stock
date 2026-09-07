package sellsuggestion_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsolation_NoTradePlanExecutionBroker(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	forbiddenImports := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/execution",
		"go-stock/backend/models",
		"go-stock/backend/data",
		"go-stock/backend/papertrading",
		"go-stock/backend/approvegate",
	}
	forbiddenSrc := []string{
		"CreatePlanWithItems",
		"BuildDraftTSellTradePlan",
		"BuildDraftTradePlanFromCandidatePool",
		"TryBeginExecute",
		"FreezeTradePlan",
		"PaperBroker",
		"RunForPlan",
		"ExecutePlanItem",
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
