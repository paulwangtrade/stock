package diagnostic_test

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/database"
	"go-stock/backend/diagnostic"

	"github.com/stretchr/testify/require"
)

func TestWriteBundleZip_Structure(t *testing.T) {
	t.Cleanup(database.ResetLastResultForTest)
	t.Cleanup(diagnostic.ResetMigrationProviderForTest)
	t.Cleanup(diagnostic.ResetConfigRedactedProviderForTest)
	t.Cleanup(diagnostic.ResetLastErrorForTest)
	t.Cleanup(diagnostic.ResetErrorRingForTest)

	database.SetLastResult(database.SafetyResult{OK: true, TradingAllowed: true, Integrity: "ok"})
	diagnostic.SetMigrationProvider(func() diagnostic.MigrationSnapshot {
		return diagnostic.MigrationSnapshot{
			CurrentVersion:  3,
			RequiredVersion: 3,
			Status:          "READY",
			TradingAllowed:  true,
		}
	})
	diagnostic.SetConfigRedactedProvider(func() string {
		return `{"api_key":"[redacted]","enablePaperOpenBuy":true}`
	})

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bundle.zip")
	require.NoError(t, diagnostic.WriteBundleZipToFile(zipPath, "test_export"))

	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer r.Close()

	names := make(map[string]struct{}, len(r.File))
	for _, f := range r.File {
		names[f.Name] = struct{}{}
	}
	require.Contains(t, names, "bundle.json")
	require.Contains(t, names, "diagnostic.json")
	require.Contains(t, names, "README.txt")
	require.Contains(t, names, "config_redacted.json")

	var bundle diagnostic.BundleV2
	for _, f := range r.File {
		if f.Name != "bundle.json" {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		require.NoError(t, json.NewDecoder(rc).Decode(&bundle))
		_ = rc.Close()
	}
	require.Equal(t, diagnostic.BundleSchemaVersion, bundle.BundleSchema)
	require.NotEmpty(t, bundle.DiagnosticID)
	require.NotEmpty(t, bundle.Version.Version)
	require.Equal(t, "READY", bundle.Migration.Status)
	require.NotEmpty(t, bundle.ProviderMode.Adoption)
	require.Equal(t, "test_export", bundle.Surface)

	raw, err := os.ReadFile(zipPath)
	require.NoError(t, err)
	blob := strings.ToLower(string(raw))
	require.NotContains(t, blob, "account_cash")
	require.NotContains(t, blob, "sk-live")
}

func TestDefaultBundleFilename(t *testing.T) {
	name := diagnostic.DefaultBundleFilename("1.2.3")
	require.True(t, strings.HasPrefix(name, "go-stock-diag-1.2.3-"))
	require.True(t, strings.HasSuffix(name, ".zip"))
}

func TestBuildBundleV2_FromReport(t *testing.T) {
	report := diagnostic.CollectReport("test")
	bundle := diagnostic.BuildBundleV2(report)
	require.Equal(t, report.DiagnosticID, bundle.DiagnosticID)
	require.Equal(t, report.LastJobs, bundle.JobStatus)
	require.Equal(t, report.ErrorSummary.TotalCount, bundle.ErrorSummary.TotalCount)
}
