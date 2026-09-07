package version_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/version"

	"github.com/stretchr/testify/require"
)

func TestParse_OK(t *testing.T) {
	p, err := version.Parse("0.1.0-beta")
	require.NoError(t, err)
	require.Equal(t, 0, p.Major)
	require.Equal(t, 1, p.Minor)
	require.Equal(t, 0, p.Patch)
	require.Equal(t, "beta", p.Pre)

	p2, err := version.Parse("v1.2.3")
	require.NoError(t, err)
	require.Equal(t, 1, p2.Major)
	require.Equal(t, 2, p2.Minor)
	require.Equal(t, 3, p2.Patch)
	require.Empty(t, p2.Pre)
}

func TestParse_Invalid(t *testing.T) {
	_, err := version.Parse("")
	require.Error(t, err)
	_, err = version.Parse("abc")
	require.Error(t, err)
	_, err = version.Parse("1")
	require.Error(t, err)
	_, err = version.Parse("1.2.x")
	require.Error(t, err)
}

func TestCompare_UpdateOrdering(t *testing.T) {
	a, err := version.Parse("0.1.0-beta")
	require.NoError(t, err)
	b, err := version.Parse("0.1.0")
	require.NoError(t, err)
	require.Equal(t, -1, version.Compare(a, b), "pre-release < release")

	c, err := version.Parse("0.1.1-beta")
	require.NoError(t, err)
	require.Equal(t, 1, version.Compare(c, a))

	newer, err := version.IsNewer("0.1.0-beta", "0.1.1-beta")
	require.NoError(t, err)
	require.True(t, newer)

	newer, err = version.IsNewer("0.1.1-beta", "0.1.0-beta")
	require.NoError(t, err)
	require.False(t, newer)
}

func TestUpdateChecker_LocalJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "latest_version": "0.1.1-beta",
  "release_note": "beta fix",
  "download_url": "https://example.invalid/app.zip",
  "channel": "beta"
}`), 0o644))

	chk := &version.UpdateChecker{
		Provider: version.LocalUpdateProvider{Path: path},
		Current: func() version.Info {
			return version.Info{Version: "0.1.0-beta", Channel: "beta"}
		},
	}
	res := chk.CheckUpdate(context.Background())
	require.Empty(t, res.Error)
	require.Equal(t, "0.1.0-beta", res.CurrentVersion)
	require.Equal(t, "0.1.1-beta", res.LatestVersion)
	require.True(t, res.UpdateAvailable)
	require.Equal(t, version.StatusUpdateAvailable, res.Status)
	require.Equal(t, "beta fix", res.ReleaseNote)
	require.Contains(t, res.DownloadURL, "example.invalid")
}

func TestUpdateChecker_UpToDate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "latest_version": "0.1.0-beta",
  "release_note": "same",
  "download_url": "",
  "channel": "beta"
}`), 0o644))
	chk := &version.UpdateChecker{
		Provider: version.LocalUpdateProvider{Path: path},
		Current:  func() version.Info { return version.Info{Version: "0.1.0-beta", Channel: "beta"} },
	}
	res := chk.CheckUpdate(context.Background())
	require.Empty(t, res.Error)
	require.False(t, res.UpdateAvailable)
	require.Equal(t, version.StatusUpToDate, res.Status)
	require.Equal(t, "0.1.0-beta", res.LatestVersion)
}

