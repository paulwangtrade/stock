// Package diagnostic collects read-only DiagnosticInfo for Beta support (Phase13-V.3).
//
// Forbidden in snapshots: stocks, holdings, fills, passwords, API keys.
// Forbidden: cloud upload; trading writes (Broker / Gateway / Execution / TradePlan).
package diagnostic

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go-stock/backend/database"
	"go-stock/backend/version"
)

const SchemaVersion = "diag-2"

// Error categories (U.7).
const (
	CategoryNone        = "none"
	CategoryDBOpen      = "db_open"
	CategoryDBIntegrity = "db_integrity"
	CategoryDBAccess    = "db_access"
	CategoryPanic       = "panic"
	CategoryRuntime     = "runtime"
	CategoryNetwork     = "network"
	CategoryMarketData  = "marketdata"
	CategoryConfig      = "config"
	CategoryUpdate      = "update"
	CategoryVersion     = "version"
	CategoryUnknown     = "unknown"
)

// Environment is OS identity without full filesystem paths.
type Environment struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	GoOSGoArch  string `json:"goos_goarch"`
	AppCwdHash8 string `json:"app_cwd_hash8,omitempty"`
}

// DatabaseStatus is a privacy-safe view of startup Safety.
type DatabaseStatus struct {
	OK             bool   `json:"ok"`
	Integrity      string `json:"integrity,omitempty"`
	TradingAllowed bool   `json:"trading_allowed"`
	BackupFileName string `json:"backup_file_name,omitempty"` // basename only
	ErrorCode      string `json:"error_code,omitempty"`
	MessageSafe    string `json:"message_safe,omitempty"`
}

