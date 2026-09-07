package authority

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDependencyGuard_ExecutionBrokerMustNotImportDecision(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	// backend/decision/authority → repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))

	violations, err := ScanForbiddenDecisionImports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("forbidden Decision imports:\n%s", joinLines(violations))
	}
}

func joinLines(ss []string) string {
	out := ""
	for _, s := range ss {
		out += "  - " + s + "\n"
	}
	return out
}
