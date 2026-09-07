package version

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// UpdateProvider supplies latest release metadata for update checks.
// Implementations must not contact legacy open-source release endpoints.
type UpdateProvider interface {
	Name() string
	FetchLatest(ctx context.Context) (Manifest, error)
}

// Provider is a legacy alias for UpdateProvider.
type Provider = UpdateProvider

// LocalUpdateProvider reads a static local manifest (e.g. data/version.json).
type LocalUpdateProvider struct {
	Path string
}

// Name implements UpdateProvider.
func (p LocalUpdateProvider) Name() string {
	return "local"
}

// FetchLatest implements UpdateProvider.
func (p LocalUpdateProvider) FetchLatest(ctx context.Context) (Manifest, error) {
	_ = ctx
	path := strings.TrimSpace(p.Path)
	if path == "" {
		return Manifest{}, fmt.Errorf("version: empty local manifest path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("version: read manifest: %w", err)
	}
	m, err := ParseManifestJSON(b)
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifestURL(m.DownloadURL); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// LocalFileProvider is a legacy alias for LocalUpdateProvider.
type LocalFileProvider = LocalUpdateProvider

// OfficialUpdateProvider fetches release metadata from the go-stock official channel.
// When ManifestURL is empty the provider acts as a placeholder and does not perform network I/O.
type OfficialUpdateProvider struct {
	ManifestURL string
	HTTPClient  *http.Client
}

// Name implements UpdateProvider.
func (p OfficialUpdateProvider) Name() string {
	return "official"
}

// FetchLatest implements UpdateProvider.
func (p OfficialUpdateProvider) FetchLatest(ctx context.Context) (Manifest, error) {
	url := strings.TrimSpace(p.ManifestURL)
	if url == "" {
		return Manifest{}, fmt.Errorf("version: official update channel not configured (placeholder)")
	}
	if err := AssertAllowedUpdateURL(url); err != nil {
		return Manifest{}, err
	}
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Manifest{}, fmt.Errorf("version: official update request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("version: official update fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Manifest{}, fmt.Errorf("version: official update HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Manifest{}, fmt.Errorf("version: official update read: %w", err)
	}
	m, err := ParseManifestJSON(body)
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifestURL(m.DownloadURL); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// DefaultUpdateProvider returns the product default: official placeholder (no legacy GitHub).
func DefaultUpdateProvider() UpdateProvider {
	return OfficialUpdateProvider{}
}

// DefaultCheckProviders is the provider chain for background/manual CheckUpdate.
// Local manifests are tried first (Beta/offline); official placeholder is the fallback default.
func DefaultCheckProviders() []UpdateProvider {
	return []UpdateProvider{
		LocalUpdateProvider{Path: "data/version.json"},
		LocalUpdateProvider{Path: "version.json"},
		DefaultUpdateProvider(),
	}
}

// CheckWithProviders tries providers in order; returns the first successful check result.
func CheckWithProviders(ctx context.Context, providers []UpdateProvider) CheckResult {
	if len(providers) == 0 {
		providers = DefaultCheckProviders()
	}
	curFn := Current
	var last CheckResult
	for _, p := range providers {
		if p == nil {
			continue
		}
		chk := &UpdateChecker{Provider: p, Current: curFn}
		res := chk.CheckUpdate(ctx)
		if res.Error == "" {
			res.Provider = p.Name()
			return res
		}
		last = res
		last.Provider = p.Name()
	}
	if last.Error == "" {
		last.Error = "version: no update provider succeeded"
		last.Status = StatusError
	}
	return last
}

// ForbiddenUpdateHostFragments must never appear in update manifest or download URLs.
var ForbiddenUpdateHostFragments = []string{
	"ArvinLovegood",
	"github.com/ArvinLovegood",
	"api.github.com/repos/ArvinLovegood",
	"gitproxy.click/https://github.com/ArvinLovegood",
}

// AssertAllowedUpdateURL rejects legacy open-source release endpoints.
func AssertAllowedUpdateURL(raw string) error {
	u := strings.TrimSpace(raw)
	if u == "" {
		return nil
	}
	for _, frag := range ForbiddenUpdateHostFragments {
		if strings.Contains(u, frag) {
			return fmt.Errorf("version: forbidden legacy update URL")
		}
	}
	return nil
}

// ValidateManifestURL validates an optional download URL inside a manifest.
func ValidateManifestURL(raw string) error {
	return AssertAllowedUpdateURL(raw)
}
