package recovery_test

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/execution/audit"
	"go-stock/backend/models"
	"go-stock/backend/recovery"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRecoveryReadyDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:recovery_ready_%s?mode=memory&cache=shared", t.Name())
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

func lifecycleEvents(t *testing.T, cid, symbol, req string) (prefix, tail []audit.Event) {
	t.Helper()
	spec := audit.SpecFingerprint(symbol, "buy", 10, 100)
	now := time.Now().UTC()
	prefix = []audit.Event{
		audit.BuildSubmitAttempt(audit.BackendRealStub, req, cid, "1", "1", spec, symbol, "buy", 10, 100, now),
		audit.BuildSubmitResult(
			audit.TypeSubmitAccepted, audit.BackendRealStub, req, cid, "1", "1", spec,
			"accepted", "pending", "accepted", now, now.Add(time.Second), "", "", map[string]any{},
		),
		audit.BuildReportReceived(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-1", "TRADE", req+"-E1", "new", "applied", now.Add(2*time.Second), map[string]any{},
			map[string]any{"last_qty": 40, "last_price": 10.0},
		),
		audit.BuildFillApplied(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-1", req+"-E1", "report_received:"+req+"-tr-1",
			40, 10.0, 40, 10.0, 60, 40, "pending", "partially_filled", now.Add(2*time.Second),
		),
	}
	tail = []audit.Event{
		audit.BuildReportReceived(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-2", "TRADE", req+"-E2", "new", "applied", now.Add(3*time.Second), map[string]any{},
			map[string]any{"last_qty": 60, "last_price": 10.1},
		),
		audit.BuildFillApplied(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			req+"-tr-2", req+"-E2", "report_received:"+req+"-tr-2",
			60, 10.1, 100, 10.06, 0, 60, "filled", "filled", now.Add(3*time.Second),
		),
		audit.BuildOrderTerminal(
			audit.BackendRealStub, req, cid, "1", "1", spec,
			"filled", "filled", 100, 0, 10.06, req+"-tr-2", now.Add(3*time.Second), nil,
		),
	}
	return prefix, tail
}

func seedEvents(t *testing.T, database *gorm.DB, events []audit.Event) {
	t.Helper()
	sink := audit.NewDBAuditSink(database)
	for _, ev := range events {
		require.NoError(t, sink.Write(ev))
	}
}

func createPersistentCheckpoint(t *testing.T, database *gorm.DB, cid string, events []audit.Event) {
	t.Helper()
	seedEvents(t, database, events)
	store := audit.NewPersistentCheckpointStore(database)
	res, err := audit.NewReplayer(database).WithCheckpointStore(store).
		Replay(audit.ReplayScope{ClientOrderID: cid})
	require.NoError(t, err)
	require.NotEmpty(t, res.Checkpoint.SnapshotHash)
}

func TestEvaluateRecoveryReadiness_NoCheckpoint_Degraded(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, _ := lifecycleEvents(t, "cid-ready-0", "sz000901", "req-ready-0")
	seedEvents(t, database, prefix)

	res := recovery.EvaluateRecoveryReadiness(database, nil)
	require.Equal(t, recovery.StatusDegraded, res.Status)
	require.True(t, res.CheckpointStatus.MissingBaseline)
	require.Equal(t, recovery.StatusDegraded, res.CheckpointStatus.StatusHint)
	require.NotEmpty(t, res.Warnings)
	require.Empty(t, res.Blockers)
	require.Equal(t, audit.ReplayContractVersion, res.ContractVersion)
}

func TestEvaluateRecoveryReadiness_Ready(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, tail := lifecycleEvents(t, "cid-ready-1", "sz000902", "req-ready-1")
	createPersistentCheckpoint(t, database, "cid-ready-1", prefix)
	seedEvents(t, database, tail)

	before := checkpointFingerprint(t, database)
	res := recovery.EvaluateRecoveryReadiness(database, &recovery.Options{SampleSize: 1})
	require.Equal(t, recovery.StatusReady, res.Status, "blockers=%+v warnings=%+v div=%+v",
		res.Blockers, res.Warnings, res.DivergenceSummary)
	require.Equal(t, 1, res.CheckpointStatus.IntactCount)
	require.Equal(t, 0, res.AuditContinuity.GapCount)
	require.Equal(t, "sample_memory_fold", res.ReplayVerification.Mode)
	require.Equal(t, 1, res.ReplayVerification.PassedCount)
	require.Equal(t, 0, res.ReplayVerification.FailedCount)
	require.Equal(t, 0, res.DivergenceSummary.BlockingCount)
	require.Equal(t, before, checkpointFingerprint(t, database), "evaluate must not Save/alter checkpoints")
}

