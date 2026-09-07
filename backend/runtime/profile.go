// Package runtimeutil provides a Runtime Storage Layer (Phase16.16-A/B).
//
// Phase A (Snapshot): observe-only diagnostic.
// Phase B (Init/Get): canonical path resolution anchored to os.Executable(),
//   so db, config and log paths are stable regardless of cwd.
//
// Phase B UserDataDir layout (exe-relative, no AppData migration yet):
//
//	<exeDir>/data/       ← UserDataDir  (database + config JSON files)
//	<exeDir>/logs/       ← LogDir
//	<exeDir>/runtime/    ← RuntimeDir   (profile.json, future migration markers)
package runtimeutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultDBFile is the SQLite filename inside UserDataDir.
const DefaultDBFile = "stock.db"

// DefaultDBRelPath is the legacy relative path used before Phase B.
const DefaultDBRelPath = "data/stock.db"

// ─────────────────────────────────────────────────────────────────────────────
// RuntimeProfile
// ─────────────────────────────────────────────────────────────────────────────

// RuntimeProfile is the canonical, immutable storage layout for this process.
type RuntimeProfile struct {
	Timestamp   time.Time `json:"timestamp"`
	Environment string    `json:"environment"`
	AppVersion  string    `json:"app_version,omitempty"`

	ExecutablePath string `json:"executable_path"`
	ExeDir         string `json:"exe_dir"`

	// UserDataDir is <exeDir>/data in Phase B.
	UserDataDir string `json:"user_data_dir"`
	// DatabasePath is <UserDataDir>/stock.db.
	DatabasePath string `json:"database_path"`
	// ConfigDir is the same as UserDataDir (config JSON files live beside the db).
	ConfigDir string `json:"config_dir"`
	// LogDir is <exeDir>/logs.
	LogDir string `json:"log_dir"`
	// RuntimeDir is <exeDir>/runtime (profile snapshots, migration markers).
	RuntimeDir string `json:"runtime_dir"`

	// Diagnostic fields (Phase A compat, still useful).
	CurrentWorkingDir      string `json:"current_working_dir"`
	ExeDirMatchesCwd       bool   `json:"exe_dir_matches_cwd"`
	CwdMismatchRisk        bool   `json:"cwd_mismatch_risk"`
	DatabaseExistsAtExeDir bool   `json:"database_exists_at_exe_dir"`
}

