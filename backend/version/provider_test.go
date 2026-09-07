package version_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/version"

	"github.com/stretchr/testify/require"
)

func TestOfficialUpdateProvider_PlaceholderNoNetwork(t *testing.T) {
	t.Parallel()
	p := version.OfficialUpdateProvider{}
	m, err := p.FetchLatest(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "placeholder")
	require.Empty(t, m.LatestVersion)
	require.Equal(t, "official", p.Name())
}

func TestDefaultUpdateProvider_IsOfficialPlaceholder(t *testing.T) {
	t.Parallel()
	p := version.DefaultUpdateProvider()
	require.Equal(t, "official", p.Name())
	_, err := p.FetchLatest(context.Background())
	require.Error(t, err)
}

func TestLocalUpdateProvider_RejectsLegacyDownloadURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "version.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "latest_version": "0.2.0",
  "release_note": "bad",
  "download_url": "https://github.com/ArvinLovegood/go-stock/releases/download/v0.2.0/go-stock.exe"
}`), 0o644))
	p := version.LocalUpdateProvider{Path: path}
	_, err := p.FetchLatest(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "forbidden legacy")
}

func TestOfficialUpdateProvider_FetchesManifest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "latest_version": "0.2.0",
  "release_note": "official beta",
  "download_url": "https://cdn.example.test/go-stock.exe",
  "channel": "beta"
}`))
	}))
	defer srv.Close()

	p := version.OfficialUpdateProvider{ManifestURL: srv.URL, HTTPClient: srv.Client()}
	m, err := p.FetchLatest(context.Background())
	require.NoError(t, err)
	require.Equal(t, "0.2.0", m.LatestVersion)
	require.Equal(t, "official beta", m.ReleaseNote)
}

func TestCheckWithProviders_LocalBeforeOfficialPlaceholder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "version.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "latest_version": "0.2.0",
  "release_note": "local manifest",
  "download_url": "https://cdn.example.test/go-stock.exe"
}`), 0o644))

	providers := []version.UpdateProvider{
		version.LocalUpdateProvider{Path: path},
		version.DefaultUpdateProvider(),
	}
	res := version.CheckWithProviders(context.Background(), providers)
	require.Empty(t, res.Error)
	require.Equal(t, "local", res.Provider)
	require.True(t, res.UpdateAvailable)
}

func TestAssertAllowedUpdateURL_BlocksLegacyGitHub(t *testing.T) {
	t.Parallel()
	err := version.AssertAllowedUpdateURL("https://api.github.com/repos/ArvinLovegood/go-stock/releases/latest")
	require.Error(t, err)
	require.NoError(t, version.AssertAllowedUpdateURL("https://updates.example.test/manifest.json"))
}

func TestDefaultCheckProviders_LastIsOfficialPlaceholder(t *testing.T) {
	t.Parallel()
	chain := version.DefaultCheckProviders()
	require.GreaterOrEqual(t, len(chain), 1)
	last := chain[len(chain)-1]
	require.Equal(t, "official", last.Name())
	_, err := last.FetchLatest(context.Background())
	require.True(t, strings.Contains(err.Error(), "placeholder"))
}
