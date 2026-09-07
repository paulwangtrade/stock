package dailypilot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/dailypilot"

	"github.com/stretchr/testify/require"
)

func TestBuildExportReport_NoDataShowsDataGaps(t *testing.T) {
	require.False(t, dailypilot.DefaultEnabled)

	rep := dailypilot.BuildExportReport(dailypilot.DailyPilotInput{
		TradeDate: "2026-08-23",
	})
	require.NotNil(t, rep)
	require.True(t, rep.Enabled)
	require.False(t, rep.Skipped)
	require.Equal(t, "2026-08-23", rep.TradeDate)
	require.True(t, rep.ReadOnly)
	require.True(t, rep.RecordOnly)
	require.True(t, rep.NotExecution)
	require.True(t, rep.NotProviderSwitch)

	require.NotEmpty(t, rep.DataGaps)
	require.Contains(t, rep.DataGaps, "shadow_records_missing")
	require.Contains(t, rep.DataGaps, "validation_missing")
	require.Contains(t, rep.DataGaps, "controlled_monitor_missing")
	require.Contains(t, rep.DataGaps, "portfolio_history_missing")
	require.Contains(t, rep.DataGaps, "policy_snapshot_missing")

	raw, err := dailypilot.ToJSON(rep)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"data_gaps"`)
	require.Contains(t, string(raw), "shadow_records_missing")
	require.NotContains(t, string(raw), "SetActive")
}

func TestExportJSON_EmptyInput(t *testing.T) {
	raw, err := dailypilot.ExportJSON(dailypilot.DailyPilotInput{TradeDate: "2026-08-23"})
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	gaps, ok := m["data_gaps"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, gaps)

	hint := dailypilot.FormatDataGapsHint([]string{"shadow_records_missing", "validation_missing"})
	require.Equal(t, "data_gaps:shadow_records_missing,validation_missing", hint)
	require.Empty(t, dailypilot.FormatDataGapsHint(nil))
}

func TestExport_IsolationNoControlledActive(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	src, err := os.ReadFile(filepath.Join(wd, "export.go"))
	require.NoError(t, err)
	text := string(src)
	require.NotContains(t, text, "controlledadoption")
	require.NotContains(t, text, "SetActive(")
	require.NotContains(t, text, "Active()")
	require.NotContains(t, text, "go-stock/backend/execution")
	require.NotContains(t, text, "go-stock/backend/strategy")
	require.False(t, strings.Contains(text, "http.Post") || strings.Contains(text, "Upload"))
}
