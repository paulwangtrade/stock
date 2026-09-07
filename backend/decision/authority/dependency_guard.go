package authority

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ForbiddenDecisionImporters packages that must never import go-stock/backend/decision/*.
var ForbiddenDecisionImporters = []string{
	"backend/execution",
	"backend/broker",
	"backend/strategy/rank", // reserved; may not exist yet
}

const decisionImportPrefix = "go-stock/backend/decision"

// ScanForbiddenDecisionImports walks repo-relative dirs and returns violating import paths.
// root is repository root (contains backend/).
func ScanForbiddenDecisionImports(root string) ([]string, error) {
	var violations []string
	for _, rel := range ForbiddenDecisionImporters {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // package not present yet — still a freeze target
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				base := d.Name()
				if base == "testdata" || base == "vendor" || base == "logs" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// include _test.go — production and tests must not pull Decision into execution path
			imps, err := listImports(path)
			if err != nil {
				return err
			}
			for _, imp := range imps {
				if imp == decisionImportPrefix || strings.HasPrefix(imp, decisionImportPrefix+"/") {
					relPath, _ := filepath.Rel(root, path)
					violations = append(violations, relPath+` imports `+imp)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return violations, nil
}

func listImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		out = append(out, strings.Trim(imp.Path.Value, `"`))
	}
	return out, nil
}
