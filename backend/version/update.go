package version

import (
	"context"
	"fmt"
	"strings"
)

// Manifest is the static/local update catalog (version.json).
type Manifest struct {
	LatestVersion string `json:"latest_version"`
	ReleaseNote   string `json:"release_note"`
	DownloadURL   string `json:"download_url"`
	Channel       string `json:"channel,omitempty"`
}

// CheckResult is returned by UpdateChecker.CheckUpdate (read-only; no download).
type CheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	// Status is UpToDate | UpdateAvailable | Error (Phase13-V.4).
	Status      string `json:"status"`
	ReleaseNote string `json:"release_note"`
	DownloadURL string `json:"download_url"`
	Channel     string `json:"channel,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Update notification statuses (local manifest compare only).
const (
	StatusUpToDate        = "UpToDate"
	StatusUpdateAvailable = "UpdateAvailable"
	StatusError           = "Error"
)

// UpdateChecker compares current identity to a Provider catalog.
type UpdateChecker struct {
	Provider UpdateProvider
	Current  func() Info
}

// DefaultLocalChecker uses path (e.g. data/version.json or build/version.json).
func DefaultLocalChecker(manifestPath string) *UpdateChecker {
	return &UpdateChecker{
		Provider: LocalUpdateProvider{Path: manifestPath},
		Current:  Current,
	}
}

// CheckUpdate returns whether an update is available. Never downloads or installs.
func (c *UpdateChecker) CheckUpdate(ctx context.Context) CheckResult {
	curFn := c.Current
	if curFn == nil {
		curFn = Current
	}
	cur := curFn()
	out := CheckResult{
		CurrentVersion: cur.Version,
		Channel:        cur.Channel,
		Status:         StatusError,
	}
	if c == nil || c.Provider == nil {
		out.Error = "version: nil update provider"
		return out
	}
	if name := c.Provider.Name(); name != "" {
		out.Provider = name
	}
	m, err := c.Provider.FetchLatest(ctx)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.LatestVersion = m.LatestVersion
	out.ReleaseNote = m.ReleaseNote
	out.DownloadURL = m.DownloadURL
	if m.Channel != "" {
		out.Channel = m.Channel
	}
	newer, err := IsNewer(cur.Version, m.LatestVersion)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.UpdateAvailable = newer
	if newer {
		out.Status = StatusUpdateAvailable
	} else {
		out.Status = StatusUpToDate
	}
	return out
}

// FormatUserMessage returns a short human-readable summary for UI notifications.
func FormatUserMessage(res CheckResult) string {
	if res.Error != "" {
		return fmt.Sprintf("更新检查失败：%s", strings.TrimSpace(res.Error))
	}
	if res.UpdateAvailable {
		note := strings.TrimSpace(res.ReleaseNote)
		if note == "" {
			note = "发现新版本：" + res.LatestVersion
		}
		return note
	}
	return "当前版本无更新"
}
