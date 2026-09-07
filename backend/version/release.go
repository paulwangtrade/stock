package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileConfig is the on-disk unified release identity.
// Path convention: data/release.json (distinct from data/version.json update manifest).
type FileConfig struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	Channel   string `json:"channel"`
	BuildMode string `json:"build_mode"`
}

var (
	bootOnce sync.Once
	bootErr  error
)

// DefaultReleasePaths are tried in order for the unified release config.
func DefaultReleasePaths() []string {
	return []string{
		filepath.Join("data", "release.json"),
		"release.json",
	}
}

// LoadFileConfig reads a release identity JSON file (read-only).
func LoadFileConfig(path string) (FileConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, err
	}
	var cfg FileConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return FileConfig{}, err
	}
	cfg.Version = strings.TrimSpace(cfg.Version)
	cfg.BuildTime = strings.TrimSpace(cfg.BuildTime)
	cfg.GitCommit = strings.TrimSpace(cfg.GitCommit)
	cfg.Channel = strings.TrimSpace(cfg.Channel)
	cfg.BuildMode = strings.TrimSpace(cfg.BuildMode)
	return cfg, nil
}

// ApplyFileConfig writes non-empty file fields into the Version Domain.
func ApplyFileConfig(cfg FileConfig) {
	ApplyOverrides(cfg.Version, cfg.BuildTime, cfg.GitCommit, cfg.Channel, cfg.BuildMode)
}

// TryLoadReleaseFile loads the first readable release.json from candidates.
// Returns the path used, or "" if none found. Malformed JSON returns error.
func TryLoadReleaseFile(paths []string) (used string, err error) {
	if len(paths) == 0 {
		paths = DefaultReleasePaths()
	}
	var lastErr error
	for _, p := range paths {
		cfg, e := LoadFileConfig(p)
		if e != nil {
			if os.IsNotExist(e) {
				continue
			}
			lastErr = e
			continue
		}
		ApplyFileConfig(cfg)
		return p, nil
	}
	return "", lastErr
}

// Bootstrap initializes the unified identity once:
//  1. data/release.json (or ./release.json) — product config
//  2. legacy main.Version / main.VersionCommit (ldflags) — override when non-empty
//
// Safe to call multiple times; only the first call loads the file.
func Bootstrap(legacyVersion, legacyCommit string) error {
	bootOnce.Do(func() {
		_, bootErr = TryLoadReleaseFile(DefaultReleasePaths())
		ApplyLegacy(legacyVersion, legacyCommit)
	})
	return bootErr
}

// ResetBootstrapForTest clears the once-guard (unit tests only).
func ResetBootstrapForTest() {
	bootOnce = sync.Once{}
	bootErr = nil
}
