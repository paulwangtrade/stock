// Package database provides startup-level SQLite safety (Phase10-H.3 / DB-1).
// Backup + PRAGMA integrity_check only. No silent repair, no schema changes.
package database

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-stock/backend/security"

	"gorm.io/gorm"
)

const (
	// BackupNamePrefix is used as: <dbBase>.backup.<timestamp>
	// Example: stock.db.backup.20260809113100
	BackupNamePrefix = "backup"
)

// SafetyResult is the outcome of startup database safety checks.
type SafetyResult struct {
	OK             bool   `json:"ok"`
	DatabasePath   string `json:"database_path,omitempty"`
	BackupPath     string `json:"backup_path,omitempty"`
	Integrity      string `json:"integrity,omitempty"` // "ok" or raw pragma output
	Error          string `json:"error,omitempty"`
	RecoveryHint   string `json:"recovery_hint,omitempty"`
	TradingAllowed bool   `json:"trading_allowed"`
}

// DatabaseSafetyService runs backup + integrity_check at startup.
type DatabaseSafetyService struct {
	// Now optional clock for tests.
	Now func() time.Time
	// CopyFile optional override for tests.
	CopyFile func(src, dst string) error
}

// Default returns a production DatabaseSafetyService.
func Default() *DatabaseSafetyService {
	return &DatabaseSafetyService{}
}

func (s *DatabaseSafetyService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *DatabaseSafetyService) copyFile(src, dst string) error {
	if s != nil && s.CopyFile != nil {
		return s.CopyFile(src, dst)
	}
	return copyFileContents(src, dst)
}

// Backup copies the SQLite main file to "<name>.backup.<YYYYMMDDHHMMSS>" beside it.
// Does not modify the source DB. Prefer calling after wal_checkpoint when a live connection exists.
func (s *DatabaseSafetyService) Backup(dbFilePath string) (string, error) {
	dbFilePath = strings.TrimSpace(dbFilePath)
	if dbFilePath == "" {
		return "", fmt.Errorf("database safety: empty database path")
	}
	st, err := os.Stat(dbFilePath)
	if err != nil {
		return "", fmt.Errorf("database safety: backup stat: %w", err)
	}
	if st.IsDir() {
		return "", fmt.Errorf("database safety: path is a directory: %s", dbFilePath)
	}
	ts := s.now().Format("20060102150405")
	dir := filepath.Dir(dbFilePath)
	base := filepath.Base(dbFilePath)
	backupName := fmt.Sprintf("%s.%s.%s", base, BackupNamePrefix, ts)
	backupPath := filepath.Join(dir, backupName)
	if err := s.copyFile(dbFilePath, backupPath); err != nil {
		return "", fmt.Errorf("database safety: backup copy: %w", err)
	}
	// Best-effort companion WAL/SHM copies (no silent failure of main backup).
	_ = s.copyFile(dbFilePath+"-wal", backupPath+"-wal")
	_ = s.copyFile(dbFilePath+"-shm", backupPath+"-shm")
	return backupPath, nil
}

// IntegrityCheck runs PRAGMA integrity_check. Returns nil only when result is exactly "ok".
// Never attempts repair.
func (s *DatabaseSafetyService) IntegrityCheck(gdb *gorm.DB) error {
	if gdb == nil {
		return fmt.Errorf("database safety: integrity_check: nil db")
	}
	var result string
	if err := gdb.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		return fmt.Errorf("database safety: integrity_check query failed: %w", err)
	}
	result = strings.TrimSpace(result)
	if !strings.EqualFold(result, "ok") {
		return fmt.Errorf("database safety: integrity_check failed: %s", result)
	}
	return nil
}

// RunAtStartup performs: optional WAL checkpoint → Backup → IntegrityCheck.
// On failure: returns OK=false with RecoveryHint; does not repair or mutate schema.
func (s *DatabaseSafetyService) RunAtStartup(dbFilePath string, gdb *gorm.DB) SafetyResult {
	out := SafetyResult{
		DatabasePath:   strings.TrimSpace(dbFilePath),
		TradingAllowed: false,
	}
	if out.DatabasePath == "" {
		out.Error = "database safety: empty database path"
		out.RecoveryHint = recoveryHint("", "")
		return out
	}
	if _, err := os.Stat(out.DatabasePath); err != nil {
		// Fresh install: no file yet — treat as pass (Init may create empty DB).
		if os.IsNotExist(err) {
			out.OK = true
			out.Integrity = "skipped_missing_file"
			out.TradingAllowed = true
			return out
		}
		out.Error = fmt.Sprintf("database safety: cannot access db file: %v", err)
		out.Error = security.SanitizeErrorMessage(out.Error)
		out.RecoveryHint = recoveryHint(out.DatabasePath, "")
		return out
	}

	if gdb != nil {
		_ = gdb.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
	}

	backupPath, berr := s.Backup(out.DatabasePath)
	if berr != nil {
		out.Error = security.SanitizeErrorMessage(berr.Error())
		out.RecoveryHint = recoveryHint(out.DatabasePath, "")
		return out
	}
	out.BackupPath = backupPath

	if err := s.IntegrityCheck(gdb); err != nil {
		out.Error = security.SanitizeErrorMessage(err.Error())
		out.Integrity = strings.TrimPrefix(err.Error(), "database safety: integrity_check failed: ")
		out.RecoveryHint = recoveryHint(out.DatabasePath, backupPath)
		return out
	}
	out.OK = true
	out.Integrity = "ok"
	out.TradingAllowed = true
	return out
}

func recoveryHint(dbPath, backupPath string) string {
	var b strings.Builder
	b.WriteString("DATABASE INTEGRITY FAILURE — trading initialization is blocked. ")
	b.WriteString("Do NOT auto-repair. Close the app, restore from a known-good backup, then restart. ")
	if backupPath != "" {
		b.WriteString("Startup backup copy: ")
		b.WriteString(security.PathHint(backupPath))
		b.WriteString(". ")
	}
	if dbPath != "" {
		b.WriteString("Live DB path: ")
		b.WriteString(security.PathHint(dbPath))
		b.WriteString(". ")
	}
	b.WriteString("If no good backup exists, contact support with the backup file and error log.")
	return b.String()
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// --- process-wide last result for TradingPreflight ---

var (
	lastMu     sync.RWMutex
	lastResult = SafetyResult{OK: true, TradingAllowed: true, Integrity: "not_run"}
)

// SetLastResult stores startup safety outcome for preflight gates.
func SetLastResult(r SafetyResult) {
	lastMu.Lock()
	defer lastMu.Unlock()
	lastResult = r
}

// LastResult returns the last startup safety outcome.
func LastResult() SafetyResult {
	lastMu.RLock()
	defer lastMu.RUnlock()
	return lastResult
}

// IsTradingSafe reports whether trading init may proceed (integrity OK).
func IsTradingSafe() bool {
	r := LastResult()
	return r.OK && r.TradingAllowed
}

// ResetLastResultForTest restores permissive default (unit tests).
func ResetLastResultForTest() {
	SetLastResult(SafetyResult{OK: true, TradingAllowed: true, Integrity: "not_run"})
}

// StripDSNPath returns the filesystem path before '?' in a SQLite DSN.
func StripDSNPath(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	if i := strings.Index(dsn, "?"); i >= 0 {
		return dsn[:i]
	}
	return dsn
}
