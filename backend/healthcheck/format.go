package healthcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// FormatText renders a human-readable report.
func FormatText(r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DB Health Check — %s\n", r.Status)
	fmt.Fprintf(&b, "Checked: %s\n\n", r.CheckedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&b, "[Schema]\n")
	fmt.Fprintf(&b, "  applied=%d required=%d ok=%v\n", r.Schema.AppliedVersion, r.Schema.RequiredVersion, r.Schema.OK)
	if r.Schema.Detail != "" {
		fmt.Fprintf(&b, "  %s\n", r.Schema.Detail)
	}
	fmt.Fprintf(&b, "\n[Tables]\n")
	for _, t := range r.Tables {
		exists := "MISSING"
		if t.Exists {
			exists = fmt.Sprintf("rows=%d", t.RowCount)
		}
		fmt.Fprintf(&b, "  %-24s %-22s %s\n", t.Key, "("+t.PhysicalName+")", exists)
		if t.Note != "" {
			fmt.Fprintf(&b, "    note: %s\n", t.Note)
		}
	}
	fmt.Fprintf(&b, "\n[Consistency]\n")
	for _, c := range r.Checks {
		flag := "PASS"
		if c.Skipped {
			flag = "SKIP"
		} else if !c.OK {
			flag = "FAIL"
		}
		fmt.Fprintf(&b, "  [%s] %s — %s\n", flag, c.Name, c.Detail)
	}
	if len(r.Messages) > 0 {
		fmt.Fprintf(&b, "\n[Summary]\n")
		for _, m := range r.Messages {
			fmt.Fprintf(&b, "  %s\n", m)
		}
	}
	return b.String()
}

// WriteReport writes text report to path (creates/overwrites).
func WriteReport(path string, r Result) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(FormatText(r)), 0o644)
}

// EncodeJSON writes JSON to w.
func EncodeJSON(w io.Writer, r Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