// LogLines returns human-readable startup log lines.
func (p *RuntimeProfile) LogLines() []string {
	mismatch := ""
	if p.CwdMismatchRisk {
		mismatch = " ⚠ CWD≠ExeDir (resolved from exe)"
	}
	return []string{
		"=== Runtime Storage Profile (Phase16.16-B) ===",
		fmt.Sprintf("  Environment  : %s", p.Environment),
		fmt.Sprintf("  AppVersion   : %s", p.AppVersion),
		fmt.Sprintf("  Executable   : %s", p.ExecutablePath),
		fmt.Sprintf("  ExeDir       : %s", p.ExeDir),
		fmt.Sprintf("  WorkingDir   : %s%s", p.CurrentWorkingDir, mismatch),
		fmt.Sprintf("  UserDataDir  : %s", p.UserDataDir),
		fmt.Sprintf("  DatabasePath : %s  [exists=%v]", p.DatabasePath, p.DatabaseExistsAtExeDir),
		fmt.Sprintf("  ConfigDir    : %s", p.ConfigDir),
		fmt.Sprintf("  LogDir       : %s", p.LogDir),
		fmt.Sprintf("  RuntimeDir   : %s", p.RuntimeDir),
		"===============================================",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Global singleton
// ─────────────────────────────────────────────────────────────────────────────

var (
	globalMu      sync.RWMutex
	globalProfile *RuntimeProfile
)

// Init resolves all paths from os.Executable(), creates required directories,
// persists a profile.json to RuntimeDir, and stores the profile globally.
// Must be called once in main() before db.Init and logger.InitWithDir.
func Init(env, appVersion string) (*RuntimeProfile, error) {
	p, err := buildProfile(env, appVersion)
	if err != nil {
		return nil, err
	}
	if err := ensureDirs(p); err != nil {
		return nil, fmt.Errorf("runtime.Init: mkdir: %w", err)
	}
	writeProfileJSON(p) // best-effort, never fatal

	globalMu.Lock()
	globalProfile = p
	globalMu.Unlock()
	return p, nil
}

// Get returns the initialised RuntimeProfile.
// If Init has not been called (e.g. in unit tests), returns nil.
func Get() *RuntimeProfile {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalProfile
}

// GetDatabasePath returns the absolute path to stock.db.
// Falls back to the legacy relative path when Init has not been called.
func GetDatabasePath() string {
	if p := Get(); p != nil {
		return p.DatabasePath
	}
	return DefaultDBRelPath
}

// GetConfigPath returns the absolute path for a config filename inside ConfigDir.
// Falls back to filepath.Join("data", filename) when Init has not been called,
// preserving backward-compatibility for unit tests that set cwd explicitly.
func GetConfigPath(filename string) string {
	if p := Get(); p != nil {
		return filepath.Join(p.ConfigDir, filename)
	}
	return filepath.Join("data", filename)
}

// GetLogDir returns the absolute path of the log directory.
// Falls back to "./logs" when Init has not been called.
func GetLogDir() string {
	if p := Get(); p != nil {
		return p.LogDir
	}
	return "./logs"
}

// ─────────────────────────────────────────────────────────────────────────────
// Snapshot (Phase A compat — no Init required)
// ─────────────────────────────────────────────────────────────────────────────

// Snapshot captures a diagnostic profile without persisting or storing globally.
// Safe to call before Init.
func Snapshot() (*RuntimeProfile, error) {
	return buildProfile("", "")
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

func buildProfile(env, appVersion string) (*RuntimeProfile, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("runtimeutil: os.Executable: %w", err)
	}
	if resolved, err2 := filepath.EvalSymlinks(exe); err2 == nil {
		exe = resolved
	}
	exe = filepath.Clean(exe)
	exeDir := filepath.Dir(exe)

	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "<unknown>"
	}
	cwd = filepath.Clean(cwd)

	exeDirNorm := filepath.ToSlash(strings.ToLower(exeDir))
	cwdNorm := filepath.ToSlash(strings.ToLower(cwd))
	match := exeDirNorm == cwdNorm

	userDataDir := filepath.Join(exeDir, "data")
	dbPath := filepath.Join(userDataDir, DefaultDBFile)
	logDir := filepath.Join(exeDir, "logs")
	runtimeDir := filepath.Join(exeDir, "runtime")

	if env == "" {
		env = inferEnvironment(exe)
	}

	return &RuntimeProfile{
		Timestamp:              time.Now(),
		Environment:            env,
		AppVersion:             appVersion,
		ExecutablePath:         exe,
		ExeDir:                 exeDir,
		UserDataDir:            userDataDir,
		DatabasePath:           dbPath,
		ConfigDir:              userDataDir,
		LogDir:                 logDir,
		RuntimeDir:             runtimeDir,
		CurrentWorkingDir:      cwd,
		ExeDirMatchesCwd:       match,
		CwdMismatchRisk:        !match,
		DatabaseExistsAtExeDir: fileExists(dbPath),
	}, nil
}

func ensureDirs(p *RuntimeProfile) error {
	for _, dir := range []string{p.UserDataDir, p.LogDir, p.RuntimeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("MkdirAll %s: %w", dir, err)
		}
	}
	return nil
}

func writeProfileJSON(p *RuntimeProfile) {
	path := filepath.Join(p.RuntimeDir, "profile.json")
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func inferEnvironment(exePath string) string {
	base := strings.ToLower(filepath.Base(exePath))
	if idx := strings.LastIndex(base, "."); idx > 0 {
		base = base[:idx]
	}
	switch {
	case strings.HasSuffix(base, "-beta") || strings.Contains(base, "-beta-"):
		return "beta"
	case strings.HasSuffix(base, "-dev") || strings.Contains(base, "-dev-"):
		return "dev"
	default:
		return "production"
	}
}
