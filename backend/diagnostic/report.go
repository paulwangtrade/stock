package diagnostic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/database"
	"go-stock/backend/version"
)

const BundleSchemaVersion = "diag-bundle-2"

// Report is the diag-2 diagnostic ticket (superset of legacy Info).
type Report struct {
	SchemaVersion  string               `json:"schema_version"`
	DiagnosticID   string               `json:"diagnostic_id"`
	Timestamp      string               `json:"timestamp"`
	Surface        string               `json:"surface,omitempty"`
	Version        string               `json:"version"`
	BuildTime      string               `json:"build_time"`
	GitCommit      string               `json:"git_commit"`
	CommitHash     string               `json:"commit_hash"`
	Channel        string               `json:"channel"`
	BuildMode      string               `json:"build_mode"`
	VersionInfo    version.VersionInfo  `json:"version_info"`
	Build          BuildBlock           `json:"build"`
	Migration      MigrationSnapshot    `json:"migration"`
	Environment    Environment          `json:"environment"`
	Database       DatabaseStatus       `json:"database"`
	TradingAllowed *bool                `json:"trading_allowed,omitempty"`
	LastJobs       []JobStatusEntry     `json:"last_jobs,omitempty"`
	TradingStatus  TradingStatusSummary `json:"trading_status"`
	ProviderMode   ProviderModeSnapshot `json:"provider_mode"`
	Errors         []ErrorEntry         `json:"errors,omitempty"`
	ErrorSummary   ErrorSummary         `json:"error_summary"`
	LogSummary     LogSummary           `json:"log_summary"`
	CrashPresent   bool                 `json:"crash_reports_present"`
	CrashNames     []string             `json:"crash_report_names,omitempty"`
	ErrorCategory  string               `json:"error_category"`
	ErrorCode      string               `json:"error_code,omitempty"`
	MessageSafe    string               `json:"message_safe"`
	ExportNote     string               `json:"export_note"`
}

// BundleV2 is the DiagnosticBundle v2 manifest (local zip, no cloud).
type BundleV2 struct {
	BundleSchema  string               `json:"bundle_schema"`
	DiagnosticID  string               `json:"diagnostic_id"`
	CreatedAt     string               `json:"created_at"`
	Surface       string               `json:"surface,omitempty"`
	Version       VersionBlock         `json:"version"`
	Build         BuildBlock           `json:"build"`
	Migration     MigrationSnapshot    `json:"migration"`
	Environment   Environment          `json:"environment"`
	JobStatus     []JobStatusEntry     `json:"job_status"`
	TradingStatus TradingStatusSummary `json:"trading_status"`
	ProviderMode  ProviderModeSnapshot `json:"provider_mode"`
	ErrorSummary  ErrorSummary         `json:"error_summary"`
	Database      DatabaseStatus       `json:"database"`
	LogSummary    LogSummary           `json:"log_summary"`
	CrashPresent  bool                 `json:"crash_reports_present"`
	CrashNames    []string             `json:"crash_report_names,omitempty"`
	ExportNote    string               `json:"export_note"`
}

