package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	runtimeutil "go-stock/backend/runtime"
)

const CrashSchemaVersion = "crash-1"

// CrashReport is a local-only panic ticket (no holdings / secrets / upload).
type CrashReport struct {
	SchemaVersion string      `json:"schema_version"`
	Timestamp     string      `json:"timestamp"`
	Panic         string      `json:"panic"`
	Stack         string      `json:"stack,omitempty"`
	VersionInfo   VersionInfo `json:"version_info"`
	PathHint      string      `json:"path_hint,omitempty"`
	Note          string      `json:"note"`
}

// WriteCrashReport writes data/crash_reports/crash-<utc>.json and returns the path.
// Never panics; returns error only when write fails. Safe for main() recover paths.
func WriteCrashReport(panicValue any, stack string) (string, error) {
	crashDir := runtimeutil.GetConfigPath("crash_reports")
	_ = os.MkdirAll(crashDir, 0o755)
	ts := time.Now().UTC()
	name := fmt.Sprintf("crash-%s.json", ts.Format("20060102T150405Z"))
	path := filepath.Join(crashDir, name)

	rep := CrashReport{
		SchemaVersion: CrashSchemaVersion,
		Timestamp:     ts.Format(time.RFC3339),
		Panic:         sanitizeCrashText(fmt.Sprint(panicValue), 500),
		Stack:         sanitizeCrashText(stack, 8000),
		VersionInfo:   Current(),
		PathHint:      filepath.ToSlash(path),
		Note:          "local-only crash ticket; do not upload; no holdings/API keys; not a trading event",
	}
	raw, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeCrashText(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Drop absolute Windows/Unix paths from stack for privacy (keep basenames loosely).
	s = strings.ReplaceAll(s, "\\", "/")
	if maxRunes > 0 && utf8.RuneCountInString(s) > maxRunes {
		runes := []rune(s)
		s = string(runes[:maxRunes]) + "…"
	}
	return s
}