// LogSummary is a short, sanitized log tail.
type LogSummary struct {
	InfoTail  []string `json:"info_tail,omitempty"`
	ErrorTail []string `json:"error_tail,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// Info is the exportable diagnostic ticket (diag-2).
type Info struct {
	SchemaVersion       string               `json:"schema_version"`
	DiagnosticID        string               `json:"diagnostic_id"`
	Timestamp           string               `json:"timestamp"`
	Version             string               `json:"version"`
	BuildTime           string               `json:"build_time"`
	GitCommit           string               `json:"git_commit"`
	CommitHash          string               `json:"commit_hash"` // alias of git_commit (diag-1 compat)
	Channel             string               `json:"channel"`
	BuildMode           string               `json:"build_mode"`
	VersionInfo         version.VersionInfo  `json:"version_info"`
	Build               BuildBlock           `json:"build"`
	Migration           MigrationSnapshot    `json:"migration"`
	Environment         Environment          `json:"environment"`
	Database            DatabaseStatus       `json:"database"`
	LastJobs            []JobStatusEntry     `json:"last_jobs,omitempty"`
	TradingStatus       TradingStatusSummary `json:"trading_status"`
	ProviderMode        ProviderModeSnapshot `json:"provider_mode"`
	Errors              []ErrorEntry         `json:"errors,omitempty"`
	ErrorSummary        ErrorSummary         `json:"error_summary"`
	LogSummary          LogSummary           `json:"log_summary"`
	TradingAllowed      *bool                `json:"trading_allowed,omitempty"`
	CrashReportsPresent bool                 `json:"crash_reports_present"`
	CrashReportNames    []string             `json:"crash_report_names,omitempty"`
	ErrorCategory       string               `json:"error_category"`
	ErrorCode           string               `json:"error_code,omitempty"`
	MessageSafe         string               `json:"message_safe"`
	Surface             string               `json:"surface,omitempty"`
	ExportNote          string               `json:"export_note"`
}

type lastError struct {
	Category    string
	Code        string
	MessageSafe string
}

var (
	lastMu sync.RWMutex
	last   lastError
)

// RecordError stores a sanitized last-error hint for the next export (optional callers).
func RecordError(category, code, message string) {
	lastMu.Lock()
	defer lastMu.Unlock()
	last = lastError{
		Category:    strings.TrimSpace(category),
		Code:        strings.TrimSpace(code),
		MessageSafe: SanitizeMessage(message, 200),
	}
}

// ResetLastErrorForTest clears the recorded error (unit tests).
func ResetLastErrorForTest() {
	lastMu.Lock()
	defer lastMu.Unlock()
	last = lastError{}
}

func peekLastError() lastError {
	lastMu.RLock()
	defer lastMu.RUnlock()
	return last
}

// Collect builds a DiagnosticInfo snapshot (read-only, diag-2).
func Collect(surface string) Info {
	return ReportToInfo(CollectReport(surface))
}

// ToJSON returns pretty JSON bytes.
func ToJSON(info Info) ([]byte, error) {
	return json.MarshalIndent(info, "", "  ")
}

func collectEnvironment() Environment {
	cwd, _ := os.Getwd()
	return Environment{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		GoOSGoArch:  runtime.GOOS + "/" + runtime.GOARCH,
		AppCwdHash8: hash8(cwd),
	}
}

func mapDatabase(s database.SafetyResult) DatabaseStatus {
	out := DatabaseStatus{
		OK:             s.OK,
		Integrity:      SanitizeMessage(s.Integrity, 120),
		TradingAllowed: s.OK && s.TradingAllowed,
		BackupFileName: basenameOnly(s.BackupPath),
	}
	if !s.OK {
		out.ErrorCode, out.MessageSafe = safetyErrorFields(s)
	}
	return out
}

func classifySafety(s database.SafetyResult) (category, code, message string) {
	errLower := strings.ToLower(s.Error + " " + s.Integrity)
	switch {
	case strings.Contains(errLower, "malformed") || strings.Contains(errLower, "not a database"):
		return CategoryDBOpen, "SQLITE_MALFORMED", SanitizeMessage(s.Error, 200)
	case strings.Contains(errLower, "integrity"):
		return CategoryDBIntegrity, "INTEGRITY_FAIL", SanitizeMessage(s.Error, 200)
	case strings.Contains(errLower, "access") || strings.Contains(errLower, "permission") || strings.Contains(errLower, "denied"):
		return CategoryDBAccess, "DB_ACCESS", SanitizeMessage(s.Error, 200)
	case s.Error != "":
		return CategoryDBIntegrity, "DB_SAFETY_FAIL", SanitizeMessage(s.Error, 200)
	default:
		return CategoryDBIntegrity, "DB_SAFETY_FAIL", "database safety failed"
	}
}

func safetyErrorFields(s database.SafetyResult) (code, message string) {
	_, code, message = classifySafety(s)
	return code, message
}

func collectLogSummary(maxLines int) LogSummary {
	sum := LogSummary{
		Note: "tails sanitized; absolute paths and secrets redacted",
	}
	sum.InfoTail = readLogTail(filepath.Join("logs", "info.log"), maxLines)
	sum.ErrorTail = readLogTail(filepath.Join("logs", "error.log"), maxLines)
	return sum
}

func readLogTail(path string, maxLines int) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// Cap read size to avoid huge files in memory.
	if len(raw) > 256*1024 {
		raw = raw[len(raw)-256*1024:]
	}
	text := string(raw)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, maxLines)
	for i := len(lines) - 1; i >= 0 && len(out) < maxLines; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		safe := SanitizeMessage(line, 240)
		if safe == "" || looksForbiddenLine(safe) {
			continue
		}
		out = append(out, safe)
	}
	// reverse to chronological
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func looksForbiddenLine(s string) bool {
	l := strings.ToLower(s)
	keys := []string{
		"apikey", "api_key", "api-key", "authorization:", "bearer ",
		"password", "passwd", "secret", "token=", "tushare",
		"sponsorcode", "license_key", "gstk-",
	}
	for _, k := range keys {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}

// SanitizeMessage redacts secrets and paths, truncates length.
func SanitizeMessage(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = redactSecrets(s)
	s = redactPaths(s)
	if maxRunes > 0 {
		s = truncateRunes(s, maxRunes)
	}
	return s
}

func redactSecrets(s string) string {
	lower := strings.ToLower(s)
	// crude token-ish patterns
	for _, needle := range []string{"api_key", "apikey", "api-key", "authorization", "bearer ", "password", "secret"} {
		if i := strings.Index(lower, needle); i >= 0 {
			return s[:i] + needle + "=[REDACTED]"
		}
	}
	return s
}

func redactPaths(s string) string {
	// Replace long absolute-looking segments with hash placeholder.
	parts := strings.Fields(s)
	for i, p := range parts {
		if looksAbsPath(p) {
			parts[i] = "path#" + hash8(p)
		}
	}
	return strings.Join(parts, " ")
}

func looksAbsPath(p string) bool {
	p = strings.Trim(p, `"'`)
	if len(p) < 4 {
		return false
	}
	if strings.HasPrefix(p, "/") && strings.Count(p, "/") >= 2 {
		return true
	}
	// Windows drive
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true
	}
	return false
}

func basenameOnly(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.Base(filepath.Clean(p))
}

func hash8(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:8])
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

func newDiagnosticID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("diag-%d", time.Now().UnixNano())
	}
	return "diag-" + hex.EncodeToString(b[:])
}
