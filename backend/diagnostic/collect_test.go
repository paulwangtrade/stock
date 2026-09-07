package diagnostic_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/database"
	"go-stock/backend/diagnostic"

	"github.com/stretchr/testify/require"
)

func TestCollect_BasicFields(t *testing.T) {
	t.Cleanup(database.ResetLastResultForTest)
	t.Cleanup(diagnostic.ResetLastErrorForTest)
	diagnostic.ResetLastErrorForTest()
	database.SetLastResult(database.SafetyResult{
		OK: true, TradingAllowed: true, Integrity: "ok", BackupPath: `D:\stock\data\stock.db.backup.20260810090000`,
	})

	info := diagnostic.Collect("about_export")
	require.Equal(t, diagnostic.SchemaVersion, info.SchemaVersion)
	require.NotEmpty(t, info.DiagnosticID)
	require.NotEmpty(t, info.Timestamp)
	require.NotEmpty(t, info.Version)
	require.NotEmpty(t, info.BuildMode)
	require.Equal(t, info.Version, info.VersionInfo.Version)
	require.Equal(t, info.BuildMode, info.VersionInfo.BuildMode)
	require.Equal(t, "about_export", info.Surface)
	require.Equal(t, "ok", info.Database.Integrity)
	require.Equal(t, "stock.db.backup.20260810090000", info.Database.BackupFileName)
	require.NotContains(t, info.Database.BackupFileName, `\`)
	require.NotNil(t, info.TradingAllowed)
	require.True(t, *info.TradingAllowed)
	require.Contains(t, info.ExportNote, "local-only")
	require.Equal(t, diagnostic.SchemaVersion, info.SchemaVersion)
	require.NotEmpty(t, info.Build.GOOS)
	require.NotEmpty(t, info.Migration.Status)
	require.NotEmpty(t, info.ProviderMode.Adoption)

	raw, err := diagnostic.ToJSON(info)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	blob := strings.ToLower(string(raw))
	require.NotContains(t, blob, "sk-")
	require.NotContains(t, blob, "bearer ")
	require.NotContains(t, blob, "api_key=")
	require.NotContains(t, blob, "password=")
}

func TestCollect_IntegrityFailureCategory(t *testing.T) {
	t.Cleanup(database.ResetLastResultForTest)
	t.Cleanup(diagnostic.ResetLastErrorForTest)
	diagnostic.ResetLastErrorForTest()
	database.SetLastResult(database.SafetyResult{
		OK: false, TradingAllowed: false,
		Error: "database safety: integrity_check failed: *** in database main",
		Integrity: "failed",
		BackupPath: "/tmp/data/stock.db.backup.1",
	})
	info := diagnostic.Collect("test")
	require.Equal(t, diagnostic.CategoryDBIntegrity, info.ErrorCategory)
	require.Equal(t, "INTEGRITY_FAIL", info.ErrorCode)
	require.False(t, info.Database.OK)
	require.Equal(t, "stock.db.backup.1", info.Database.BackupFileName)
}

func TestSanitizeMessage_RedactsSecretsAndPaths(t *testing.T) {
	s := diagnostic.SanitizeMessage(`user password=secret123 path D:\Users\alice\go-stock\data\stock.db`, 200)
	require.Contains(t, s, "[REDACTED]")
	require.NotContains(t, s, "secret123")
	require.NotContains(t, s, `D:\Users\alice`)
}

func TestLogSummary_DropsForbiddenLines(t *testing.T) {
	dir := t.TempDir()
	prev, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	require.NoError(t, os.MkdirAll("logs", 0o755))
	content := "ok line one\napi_key=sk-live-xxx should drop\npassword=hunter2\nnormal runtime info\n"
	require.NoError(t, os.WriteFile(filepath.Join("logs", "error.log"), []byte(content), 0o644))

	info := diagnostic.Collect("test")
	joined := strings.Join(info.LogSummary.ErrorTail, "\n")
	require.NotContains(t, strings.ToLower(joined), "sk-live")
	require.NotContains(t, strings.ToLower(joined), "hunter2")
	require.Contains(t, joined, "ok line one")
}

func TestRecordError_UsedWhenSafetyOK(t *testing.T) {
	t.Cleanup(database.ResetLastResultForTest)
	t.Cleanup(diagnostic.ResetLastErrorForTest)
	database.ResetLastResultForTest()
	diagnostic.ResetLastErrorForTest()
	diagnostic.RecordError(diagnostic.CategoryConfig, "CFG_BAD", "bad config api_key=xxx")
	info := diagnostic.Collect("test")
	require.Equal(t, diagnostic.CategoryConfig, info.ErrorCategory)
	require.Equal(t, "CFG_BAD", info.ErrorCode)
	require.NotContains(t, info.MessageSafe, "xxx")
}
