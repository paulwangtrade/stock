package portfoliotarget

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
	"go-stock/backend/data",
	"go-stock/backend/approvegate",
	"go-stock/backend/tradingautomation",
	"go-stock/backend/risk",
	"go-stock/backend/allocationengine",
}

var forbiddenSnippets = []string{
	"CreatePlanWithItems",
	"FreezeTradePlan",
	"ExecutePlanItem",
	"BuildDraftTradePlan",
	"PlanFilter(",
}

func TestIsolation_NoWriteChain(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		require.NoError(t, err)
		text := string(src)
		for _, snip := range forbiddenSnippets {
			require.NotContains(t, text, snip, e.Name())
		}
		file, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range file.Imports {
			pathLit := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, pathLit, e.Name())
			}
		}
	}
}

func TestIsolation_WriteChainDoesNotImport(t *testing.T) {
	targets := []string{
		"../strategy/build_draft_trade_plan.go",
		"../strategy/controlled_draft.go",
		"../execution/port.go",
		"../risk/plan_filter.go",
	}
	for _, rel := range targets {
		path := filepath.Clean(rel)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(src), "portfoliotarget", path)
	}
}
