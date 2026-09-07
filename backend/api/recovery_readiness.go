package api

import (
	"net/http"
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/recovery"

	"gorm.io/gorm"
)

// RecoveryReadinessView is the snake_case HTTP DTO for
// GET /api/recovery/readiness. It intentionally exposes no persistence model.
type RecoveryReadinessView struct {
	Status             string                         `json:"status"`
	CheckpointStatus   RecoveryCheckpointStatusView   `json:"checkpoint_status"`
	AuditContinuity    RecoveryAuditContinuityView    `json:"audit_continuity"`
	ReplayVerification RecoveryReplayVerificationView `json:"replay_verification"`
	DivergenceSummary  RecoveryDivergenceSummaryView  `json:"divergence_summary"`
}

type RecoveryCheckpointStatusView struct {
	ScopeCount            int                            `json:"scope_count"`
	IntactCount           int                            `json:"intact_count"`
	CorruptCount          int                            `json:"corrupt_count"`
	ContractMismatchCount int                            `json:"contract_mismatch_count"`
	MissingBaseline       bool                           `json:"missing_baseline"`
	MaxLagEvents          uint                           `json:"max_lag_events"`
	Samples               []RecoveryCheckpointSampleView `json:"samples"`
	StatusHint            string                         `json:"status_hint"`
}

type RecoveryCheckpointSampleView struct {
	Scope       string `json:"scope"`
	LastAuditID uint   `json:"last_audit_id"`
	LastEventID string `json:"last_event_id"`
	Version     int    `json:"version"`
	Intact      bool   `json:"intact"`
	LagEvents   uint   `json:"lag_events"`
	Error       string `json:"error,omitempty"`
}

type RecoveryAuditContinuityView struct {
	MaxAuditID     uint   `json:"max_audit_id"`
	EventCount     int64  `json:"event_count"`
	HighWatermark  uint   `json:"high_watermark"`
	WindowChecked  bool   `json:"window_checked"`
	GapCount       int    `json:"gap_count"`
	AnchorMismatch int    `json:"anchor_mismatch"`
	StatusHint     string `json:"status_hint"`
}

type RecoveryReplayVerificationView struct {
	Mode         string `json:"mode"`
	SampleSize   int    `json:"sample_size"`
	PassedCount  int    `json:"passed_count"`
	FailedCount  int    `json:"failed_count"`
	SkippedCount int    `json:"skipped_count"`
	LastError    string `json:"last_error,omitempty"`
	StatusHint   string `json:"status_hint"`
}

type RecoveryDivergenceSummaryView struct {
	Total         int                          `json:"total"`
	ByKind        map[string]int               `json:"by_kind"`
	BySeverity    map[string]int               `json:"by_severity"`
	BlockingCount int                          `json:"blocking_count"`
	Top           []RecoveryDivergenceItemView `json:"top"`
}

