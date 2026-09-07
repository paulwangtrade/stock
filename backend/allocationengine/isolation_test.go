package allocationengine

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
	"go-stock/backend/positionsizing",
	"go-stock/backend/papertrading",
	"go-stock/backend/decisionprovider",
	"go-stock/backend/portfoliolayer",
}

var forbiddenSourceTokens = []string{
	"CreatePlanWithItems",
	"ExecutePlanItem",
	"ProposeDefault",
	"go-stock/backend/risk",
	"go-stock/backend/execution",
	"go-stock/backend/positionsizing",
}

func TestPackage_DoesNotImportWriteChain(t *testing.T) {
	t.Parallel()
	root := packageDir(t)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(root, e.Name())
		src, err := os.ReadFile(path)
		require.NoError(t, err)
		text := string(src)
		for _, tok := range forbiddenSourceTokens {
			require.NotContains(t, text, tok, "%s must not mention %s", e.Name(), tok)
		}
		f, err := parser.ParseFile(fset, e.Name(), src, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				require.NotEqual(t, bad, p, "%s imports %s", e.Name(), bad)
			}
		}
	}
}

func packageDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return wd
}
