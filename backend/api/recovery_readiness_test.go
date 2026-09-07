package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/recovery"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRecoveryReadinessAPIDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:recovery_readiness_api_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.MigrateAuditEvents(database))
	require.NoError(t, db.MigrateRecoveryCheckpoints(database))
	return database
}

func sampleRecoveryReadinessResult() recovery.RecoveryReadinessResult {
	return recovery.RecoveryReadinessResult{
		Status: recovery.StatusBlocked,
		CheckpointStatus: recovery.CheckpointStatus{
			ScopeCount:            2,
			IntactCount:           1,
			CorruptCount:          1,
			ContractMismatchCount: 0,
			MissingBaseline:       false,
			MaxLagEvents:          8,
			StatusHint:            recovery.StatusBlocked,
			Samples: []recovery.CheckpointSample{{
				Scope:       "client_order_id:cid-api-1",
				LastAuditID: 12,
				LastEventID: "fill_applied:E1",
				Version:     3,
				Intact:      true,
				LagEvents:   8,
			}},
		},
		AuditContinuity: recovery.AuditContinuity{
			MaxAuditID:     20,
			EventCount:     20,
			HighWatermark:  20,
			WindowChecked:  true,
			GapCount:       1,
			AnchorMismatch: 1,
			StatusHint:     recovery.StatusBlocked,
		},
		ReplayVerification: recovery.ReplayVerification{
			Mode:         "sample_memory_fold",
			SampleSize:   1,
			PassedCount:  0,
			FailedCount:  1,
			SkippedCount: 0,
			LastError:    "sample mismatch",
			StatusHint:   recovery.StatusBlocked,
		},
		DivergenceSummary: recovery.DivergenceSummary{
			Total:         1,
			ByKind:        map[string]int{"audit_window_gap": 1},
			BySeverity:    map[string]int{"ERROR": 1},
			BlockingCount: 1,
			Top: []recovery.DivergenceItem{{
				Kind:      "audit_window_gap",
				Severity:  "ERROR",
				Blocking:  true,
				Scope:     "client_order_id:cid-api-1",
				EventID:   "event-1",
				EventType: "FILL_APPLIED",
				Detail:    "missing row",
			}},
		},
	}
}

func TestRecoveryReadinessAPI_GETMapsSnakeCaseDTO(t *testing.T) {
	database := setupRecoveryReadinessAPIDB(t)
	calls := 0
	h := api.NewRecoveryReadinessHandlerWithEvaluator(
		database,
		func(gotDB *gorm.DB, opts *recovery.Options) recovery.RecoveryReadinessResult {
			calls++
			require.Same(t, database, gotDB)
			require.Nil(t, opts)
			return sampleRecoveryReadinessResult()
		},
	)

	mux := http.NewServeMux()
	api.RegisterRecoveryReadinessHandler(mux, h)
	req := httptest.NewRequest(http.MethodGet, "/api/recovery/readiness", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, calls, "handler must delegate exactly once")
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var view api.RecoveryReadinessView
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &view))
	require.Equal(t, recovery.StatusBlocked, view.Status)
	require.Equal(t, 2, view.CheckpointStatus.ScopeCount)
	require.Equal(t, 1, view.CheckpointStatus.CorruptCount)
	require.Len(t, view.CheckpointStatus.Samples, 1)
	require.Equal(t, uint(12), view.CheckpointStatus.Samples[0].LastAuditID)
	require.Equal(t, uint(20), view.AuditContinuity.HighWatermark)
	require.Equal(t, 1, view.AuditContinuity.GapCount)
	require.Equal(t, 1, view.ReplayVerification.FailedCount)
	require.Equal(t, 1, view.DivergenceSummary.BlockingCount)
	require.Equal(t, 1, view.DivergenceSummary.ByKind["audit_window_gap"])
	require.Len(t, view.DivergenceSummary.Top, 1)
	require.Equal(t, "FILL_APPLIED", view.DivergenceSummary.Top[0].EventType)

	body := rec.Body.String()
	for _, key := range []string{
		`"checkpoint_status"`,
		`"audit_continuity"`,
		`"replay_verification"`,
		`"divergence_summary"`,
		`"scope_count"`,
		`"last_audit_id"`,
		`"high_watermark"`,
		`"blocking_count"`,
		`"event_type"`,
	} {
		require.Contains(t, body, key)
	}
	for _, forbidden := range []string{
		"checkpointStatus",
		"auditContinuity",
		"replayVerification",
		"divergenceSummary",
		"snapshotJson",
		"snapshotHash",
		"RecoveryCheckpoint",
	} {
		require.NotContains(t, body, forbidden)
	}
}

func TestRecoveryReadinessAPI_OnlyGET(t *testing.T) {
	calls := 0
	h := api.NewRecoveryReadinessHandlerWithEvaluator(
		nil,
		func(_ *gorm.DB, _ *recovery.Options) recovery.RecoveryReadinessResult {
			calls++
			return sampleRecoveryReadinessResult()
		},
	)
	mux := http.NewServeMux()
	api.RegisterRecoveryReadinessHandler(mux, h)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/recovery/readiness", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code, method)
	}
	require.Zero(t, calls, "non-GET requests must not evaluate readiness")
}

func TestRecoveryReadinessAPI_NotFoundDoesNotEvaluate(t *testing.T) {
	calls := 0
	h := api.NewRecoveryReadinessHandlerWithEvaluator(
		nil,
		func(_ *gorm.DB, _ *recovery.Options) recovery.RecoveryReadinessResult {
			calls++
			return sampleRecoveryReadinessResult()
		},
	)
	req := httptest.NewRequest(http.MethodGet, "/api/recovery/unknown", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Zero(t, calls)
}

func TestRecoveryReadinessAPI_RealServiceNoCheckpointDegraded(t *testing.T) {
	database := setupRecoveryReadinessAPIDB(t)
	h := api.NewRecoveryReadinessHandler(database)
	req := httptest.NewRequest(http.MethodGet, "/api/recovery/readiness/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var view api.RecoveryReadinessView
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &view))
	require.Equal(t, recovery.StatusDegraded, view.Status)
	require.True(t, view.CheckpointStatus.MissingBaseline)
	require.Zero(t, view.CheckpointStatus.ScopeCount)
	require.Equal(t, recovery.StatusDegraded, view.CheckpointStatus.StatusHint)
}

func TestRecoveryReadinessAssetMiddleware_RoutesAndPassesThrough(t *testing.T) {
	database := setupRecoveryReadinessAPIDB(t)
	original := db.Dao
	db.Dao = database
	t.Cleanup(func() { db.Dao = original })

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("next"))
	})
	handler := api.RecoveryReadinessAssetMiddleware(next)

	apiReq := httptest.NewRequest(http.MethodGet, "/api/recovery/readiness", nil)
	apiRec := httptest.NewRecorder()
	handler.ServeHTTP(apiRec, apiReq)
	require.Equal(t, http.StatusOK, apiRec.Code)

	nextReq := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	nextRec := httptest.NewRecorder()
	handler.ServeHTTP(nextRec, nextReq)
	require.Equal(t, http.StatusTeapot, nextRec.Code)
	require.Equal(t, "next", strings.TrimSpace(nextRec.Body.String()))
}
