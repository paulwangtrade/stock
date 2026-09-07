// Package recovery provides Audit Recovery subsystem health checks.
//
// Phase6.5.8.5.1: read-only Recovery Readiness Gate. It never Saves persistent
// checkpoints, never mutates Order / Fill / Position / TradePlan / Frozen Spec,
// and is not wired into trading entry points (Submit / Approve / Freeze / Guard).
package recovery

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/execution/audit"
	"go-stock/backend/models"

	"gorm.io/gorm"
)

// Overall recovery readiness statuses.
const (
	StatusReady    = "READY"
	StatusDegraded = "DEGRADED"
	StatusBlocked  = "BLOCKED"
)

const (
	severityInfo     = "INFO"
	severityWarn     = "WARN"
	severityError    = "ERROR"
	severityCritical = "CRITICAL"
)

// Options configures the read-only evaluator.
type Options struct {
	// SampleSize caps how many intact checkpoints are memory-fold verified (default 3).
	SampleSize int
	// SoftLagEvents emits a WARN when max lag exceeds this (default 1000; 0 disables).
	SoftLagEvents uint
	// SkipReplayVerification skips sample memory folds (schema/checkpoint/continuity only).
	SkipReplayVerification bool
}

// Finding is one readiness finding line.
type Finding struct {
	Code     string         `json:"code"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Scope    string         `json:"scope,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

// CheckpointSample is a compact per-scope checkpoint probe result.
type CheckpointSample struct {
	Scope       string `json:"scope"`
	LastAuditID uint   `json:"lastAuditId"`
	LastEventID string `json:"lastEventId"`
	Version     int    `json:"version"`
	Intact      bool   `json:"intact"`
	LagEvents   uint   `json:"lagEvents"`
	Error       string `json:"error,omitempty"`
}

// CheckpointStatus summarizes persistent checkpoint health.
type CheckpointStatus struct {
	ScopeCount            int                `json:"scopeCount"`
	IntactCount           int                `json:"intactCount"`
	CorruptCount          int                `json:"corruptCount"`
	ContractMismatchCount int                `json:"contractMismatchCount"`
	MissingBaseline       bool               `json:"missingBaseline"`
	MaxLagEvents          uint               `json:"maxLagEvents"`
	Samples               []CheckpointSample `json:"samples,omitempty"`
	StatusHint            string             `json:"statusHint"`
}

// AuditContinuity summarizes audit_events continuity relative to checkpoints.
type AuditContinuity struct {
	MaxAuditID     uint   `json:"maxAuditId"`
	EventCount     int64  `json:"eventCount"`
	HighWatermark  uint   `json:"highWatermark"`
	WindowChecked  bool   `json:"windowChecked"`
	GapCount       int    `json:"gapCount"`
	AnchorMismatch int    `json:"anchorMismatch"`
	StatusHint     string `json:"statusHint"`
}

// ReplayVerification summarizes sample memory-fold results.
type ReplayVerification struct {
	Mode         string `json:"mode"` // none | sample_memory_fold
	SampleSize   int    `json:"sampleSize"`
	PassedCount  int    `json:"passedCount"`
	FailedCount  int    `json:"failedCount"`
	SkippedCount int    `json:"skippedCount"`
	LastError    string `json:"lastError,omitempty"`
	StatusHint   string `json:"statusHint"`
}

// DivergenceItem is a summarized divergence with optional scope.
type DivergenceItem struct {
	Kind      string `json:"kind"`
	Severity  string `json:"severity"`
	Blocking  bool   `json:"blocking"`
	Scope     string `json:"scope,omitempty"`
	EventID   string `json:"eventId,omitempty"`
	EventType string `json:"eventType,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// DivergenceSummary aggregates probe divergences.
type DivergenceSummary struct {
	Total         int              `json:"total"`
	ByKind        map[string]int   `json:"byKind,omitempty"`
	BySeverity    map[string]int   `json:"bySeverity,omitempty"`
	BlockingCount int              `json:"blockingCount"`
	Top           []DivergenceItem `json:"top,omitempty"`
}

// SchemaHealth covers required recovery tables.
type SchemaHealth struct {
	AuditEventsOK           bool   `json:"auditEventsOk"`
	RecoveryCheckpointsOK   bool   `json:"recoveryCheckpointsOk"`
	Detail                  string `json:"detail,omitempty"`
}

// RecoveryReadinessResult is the Phase6.5.8.5.1 health report.
type RecoveryReadinessResult struct {
	Status             string             `json:"status"`
	CheckedAt          time.Time          `json:"checkedAt"`
	ContractVersion    string             `json:"contractVersion"`
	Schema             SchemaHealth       `json:"schema"`
	CheckpointStatus   CheckpointStatus   `json:"checkpoint_status"`
	AuditContinuity    AuditContinuity    `json:"audit_continuity"`
	ReplayVerification ReplayVerification `json:"replay_verification"`
	DivergenceSummary  DivergenceSummary  `json:"divergence_summary"`
	Blockers           []Finding          `json:"blockers,omitempty"`
	Warnings           []Finding          `json:"warnings,omitempty"`
	Notes              []string           `json:"notes,omitempty"`
}

// EvaluateRecoveryReadiness probes audit recovery health. Read-only: never calls
// PersistentCheckpointStore.Save and never mutates business tables.
func EvaluateRecoveryReadiness(database *gorm.DB, opts *Options) RecoveryReadinessResult {
	now := time.Now().UTC()
	if opts == nil {
		opts = &Options{}
	}
	if opts.SampleSize <= 0 {
		opts.SampleSize = 3
	}
	if opts.SoftLagEvents == 0 {
		opts.SoftLagEvents = 1000
	}

	out := RecoveryReadinessResult{
		CheckedAt:       now,
		ContractVersion: audit.ReplayContractVersion,
		Blockers:        make([]Finding, 0),
		Warnings:        make([]Finding, 0),
		Notes:           make([]string, 0),
		DivergenceSummary: DivergenceSummary{
			ByKind:     make(map[string]int),
			BySeverity: make(map[string]int),
			Top:        make([]DivergenceItem, 0),
		},
		CheckpointStatus: CheckpointStatus{
			Samples:    make([]CheckpointSample, 0),
			StatusHint: StatusReady,
		},
		AuditContinuity: AuditContinuity{StatusHint: StatusReady},
		ReplayVerification: ReplayVerification{
			Mode:       "none",
			StatusHint: StatusReady,
		},
	}

	if database == nil {
		out.Blockers = append(out.Blockers, Finding{
			Code: "DB_UNAVAILABLE", Severity: severityCritical,
			Message: "database is not initialized",
		})
		out.Status = StatusBlocked
		out.CheckpointStatus.StatusHint = StatusBlocked
		out.AuditContinuity.StatusHint = StatusBlocked
		out.ReplayVerification.StatusHint = StatusBlocked
		return out
	}

	out.Schema = probeSchema(database)
	if !out.Schema.AuditEventsOK || !out.Schema.RecoveryCheckpointsOK {
		out.Blockers = append(out.Blockers, Finding{
			Code: "SCHEMA_MISSING", Severity: severityCritical,
			Message: out.Schema.Detail,
		})
		addDivergence(&out, DivergenceItem{
			Kind: "schema_missing", Severity: severityCritical, Blocking: true,
			Detail: out.Schema.Detail,
		})
		out.Status = StatusBlocked
		out.CheckpointStatus.StatusHint = StatusBlocked
		out.AuditContinuity.StatusHint = StatusBlocked
		out.ReplayVerification.StatusHint = StatusBlocked
		return out
	}

	highWater, eventCount, err := auditHighWatermark(database)
	if err != nil {
		out.Blockers = append(out.Blockers, Finding{
			Code: "AUDIT_QUERY_FAILED", Severity: severityCritical,
			Message: err.Error(),
		})
		out.Status = StatusBlocked
		return out
	}
	out.AuditContinuity.MaxAuditID = highWater
	out.AuditContinuity.HighWatermark = highWater
	out.AuditContinuity.EventCount = eventCount

	store := audit.NewPersistentCheckpointStore(database)
	rows, err := listCheckpoints(database)
	if err != nil {
		out.Blockers = append(out.Blockers, Finding{
			Code: "CHECKPOINT_QUERY_FAILED", Severity: severityCritical,
			Message: err.Error(),
		})
		out.Status = StatusBlocked
		return out
	}

	probeCheckpoints(&out, store, rows, highWater, opts)
	probeContinuity(&out, database, rows, highWater)
	if !opts.SkipReplayVerification {
		probeReplaySamples(&out, database, store, rows, highWater, opts.SampleSize)
	} else {
		out.Notes = append(out.Notes, "replay verification skipped by options")
		out.ReplayVerification.Mode = "none"
	}

	out.Status = aggregateStatus(out)
	return out
}

func probeSchema(database *gorm.DB) SchemaHealth {
	h := SchemaHealth{}
	h.AuditEventsOK = database.Migrator().HasTable(&models.AuditEvent{})
	h.RecoveryCheckpointsOK = database.Migrator().HasTable(&models.RecoveryCheckpoint{})
	switch {
	case !h.AuditEventsOK && !h.RecoveryCheckpointsOK:
		h.Detail = "missing tables: audit_events, audit_recovery_checkpoints"
	case !h.AuditEventsOK:
		h.Detail = "missing table: audit_events"
	case !h.RecoveryCheckpointsOK:
		h.Detail = "missing table: audit_recovery_checkpoints"
	}
	return h
}

func auditHighWatermark(database *gorm.DB) (uint, int64, error) {
	var count int64
	if err := database.Model(&models.AuditEvent{}).Count(&count).Error; err != nil {
		return 0, 0, err
	}
	if count == 0 {
		return 0, 0, nil
	}
	var maxID uint
	if err := database.Model(&models.AuditEvent{}).Select("MAX(id)").Scan(&maxID).Error; err != nil {
		return 0, count, err
	}
	return maxID, count, nil
}

func listCheckpoints(database *gorm.DB) ([]models.RecoveryCheckpoint, error) {
	var rows []models.RecoveryCheckpoint
	if err := database.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func probeCheckpoints(
	out *RecoveryReadinessResult,
	store *audit.PersistentCheckpointStore,
	rows []models.RecoveryCheckpoint,
	highWater uint,
	opts *Options,
) {
	out.CheckpointStatus.ScopeCount = len(rows)
	if len(rows) == 0 {
		out.CheckpointStatus.MissingBaseline = true
		out.CheckpointStatus.StatusHint = StatusDegraded
		out.Warnings = append(out.Warnings, Finding{
			Code: "NO_CHECKPOINT", Severity: severityWarn,
			Message: "no recovery checkpoints; recovery baseline not established",
		})
		addDivergence(out, DivergenceItem{
			Kind: "no_checkpoint", Severity: severityWarn, Blocking: false,
			Detail: "missing recovery baseline",
		})
		return
	}

	for _, row := range rows {
		sample := CheckpointSample{
			Scope:       row.Scope,
			LastAuditID: row.LastAuditID,
			LastEventID: row.LastEventID,
			Version:     row.Version,
		}
		if highWater > row.LastAuditID {
			sample.LagEvents = highWater - row.LastAuditID
		}
		if sample.LagEvents > out.CheckpointStatus.MaxLagEvents {
			out.CheckpointStatus.MaxLagEvents = sample.LagEvents
		}

		_, err := store.LoadVerified(row.Scope)
		switch {
		case err == nil:
			sample.Intact = true
			out.CheckpointStatus.IntactCount++
		case errors.Is(err, audit.ErrCheckpointCorrupted):
			out.CheckpointStatus.CorruptCount++
			sample.Error = err.Error()
			out.Blockers = append(out.Blockers, Finding{
				Code: "CHECKPOINT_CORRUPT", Severity: severityCritical,
				Message: "checkpoint snapshot integrity failed", Scope: row.Scope,
				Evidence: map[string]any{"error": err.Error()},
			})
			addDivergence(out, DivergenceItem{
				Kind: audit.DivergenceCheckpointCorrupt, Severity: severityCritical,
				Blocking: true, Scope: row.Scope, Detail: err.Error(),
			})
		case errors.Is(err, audit.ErrCheckpointContract):
			out.CheckpointStatus.ContractMismatchCount++
			sample.Error = err.Error()
			out.Blockers = append(out.Blockers, Finding{
				Code: "CHECKPOINT_CONTRACT", Severity: severityCritical,
				Message: "checkpoint replay contract version mismatch", Scope: row.Scope,
				Evidence: map[string]any{"error": err.Error()},
			})
			addDivergence(out, DivergenceItem{
				Kind: "checkpoint_contract_mismatch", Severity: severityCritical,
				Blocking: true, Scope: row.Scope, Detail: err.Error(),
			})
		default:
			sample.Error = err.Error()
			out.Blockers = append(out.Blockers, Finding{
				Code: "CHECKPOINT_LOAD_FAILED", Severity: severityCritical,
				Message: "checkpoint load failed", Scope: row.Scope,
				Evidence: map[string]any{"error": err.Error()},
			})
			addDivergence(out, DivergenceItem{
				Kind: "checkpoint_load_failed", Severity: severityCritical,
				Blocking: true, Scope: row.Scope, Detail: err.Error(),
			})
		}
		out.CheckpointStatus.Samples = append(out.CheckpointStatus.Samples, sample)
	}

	if out.CheckpointStatus.CorruptCount > 0 || out.CheckpointStatus.ContractMismatchCount > 0 {
		out.CheckpointStatus.StatusHint = StatusBlocked
	} else if opts.SoftLagEvents > 0 && out.CheckpointStatus.MaxLagEvents > opts.SoftLagEvents {
		out.CheckpointStatus.StatusHint = StatusDegraded
		out.Warnings = append(out.Warnings, Finding{
			Code: "CHECKPOINT_LAG", Severity: severityWarn,
			Message: fmt.Sprintf("max checkpoint lag %d exceeds soft threshold %d",
				out.CheckpointStatus.MaxLagEvents, opts.SoftLagEvents),
			Evidence: map[string]any{"max_lag": out.CheckpointStatus.MaxLagEvents},
		})
	} else {
		out.CheckpointStatus.StatusHint = StatusReady
	}
}

func probeContinuity(
	out *RecoveryReadinessResult,
	database *gorm.DB,
	rows []models.RecoveryCheckpoint,
	highWater uint,
) {
	if len(rows) == 0 {
		out.AuditContinuity.StatusHint = StatusDegraded
		return
	}
	out.AuditContinuity.WindowChecked = true
	harness := audit.NewRecoveryDrillHarness(database)

	for _, row := range rows {
		// Anchor: last_audit_id must still exist with matching event_id.
		if row.LastAuditID > 0 {
			var anchor models.AuditEvent
			err := database.Where("id = ?", row.LastAuditID).First(&anchor).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				out.AuditContinuity.AnchorMismatch++
				out.AuditContinuity.GapCount++
				out.Blockers = append(out.Blockers, Finding{
					Code: "AUDIT_WINDOW_GAP", Severity: severityError,
					Message: "checkpoint last_audit_id missing from audit_events",
					Scope:   row.Scope,
					Evidence: map[string]any{"last_audit_id": row.LastAuditID},
				})
				addDivergence(out, DivergenceItem{
					Kind: audit.DivergenceAuditWindowGap, Severity: severityError,
					Blocking: true, Scope: row.Scope,
					Detail: fmt.Sprintf("missing anchor audit_events.id=%d", row.LastAuditID),
				})
				continue
			}
			if err != nil {
				out.Blockers = append(out.Blockers, Finding{
					Code: "AUDIT_QUERY_FAILED", Severity: severityCritical,
					Message: err.Error(), Scope: row.Scope,
				})
				continue
			}
			if strings.TrimSpace(row.LastEventID) != "" &&
				strings.TrimSpace(anchor.EventID) != strings.TrimSpace(row.LastEventID) {
				out.AuditContinuity.AnchorMismatch++
				out.AuditContinuity.GapCount++
				out.Blockers = append(out.Blockers, Finding{
					Code: "AUDIT_WINDOW_GAP", Severity: severityError,
					Message: "checkpoint last_event_id does not match audit_events row",
					Scope:   row.Scope,
					Evidence: map[string]any{
						"last_audit_id":       row.LastAuditID,
						"checkpoint_event_id": row.LastEventID,
						"audit_event_id":      anchor.EventID,
					},
				})
				addDivergence(out, DivergenceItem{
					Kind: audit.DivergenceAuditWindowGap, Severity: severityError,
					Blocking: true, Scope: row.Scope, EventID: anchor.EventID,
					Detail: "anchor event_id mismatch",
				})
				continue
			}
		}

		if highWater <= row.LastAuditID {
			continue
		}
		scope, ok := parseReplayScope(row.Scope)
		if !ok {
			out.Warnings = append(out.Warnings, Finding{
				Code: "SCOPE_UNPARSED", Severity: severityWarn,
				Message: "cannot parse checkpoint scope for continuity check", Scope: row.Scope,
			})
			continue
		}
		gaps := harness.CheckAuditWindow(scope, row.LastAuditID, highWater)
		// Empty window with highWater > last means no new scoped events — not a gap.
		if len(gaps) == 1 && strings.Contains(gaps[0].Detail, "audit window empty") {
			var count int64
			q := database.Model(&models.AuditEvent{}).Where("id > ? AND id <= ?", row.LastAuditID, highWater)
			q = applyScopeFilter(q, scope)
			_ = q.Count(&count).Error
			if count == 0 {
				continue
			}
		}
		for _, g := range gaps {
			out.AuditContinuity.GapCount++
			out.Blockers = append(out.Blockers, Finding{
				Code: "AUDIT_WINDOW_GAP", Severity: severityError,
				Message: g.Detail, Scope: row.Scope,
			})
			addDivergence(out, DivergenceItem{
				Kind: audit.DivergenceAuditWindowGap, Severity: severityError,
				Blocking: true, Scope: row.Scope, Detail: g.Detail,
			})
		}
	}

	if out.AuditContinuity.GapCount > 0 {
		out.AuditContinuity.StatusHint = StatusBlocked
	} else {
		out.AuditContinuity.StatusHint = StatusReady
	}
}

func probeReplaySamples(
	out *RecoveryReadinessResult,
	database *gorm.DB,
	store *audit.PersistentCheckpointStore,
	rows []models.RecoveryCheckpoint,
	highWater uint,
	sampleSize int,
) {
	out.ReplayVerification.Mode = "sample_memory_fold"
	out.ReplayVerification.SampleSize = sampleSize

	sampled := 0
	for _, row := range rows {
		if sampled >= sampleSize {
			break
		}
		scope, ok := parseReplayScope(row.Scope)
		if !ok || scope.ClientOrderID == "" && scope.BrokerRequestID == "" && scope.LocalOrderID == "" {
			out.ReplayVerification.SkippedCount++
			continue
		}
		cp, err := store.LoadVerified(row.Scope)
		if err != nil {
			out.ReplayVerification.SkippedCount++
			continue
		}
		sampled++

		fullScope := scope
		fullScope.UntilAuditID = highWater
		discardFull := newDiscardingStore()
		full, err := audit.NewReplayer(database).WithCheckpointStore(discardFull).Replay(fullScope)
		if err != nil {
			out.ReplayVerification.FailedCount++
			out.ReplayVerification.LastError = err.Error()
			out.Blockers = append(out.Blockers, Finding{
				Code: "SAMPLE_REPLAY_FAILED", Severity: severityError,
				Message: "full sample replay failed", Scope: row.Scope,
				Evidence: map[string]any{"error": err.Error()},
			})
			addDivergence(out, DivergenceItem{
				Kind: "sample_replay_failed", Severity: severityError,
				Blocking: true, Scope: row.Scope, Detail: err.Error(),
			})
			continue
		}

		incrScope := scope
		incrScope.UntilAuditID = highWater
		discardIncr := newDiscardingStore()
		discardIncr.seed(cp)
		incremental, err := audit.NewReplayer(database).WithCheckpointStore(discardIncr).ReplayIncremental(incrScope)
		if err != nil {
			out.ReplayVerification.FailedCount++
			out.ReplayVerification.LastError = err.Error()
			out.Blockers = append(out.Blockers, Finding{
				Code: "SAMPLE_REPLAY_FAILED", Severity: severityError,
				Message: "incremental sample replay failed", Scope: row.Scope,
				Evidence: map[string]any{"error": err.Error()},
			})
			addDivergence(out, DivergenceItem{
				Kind: "sample_replay_failed", Severity: severityError,
				Blocking: true, Scope: row.Scope, Detail: err.Error(),
			})
			continue
		}

		// Detect spec_hash mismatch inside either fold.
		blockedBySpec := false
		for _, d := range append(append([]audit.Divergence{}, full.Divergences...), incremental.Divergences...) {
			if d.Kind == audit.DivergenceSpecHashMismatch {
				blockedBySpec = true
				out.ReplayVerification.FailedCount++
				out.Blockers = append(out.Blockers, Finding{
					Code: "SPEC_HASH_MISMATCH", Severity: severityCritical,
					Message: "spec_hash mismatch during sample replay", Scope: row.Scope,
					Evidence: map[string]any{"detail": d.Detail, "event_id": d.EventID},
				})
				addDivergence(out, DivergenceItem{
					Kind: audit.DivergenceSpecHashMismatch, Severity: severityCritical,
					Blocking: true, Scope: row.Scope, EventID: d.EventID, EventType: d.EventType, Detail: d.Detail,
				})
				break
			}
		}
		if blockedBySpec {
			continue
		}

		mismatches := audit.CompareReplayEqual(full, incremental)
		if len(mismatches) > 0 {
			out.ReplayVerification.FailedCount++
			out.Blockers = append(out.Blockers, Finding{
				Code: "SAMPLE_REPLAY_MISMATCH", Severity: severityError,
				Message: "sample full replay != incremental restore+tail", Scope: row.Scope,
				Evidence: map[string]any{"detail": mismatches[0].Detail},
			})
			for _, m := range mismatches {
				addDivergence(out, DivergenceItem{
					Kind: m.Kind, Severity: severityError, Blocking: true,
					Scope: row.Scope, Detail: m.Detail,
				})
			}
			continue
		}
		out.ReplayVerification.PassedCount++
	}

	switch {
	case out.ReplayVerification.FailedCount > 0:
		out.ReplayVerification.StatusHint = StatusBlocked
	case out.ReplayVerification.PassedCount == 0 && out.CheckpointStatus.IntactCount > 0:
		out.ReplayVerification.StatusHint = StatusDegraded
		out.Warnings = append(out.Warnings, Finding{
			Code: "SAMPLE_SKIPPED", Severity: severityWarn,
			Message: "no scoped intact checkpoints were sample-verified",
		})
	case out.CheckpointStatus.MissingBaseline:
		out.ReplayVerification.StatusHint = StatusDegraded
	default:
		out.ReplayVerification.StatusHint = StatusReady
	}
}

func aggregateStatus(out RecoveryReadinessResult) string {
	if len(out.Blockers) > 0 {
		return StatusBlocked
	}
	if out.CheckpointStatus.StatusHint == StatusBlocked ||
		out.AuditContinuity.StatusHint == StatusBlocked ||
		out.ReplayVerification.StatusHint == StatusBlocked {
		return StatusBlocked
	}
	if len(out.Warnings) > 0 ||
		out.CheckpointStatus.StatusHint == StatusDegraded ||
		out.AuditContinuity.StatusHint == StatusDegraded ||
		out.ReplayVerification.StatusHint == StatusDegraded {
		return StatusDegraded
	}
	return StatusReady
}

func addDivergence(out *RecoveryReadinessResult, item DivergenceItem) {
	out.DivergenceSummary.Total++
	out.DivergenceSummary.ByKind[item.Kind]++
	out.DivergenceSummary.BySeverity[item.Severity]++
	if item.Blocking {
		out.DivergenceSummary.BlockingCount++
	}
	if len(out.DivergenceSummary.Top) < 20 {
		out.DivergenceSummary.Top = append(out.DivergenceSummary.Top, item)
	}
}

func parseReplayScope(scopeKey string) (audit.ReplayScope, bool) {
	scopeKey = strings.TrimSpace(scopeKey)
	switch {
	case strings.HasPrefix(scopeKey, "client_order_id:"):
		return audit.ReplayScope{ClientOrderID: strings.TrimPrefix(scopeKey, "client_order_id:")}, true
	case strings.HasPrefix(scopeKey, "broker_request_id:"):
		return audit.ReplayScope{BrokerRequestID: strings.TrimPrefix(scopeKey, "broker_request_id:")}, true
	case strings.HasPrefix(scopeKey, "order_id:"):
		return audit.ReplayScope{LocalOrderID: strings.TrimPrefix(scopeKey, "order_id:")}, true
	case scopeKey == "global":
		return audit.ReplayScope{}, true
	default:
		return audit.ReplayScope{}, false
	}
}

func applyScopeFilter(query *gorm.DB, scope audit.ReplayScope) *gorm.DB {
	if v := strings.TrimSpace(scope.ClientOrderID); v != "" {
		query = query.Where("client_order_id = ?", v)
	}
	if v := strings.TrimSpace(scope.BrokerRequestID); v != "" {
		query = query.Where("broker_request_id = ?", v)
	}
	if v := strings.TrimSpace(scope.LocalOrderID); v != "" {
		query = query.Where("order_id = ?", v)
	}
	return query
}

// discardingStore is an in-process CheckpointStore used only for sample folds.
// Save is a no-op so EvaluateRecoveryReadiness never persists checkpoint updates.
type discardingStore struct {
	mu       sync.Mutex
	baseline map[string]audit.Checkpoint
}

func newDiscardingStore() *discardingStore {
	return &discardingStore{baseline: make(map[string]audit.Checkpoint)}
}

func (s *discardingStore) seed(cp audit.Checkpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baseline[cp.Scope] = cp
}

func (s *discardingStore) Load(scope string) (audit.Checkpoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.baseline[scope]
	return cp, ok
}

func (s *discardingStore) Save(cp audit.Checkpoint) error {
	// Intentionally discard — recovery readiness must not advance checkpoints.
	_ = cp
	return nil
}