// CollectReport builds a diag-2 snapshot (read-only; no trading writes).
func CollectReport(surface string) Report {
	bridge := version.Current()
	safety := database.LastResult()
	dbStatus := mapDatabase(safety)
	trading := safety.OK && safety.TradingAllowed

	cat := CategoryNone
	code := ""
	msg := "no recent diagnostic error"
	if !safety.OK {
		cat, code, msg = classifySafety(safety)
	}
	if le := peekLastError(); le.Category != "" && le.Category != CategoryNone && safety.OK {
		cat, code, msg = le.Category, le.Code, le.MessageSafe
	}

	env := collectEnvironment()
	jobs := CollectJobStatus()
	tradingSummary := SanitizeTradingStatus(data.GetDailyTradingStatus())
	planMode, planProvider := latestPlanProviderEnums(tradingSummary.TradeDate)
	provider := SnapshotProviderMode(planMode, planProvider)
	errors := collectErrors(cat, code, msg, jobs)
	crashPresent, crashNames := CollectCrashReportNames()

	return Report{
		SchemaVersion:  SchemaVersion,
		DiagnosticID:   newDiagnosticID(),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Surface:        strings.TrimSpace(surface),
		Version:        bridge.Version,
		BuildTime:      bridge.BuildTime,
		GitCommit:      bridge.GitCommit,
		CommitHash:     bridge.GitCommit,
		Channel:        bridge.Channel,
		BuildMode:      bridge.BuildMode,
		VersionInfo:    bridge,
		Build:          collectBuild(bridge, env),
		Migration:      collectMigration(trading),
		Environment:    env,
		Database:       dbStatus,
		TradingAllowed: &trading,
		LastJobs:       jobs,
		TradingStatus:  tradingSummary,
		ProviderMode:   provider,
		Errors:         errors,
		ErrorSummary:   buildErrorSummary(errors, cat, code, msg),
		LogSummary:     collectLogSummary(40),
		CrashPresent:   crashPresent,
		CrashNames:     crashNames,
		ErrorCategory:  cat,
		ErrorCode:      code,
		MessageSafe:    msg,
		ExportNote:     "local-only; no holdings/fills/passwords/API keys/cash/account ids",
	}
}

// BuildBundleV2 assembles DiagnosticBundle v2 from a diagnostic report.
func BuildBundleV2(report Report) BundleV2 {
	return BundleV2{
		BundleSchema:  BundleSchemaVersion,
		DiagnosticID:  report.DiagnosticID,
		CreatedAt:     report.Timestamp,
		Surface:       report.Surface,
		Version:       collectVersionBlock(report.VersionInfo),
		Build:         report.Build,
		Migration:     report.Migration,
		Environment:   report.Environment,
		JobStatus:     report.LastJobs,
		TradingStatus: report.TradingStatus,
		ProviderMode:  report.ProviderMode,
		ErrorSummary:  report.ErrorSummary,
		Database:      report.Database,
		LogSummary:    report.LogSummary,
		CrashPresent:  report.CrashPresent,
		CrashNames:    report.CrashNames,
		ExportNote:    report.ExportNote,
	}
}

func latestPlanProviderEnums(tradeDate string) (providerMode, decisionProvider string) {
	plan, err := data.NewTradePlanRepo().GetLatestByTradeDate(tradeDate)
	if err != nil || plan == nil {
		return "", ""
	}
	return plan.ProviderMode, plan.DecisionProvider
}

// ReportToInfo maps a Report to Info for Wails bindings.
func ReportToInfo(r Report) Info {
	return Info{
		SchemaVersion:       r.SchemaVersion,
		DiagnosticID:        r.DiagnosticID,
		Timestamp:           r.Timestamp,
		Version:             r.Version,
		BuildTime:           r.BuildTime,
		GitCommit:           r.GitCommit,
		CommitHash:          r.CommitHash,
		Channel:             r.Channel,
		BuildMode:           r.BuildMode,
		VersionInfo:         r.VersionInfo,
		Build:               r.Build,
		Migration:           r.Migration,
		Environment:         r.Environment,
		Database:            r.Database,
		LastJobs:            r.LastJobs,
		TradingStatus:       r.TradingStatus,
		ProviderMode:        r.ProviderMode,
		Errors:              r.Errors,
		ErrorSummary:        r.ErrorSummary,
		LogSummary:          r.LogSummary,
		TradingAllowed:      r.TradingAllowed,
		CrashReportsPresent: r.CrashPresent,
		CrashReportNames:    r.CrashNames,
		ErrorCategory:       r.ErrorCategory,
		ErrorCode:           r.ErrorCode,
		MessageSafe:         r.MessageSafe,
		Surface:             r.Surface,
		ExportNote:          r.ExportNote,
	}
}

// ToReportJSON pretty-prints a Report.
func ToReportJSON(r Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// ToBundleJSON pretty-prints BundleV2.
func ToBundleJSON(b BundleV2) ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

// DefaultBundleFilename suggests a zip filename for export.
func DefaultBundleFilename(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		v = "unknown"
	}
	v = strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(v)
	ts := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("go-stock-diag-%s-%s.zip", v, ts)
}
