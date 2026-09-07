package registry_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase3-B Freeze：registry 不得耦合 TradePlan / Execution；不得改 Candidate Score/Rank 公式。
func TestRegistryPackage_NoTradePlanOrExecutionCoupling(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	forbiddenImports := []string{
		"go-stock/backend/execution",
		"go-stock/backend/broker",
		"go-stock/backend/strategy",
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			pathLit := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbiddenImports {
				if pathLit == bad || strings.HasPrefix(pathLit, bad+"/") {
					t.Fatalf("%s imports forbidden %q", name, pathLit)
				}
			}
		}

		// production logic files must not call pool score/rank builders
		if name == "doc.go" {
			continue
		}
		body := string(src)
		for _, needle := range []string{
			"BuildCandidatePool",
			"ComposeCandidateScore",
			"SubmitPaperOrder",
			"BuildTradePlan",
		} {
			if strings.Contains(body, needle) {
				t.Fatalf("%s must not reference %q", name, needle)
			}
		}
	}

	traceSrc, err := os.ReadFile(filepath.Join(dir, "trace.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(traceSrc), "without mutating") {
		t.Fatal("trace.go should document Score/Rank non-mutation")
	}
}