func TestEvaluateRecoveryReadiness_Corruption_Blocked(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, _ := lifecycleEvents(t, "cid-ready-2", "sz000903", "req-ready-2")
	createPersistentCheckpoint(t, database, "cid-ready-2", prefix)

	require.NoError(t, database.Model(&models.RecoveryCheckpoint{}).
		Where("scope = ?", "client_order_id:cid-ready-2").
		Update("snapshot_hash", "corrupt-deadbeef").Error)

	res := recovery.EvaluateRecoveryReadiness(database, nil)
	require.Equal(t, recovery.StatusBlocked, res.Status)
	require.Equal(t, 1, res.CheckpointStatus.CorruptCount)
	require.Equal(t, recovery.StatusBlocked, res.CheckpointStatus.StatusHint)
	require.NotEmpty(t, res.Blockers)
	require.Equal(t, "CHECKPOINT_CORRUPT", res.Blockers[0].Code)
	require.Contains(t, res.DivergenceSummary.ByKind, audit.DivergenceCheckpointCorrupt)
}

func TestEvaluateRecoveryReadiness_AuditWindowGap_Blocked(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, tail := lifecycleEvents(t, "cid-ready-3", "sz000904", "req-ready-3")
	createPersistentCheckpoint(t, database, "cid-ready-3", prefix)

	var afterID uint
	require.NoError(t, database.Model(&models.RecoveryCheckpoint{}).
		Where("scope = ?", "client_order_id:cid-ready-3").
		Select("last_audit_id").Scan(&afterID).Error)

	seedEvents(t, database, tail)
	var high uint
	require.NoError(t, database.Model(&models.AuditEvent{}).
		Where("client_order_id = ?", "cid-ready-3").Select("MAX(id)").Scan(&high).Error)

	// Delete middle/tail window to create a gap relative to checkpoint + high watermark.
	resDel := database.Where("client_order_id = ? AND id > ? AND id <= ?", "cid-ready-3", afterID, high).
		Delete(&models.AuditEvent{})
	require.NoError(t, resDel.Error)
	require.Greater(t, resDel.RowsAffected, int64(0))

	// Re-insert a later event so the window is non-empty but has holes (or empty→anchor path).
	// Prefer deleting the anchor row to force a clear gap.
	require.NoError(t, database.Where("id = ?", afterID).Delete(&models.AuditEvent{}).Error)

	res := recovery.EvaluateRecoveryReadiness(database, &recovery.Options{SkipReplayVerification: true})
	require.Equal(t, recovery.StatusBlocked, res.Status)
	require.Greater(t, res.AuditContinuity.GapCount, 0)
	require.Equal(t, recovery.StatusBlocked, res.AuditContinuity.StatusHint)
	found := false
	for _, b := range res.Blockers {
		if b.Code == "AUDIT_WINDOW_GAP" {
			found = true
		}
	}
	require.True(t, found)
	require.Contains(t, res.DivergenceSummary.ByKind, audit.DivergenceAuditWindowGap)
}

