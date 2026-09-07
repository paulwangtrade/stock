package draft_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDraftPackage_NoExecutionOrBrokerImports(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(thisFile)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"go-stock/backend/execution",
		"go-stock/backend/broker",
		"PaperBroker",
		"RealBroker",
		"SubmitPaperOrder",
		"BuildTradePlan",
		"CreatePlanWithItems",
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
		if name == "doc.go" {
			continue
		}
		body := string(src)
		for _, needle := range forbidden {
			if strings.Contains(body, needle) {
				t.Fatalf("%s must not reference %q", name, needle)
			}
		}
		f, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(p, "go-stock/backend/execution") || strings.HasPrefix(p, "go-stock/backend/broker") {
				t.Fatalf("%s imports forbidden %s", name, p)
			}
		}
	}
}