func TestUpdateChecker_InvalidCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"latest_version":"0.1.1-beta"}`), 0o644))
	chk := &version.UpdateChecker{
		Provider: version.LocalUpdateProvider{Path: path},
		Current:  func() version.Info { return version.Info{Version: "not-a-version"} },
	}
	res := chk.CheckUpdate(context.Background())
	require.NotEmpty(t, res.Error)
	require.False(t, res.UpdateAvailable)
	require.Equal(t, version.StatusError, res.Status)
}

func TestCurrent_Defaults(t *testing.T) {
	info := version.Current()
	require.NotEmpty(t, info.Version)
	require.NotEmpty(t, info.Channel)
	require.Equal(t, info.GitCommit, info.CommitHash)
	require.NotEmpty(t, info.GitCommit)
	require.NotEmpty(t, info.BuildMode)
}

func TestApplyOverrides_AndLogLine(t *testing.T) {
	prevV, prevBT, prevGC, prevCh, prevBM := version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode
	t.Cleanup(func() {
		version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode = prevV, prevBT, prevGC, prevCh, prevBM
	})

	version.ApplyOverrides("9.9.9-beta", "2026-01-02T03:04:05Z", "abc1234", "beta", "production")
	info := version.Current()
	require.Equal(t, "9.9.9-beta", info.Version)
	require.Equal(t, "2026-01-02T03:04:05Z", info.BuildTime)
	require.Equal(t, "abc1234", info.GitCommit)
	require.Equal(t, "abc1234", info.CommitHash)
	require.Equal(t, "beta", info.Channel)
	require.Equal(t, "production", info.BuildMode)
	line := version.LogLine()
	require.Contains(t, line, "version=9.9.9-beta")
	require.Contains(t, line, "git_commit=abc1234")
	require.Contains(t, line, "channel=beta")
	require.Contains(t, line, "build_mode=production")
}

func TestLoadFileConfig_AndBootstrapPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "version": "0.2.0-beta",
  "build_time": "2026-08-22T00:00:00Z",
  "git_commit": "deadbeef",
  "channel": "beta",
  "build_mode": "dev"
}`), 0o644))

	prevV, prevBT, prevGC, prevCh, prevBM := version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode
	t.Cleanup(func() {
		version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode = prevV, prevBT, prevGC, prevCh, prevBM
		version.ResetBootstrapForTest()
	})
	version.ResetBootstrapForTest()
	version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode = "0.1.0-beta", "unknown", "unknown", "beta", "dev"

	used, err := version.TryLoadReleaseFile([]string{path})
	require.NoError(t, err)
	require.Equal(t, path, used)
	info := version.Current()
	require.Equal(t, "0.2.0-beta", info.Version)
	require.Equal(t, "deadbeef", info.GitCommit)
	require.Equal(t, "dev", info.BuildMode)

	// Legacy ldflags override file when non-empty.
	version.ApplyLegacy("0.3.0-beta", "cafebabe")
	info = version.Current()
	require.Equal(t, "0.3.0-beta", info.Version)
	require.Equal(t, "cafebabe", info.GitCommit)
}

func TestLoadFileConfig_EmptyFieldsDoNotClobber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "release.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "version": "0.1.0-beta",
  "build_time": "",
  "git_commit": "",
  "channel": "beta",
  "build_mode": ""
}`), 0o644))

	prevV, prevBT, prevGC, prevCh, prevBM := version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode
	t.Cleanup(func() {
		version.Version, version.BuildTime, version.GitCommit, version.Channel, version.BuildMode = prevV, prevBT, prevGC, prevCh, prevBM
	})
	version.Version = "0.1.0-beta"
	version.BuildTime = "from-ldflags"
	version.GitCommit = "from-ldflags"
	version.Channel = "beta"
	version.BuildMode = "production"

	_, err := version.TryLoadReleaseFile([]string{path})
	require.NoError(t, err)
	info := version.Current()
	require.Equal(t, "from-ldflags", info.BuildTime)
	require.Equal(t, "from-ldflags", info.GitCommit)
	require.Equal(t, "0.1.0-beta", info.Version)
	require.Equal(t, "production", info.BuildMode)
}

func TestWriteCrashReport_ContainsVersionInfo(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	prevV, prevBM := version.Version, version.BuildMode
	t.Cleanup(func() {
		version.Version, version.BuildMode = prevV, prevBM
	})
	version.Version = "0.1.0-beta"
	version.BuildMode = "dev"

	path, err := version.WriteCrashReport("boom-test", "goroutine 1 [running]:\nmain.main()")
	require.NoError(t, err)
	require.FileExists(t, path)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var rep version.CrashReport
	require.NoError(t, json.Unmarshal(raw, &rep))
	require.Equal(t, version.CrashSchemaVersion, rep.SchemaVersion)
	require.Equal(t, "boom-test", rep.Panic)
	require.Equal(t, "0.1.0-beta", rep.VersionInfo.Version)
	require.Equal(t, "dev", rep.VersionInfo.BuildMode)
	require.Contains(t, rep.Note, "local-only")
}