type RecoveryDivergenceItemView struct {
	Kind      string `json:"kind"`
	Severity  string `json:"severity"`
	Blocking  bool   `json:"blocking"`
	Scope     string `json:"scope,omitempty"`
	EventID   string `json:"event_id,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type recoveryReadinessEvaluator func(*gorm.DB, *recovery.Options) recovery.RecoveryReadinessResult

// RecoveryReadinessHandler is a read-only visibility adapter. It delegates all
// rules to recovery.EvaluateRecoveryReadiness and only maps the returned DTO.
type RecoveryReadinessHandler struct {
	database *gorm.DB
	evaluate recoveryReadinessEvaluator
}

// NewRecoveryReadinessHandler creates the production handler.
func NewRecoveryReadinessHandler(database *gorm.DB) *RecoveryReadinessHandler {
	if database == nil {
		database = db.Dao
	}
	return &RecoveryReadinessHandler{
		database: database,
		evaluate: recovery.EvaluateRecoveryReadiness,
	}
}

// NewRecoveryReadinessHandlerWithEvaluator creates an injectable handler for
// isolated API tests. Production callers should use NewRecoveryReadinessHandler.
func NewRecoveryReadinessHandlerWithEvaluator(
	database *gorm.DB,
	evaluate func(*gorm.DB, *recovery.Options) recovery.RecoveryReadinessResult,
) *RecoveryReadinessHandler {
	h := NewRecoveryReadinessHandler(database)
	if evaluate != nil {
		h.evaluate = evaluate
	}
	return h
}

// ServeHTTP serves GET /api/recovery/readiness only.
func (h *RecoveryReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewRecoveryReadinessHandler(nil)
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != "/api/recovery/readiness" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if h.evaluate == nil {
		h.evaluate = recovery.EvaluateRecoveryReadiness
	}
	result := h.evaluate(h.database, nil)
	writeJSON(w, http.StatusOK, mapRecoveryReadinessView(result))
}

func mapRecoveryReadinessView(result recovery.RecoveryReadinessResult) RecoveryReadinessView {
	samples := make([]RecoveryCheckpointSampleView, 0, len(result.CheckpointStatus.Samples))
	for _, sample := range result.CheckpointStatus.Samples {
		samples = append(samples, RecoveryCheckpointSampleView{
			Scope:       sample.Scope,
			LastAuditID: sample.LastAuditID,
			LastEventID: sample.LastEventID,
			Version:     sample.Version,
			Intact:      sample.Intact,
			LagEvents:   sample.LagEvents,
			Error:       sample.Error,
		})
	}

	top := make([]RecoveryDivergenceItemView, 0, len(result.DivergenceSummary.Top))
	for _, item := range result.DivergenceSummary.Top {
		top = append(top, RecoveryDivergenceItemView{
			Kind:      item.Kind,
			Severity:  item.Severity,
			Blocking:  item.Blocking,
			Scope:     item.Scope,
			EventID:   item.EventID,
			EventType: item.EventType,
			Detail:    item.Detail,
		})
	}

	byKind := result.DivergenceSummary.ByKind
	if byKind == nil {
		byKind = map[string]int{}
	}
	bySeverity := result.DivergenceSummary.BySeverity
	if bySeverity == nil {
		bySeverity = map[string]int{}
	}

	return RecoveryReadinessView{
		Status: result.Status,
		CheckpointStatus: RecoveryCheckpointStatusView{
			ScopeCount:            result.CheckpointStatus.ScopeCount,
			IntactCount:           result.CheckpointStatus.IntactCount,
			CorruptCount:          result.CheckpointStatus.CorruptCount,
			ContractMismatchCount: result.CheckpointStatus.ContractMismatchCount,
			MissingBaseline:       result.CheckpointStatus.MissingBaseline,
			MaxLagEvents:          result.CheckpointStatus.MaxLagEvents,
			Samples:               samples,
			StatusHint:            result.CheckpointStatus.StatusHint,
		},
		AuditContinuity: RecoveryAuditContinuityView{
			MaxAuditID:     result.AuditContinuity.MaxAuditID,
			EventCount:     result.AuditContinuity.EventCount,
			HighWatermark:  result.AuditContinuity.HighWatermark,
			WindowChecked:  result.AuditContinuity.WindowChecked,
			GapCount:       result.AuditContinuity.GapCount,
			AnchorMismatch: result.AuditContinuity.AnchorMismatch,
			StatusHint:     result.AuditContinuity.StatusHint,
		},
		ReplayVerification: RecoveryReplayVerificationView{
			Mode:         result.ReplayVerification.Mode,
			SampleSize:   result.ReplayVerification.SampleSize,
			PassedCount:  result.ReplayVerification.PassedCount,
			FailedCount:  result.ReplayVerification.FailedCount,
			SkippedCount: result.ReplayVerification.SkippedCount,
			LastError:    result.ReplayVerification.LastError,
			StatusHint:   result.ReplayVerification.StatusHint,
		},
		DivergenceSummary: RecoveryDivergenceSummaryView{
			Total:         result.DivergenceSummary.Total,
			ByKind:        byKind,
			BySeverity:    bySeverity,
			BlockingCount: result.DivergenceSummary.BlockingCount,
			Top:           top,
		},
	}
}

// RegisterRecoveryReadinessRoutes mounts the read-only route (httptest/server).
func RegisterRecoveryReadinessRoutes(mux *http.ServeMux) {
	mux.Handle("/api/recovery/readiness", NewRecoveryReadinessHandler(nil))
}

// RegisterRecoveryReadinessHandler mounts an injected handler for tests.
func RegisterRecoveryReadinessHandler(mux *http.ServeMux, h *RecoveryReadinessHandler) {
	if h == nil {
		h = NewRecoveryReadinessHandler(nil)
	}
	mux.Handle("/api/recovery/readiness", h)
}

// RecoveryReadinessAssetMiddleware mounts the visibility route on Wails.
func RecoveryReadinessAssetMiddleware(next http.Handler) http.Handler {
	h := NewRecoveryReadinessHandler(nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/recovery/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