func TestEvaluateRecoveryReadiness_SpecHashMismatch_Blocked(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, _ := lifecycleEvents(t, "cid-ready-4", "sz000905", "req-ready-4")
	createPersistentCheckpoint(t, database, "cid-ready-4", prefix)

	// Append a fill with a foreign spec_hash into the audit stream.
	tampered := audit.BuildFillApplied(
		audit.BackendRealStub, "req-ready-4", "cid-ready-4", "1", "1",
		"tampered-spec-hash-value", "tr-bad", "BAD-E", "",
		10, 10.0, 50, 10.0, 50, 10, "pending", "partially_filled", time.Now().UTC(),
	)
	require.NoError(t, audit.NewDBAuditSink(database).Write(tampered))

	res := recovery.EvaluateRecoveryReadiness(database, &recovery.Options{SampleSize: 1})
	require.Equal(t, recovery.StatusBlocked, res.Status)
	found := false
	for _, b := range res.Blockers {
		if b.Code == "SPEC_HASH_MISMATCH" {
			found = true
		}
	}
	require.True(t, found, "blockers=%+v", res.Blockers)
	require.Contains(t, res.DivergenceSummary.ByKind, audit.DivergenceSpecHashMismatch)
}

func TestEvaluateRecoveryReadiness_SampleReplayMismatch_Blocked(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, tail := lifecycleEvents(t, "cid-ready-5", "sz000906", "req-ready-5")
	createPersistentCheckpoint(t, database, "cid-ready-5", prefix)
	seedEvents(t, database, tail)

	// Tamper snapshot_json while keeping hash in sync so LoadVerified passes,
	// but restored state diverges from a true full replay.
	var row models.RecoveryCheckpoint
	require.NoError(t, database.Where("scope = ?", "client_order_id:cid-ready-5").First(&row).Error)
	snap, err := audit.DecodeSnapshot(row.SnapshotJSON, row.SnapshotHash)
	require.NoError(t, err)
	snap.FillQty = 1 // diverge from real fold
	snap.Order.FilledVolume = 1
	jsonText, hash, err := audit.EncodeSnapshot(snap)
	require.NoError(t, err)
	require.NoError(t, database.Model(&row).Updates(map[string]any{
		"snapshot_json": jsonText,
		"snapshot_hash": hash,
	}).Error)

	res := recovery.EvaluateRecoveryReadiness(database, &recovery.Options{SampleSize: 1})
	require.Equal(t, recovery.StatusBlocked, res.Status)
	require.Greater(t, res.ReplayVerification.FailedCount, 0)
	found := false
	for _, b := range res.Blockers {
		if b.Code == "SAMPLE_REPLAY_MISMATCH" {
			found = true
		}
	}
	require.True(t, found, "blockers=%+v", res.Blockers)
}

func TestEvaluateRecoveryReadiness_NilDB_Blocked(t *testing.T) {
	res := recovery.EvaluateRecoveryReadiness(nil, nil)
	require.Equal(t, recovery.StatusBlocked, res.Status)
	require.Equal(t, "DB_UNAVAILABLE", res.Blockers[0].Code)
}

func TestEvaluateRecoveryReadiness_DoesNotPersistCheckpoint(t *testing.T) {
	database := setupRecoveryReadyDB(t)
	prefix, tail := lifecycleEvents(t, "cid-ready-6", "sz000907", "req-ready-6")
	createPersistentCheckpoint(t, database, "cid-ready-6", prefix)
	seedEvents(t, database, tail)

	var before models.RecoveryCheckpoint
	require.NoError(t, database.Where("scope = ?", "client_order_id:cid-ready-6").First(&before).Error)

	_ = recovery.EvaluateRecoveryReadiness(database, &recovery.Options{SampleSize: 1})
	_ = recovery.EvaluateRecoveryReadiness(database, &recovery.Options{SampleSize: 1})

	var after models.RecoveryCheckpoint
	require.NoError(t, database.Where("scope = ?", "client_order_id:cid-ready-6").First(&after).Error)
	require.Equal(t, before.Version, after.Version)
	require.Equal(t, before.SnapshotHash, after.SnapshotHash)
	require.Equal(t, before.LastAuditID, after.LastAuditID)
	require.Equal(t, before.UpdatedAt.UnixNano(), after.UpdatedAt.UnixNano())
}

func checkpointFingerprint(t *testing.T, database *gorm.DB) string {
	t.Helper()
	var rows []models.RecoveryCheckpoint
	require.NoError(t, database.Order("id ASC").Find(&rows).Error)
	out := ""
	for _, r := range rows {
		out += fmt.Sprintf("%s|%d|%s|%d;", r.Scope, r.LastAuditID, r.SnapshotHash, r.Version)
	}
	return out
}
