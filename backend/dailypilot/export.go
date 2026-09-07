package dailypilot

import (
	"strings"
	"time"
)

// BuildExportReport builds a one-shot Beta observation ticket for local JSON export.
//
// It sets Enabled=true for this assemble only (DefaultEnabled remains false).
// Callers may omit all sources; the report still emits with data_gaps filled.
//
// Never calls Controlled adoption getters/setters, never writes TradePlan,
// never runs Execution / Broker, and never uploads.
func BuildExportReport(in DailyPilotInput) *DailyPilotReport {
	in.Enabled = true
	if strings.TrimSpace(in.TradeDate) == "" {
		in.TradeDate = time.Now().Format("2006-01-02")
	}
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now().UTC()
	}
	return BuildDailyPilotReport(in)
}

// ExportJSON marshals BuildExportReport to indented JSON bytes.
func ExportJSON(in DailyPilotInput) ([]byte, error) {
	return ToJSON(BuildExportReport(in))
}

// FormatDataGapsHint returns a short local-export hint when gaps are present.
func FormatDataGapsHint(gaps []string) string {
	if len(gaps) == 0 {
		return ""
	}
	return "data_gaps:" + strings.Join(gaps, ",")
}
