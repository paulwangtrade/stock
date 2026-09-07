package version

import "strings"

// Package version is the Version Domain: unified release identity.
// It must not import trading or commercial packages.

// Canonical build_mode values.
const (
	BuildModeDev        = "dev"
	BuildModeProduction = "production"
)

// VersionInfo is the unified product identity consumed by About, Diagnostic,
// startup logs, and crash reports.
//
// Fields: version, build_time, git_commit, channel, build_mode.
// commit_hash is a legacy alias of git_commit for older UI/bindings.
type VersionInfo struct {
	Version    string `json:"version"`
	BuildTime  string `json:"build_time"`
	GitCommit  string `json:"git_commit"`
	CommitHash string `json:"commit_hash"` // alias of GitCommit
	Channel    string `json:"channel"`
	BuildMode  string `json:"build_mode"`
}

// Info is a compatibility alias for VersionInfo (existing callers / Wails bindings).
type Info = VersionInfo

// Build-time variables (override via ldflags on this package, optional).
// Legacy main.Version / main.VersionCommit are applied via ApplyLegacy / Bootstrap.
// production builds may also set BuildMode via //go:build production (see buildmode_production.go).
var (
	Version   = "0.1.0-beta"
	BuildTime = "unknown"
	GitCommit = "unknown"
	Channel   = "beta"
	BuildMode = BuildModeDev
)

// Current returns the effective VersionInfo with safe defaults.
func Current() VersionInfo {
	v := strings.TrimSpace(Version)
	bt := strings.TrimSpace(BuildTime)
	gc := strings.TrimSpace(GitCommit)
	ch := strings.TrimSpace(Channel)
	bm := strings.TrimSpace(BuildMode)
	if v == "" {
		v = "0.1.0-beta"
	}
	if bt == "" {
		bt = "unknown"
	}
	if gc == "" {
		gc = "unknown"
	}
	if ch == "" {
		ch = "beta"
	}
	if bm == "" {
		bm = BuildModeDev
	}
	return VersionInfo{
		Version:    v,
		BuildTime:  bt,
		GitCommit:  gc,
		CommitHash: gc,
		Channel:    ch,
		BuildMode:  bm,
	}
}

// ApplyLegacy bridges main.Version / main.VersionCommit when set by older ldflags.
// Non-empty values override the Version Domain vars.
func ApplyLegacy(version, commit string) {
	if v := strings.TrimSpace(version); v != "" {
		Version = v
	}
	if c := strings.TrimSpace(commit); c != "" {
		GitCommit = c
	}
}

// ApplyOverrides sets any non-empty identity fields (used by release.json / tools).
func ApplyOverrides(version, buildTime, gitCommit, channel, buildMode string) {
	if v := strings.TrimSpace(version); v != "" {
		Version = v
	}
	if bt := strings.TrimSpace(buildTime); bt != "" {
		BuildTime = bt
	}
	if gc := strings.TrimSpace(gitCommit); gc != "" {
		GitCommit = gc
	}
	if ch := strings.TrimSpace(channel); ch != "" {
		Channel = ch
	}
	if bm := strings.TrimSpace(buildMode); bm != "" {
		BuildMode = bm
	}
}

// LogLine returns a single-line identity string for startup / diagnostic / crash logs.
func LogLine() string {
	info := Current()
	return "version=" + info.Version +
		" build_time=" + info.BuildTime +
		" git_commit=" + info.GitCommit +
		" channel=" + info.Channel +
		" build_mode=" + info.BuildMode
}
