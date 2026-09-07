package projection_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsolation_NoStrategyOrExecutionImports(t *testing.T) {
	root := filepath.Join("..", "..")
	forbidden := []string{
		"go-stock/backend/strategy",
		"go-stock/backend/papertrading/broker",
		"go-stock/backend/risk",
		"go-stock/backend/decisionprovider",
	}
	var files []string
	require.NoError(t, filepath.Walk(filepath.Join(root, "opportunity", "projection"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	}))
	require.NotEmpty(t, files)
	fset := token.NewFileSet()
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		require.NoError(t, err, path)
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				require.NotEqual(t, bad, p, "forbidden import %s in %s", bad, path)
			}
		}
	}
}
