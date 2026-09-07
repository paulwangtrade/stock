package sectorprovider

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
	"go-stock/backend/risk",
	"go-stock/backend/models",
	"go-stock/backend/approvegate",
	"go-stock/backend/tradingautomation",
	"go-stock/backend/allocationengine",
	"go-stock/backend/decisionprovider",
}

var forbiddenSnippets = []string{
	"CreatePlanWithItems",
	"FreezeTradePlan",
	"ExecutePlanItem",
	"BuildDraftTradePlan",
	"PlanFilter(",
}

func TestIsolation_NoTradeChain(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := e.Name()
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		text := string(src)
		for _, snip := range forbiddenSnippets {
			require.NotContains(t, text, snip, path)
		}
		file, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range file.Imports {
			pathLit := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, pathLit, path)
			}
		}
	}
}

func TestIsolation_TradeChainDoesNotImportSectorProvider(t *testing.T) {
	targets := []string{
		"../strategy/build_draft_trade_plan.go",
		"../strategy/controlled_draft.go",
		"../strategy/freeze_trade_plan.go",
		"../risk/plan_filter.go",
		"../execution/port.go",
	}
	for _, rel := range targets {
		path := filepath.Clean(rel)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(src), "sectorprovider", path)
	}
}
