package audit_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/execution/audit"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCheckpointDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ckpt_persist_%s?mode=memory&cache=shared", t.Name())
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

func seedAuditEvents(t *testing.T, database *gorm.DB, events []audit.Event) {
	t.Helper()
	sink := audit.NewDBAuditSink(database)
	for _, ev := range events {
		require.NoError(t, sink.Write(ev))
	}
}

func sampleLifecycleEvents(t *testing.T) (spec string, events []audit.Event) {
	t.Helper()
	spec = audit.SpecFingerprint("sz000701", "buy", 10, 100)
	now := time.Now().UTC()
	attempt := audit.BuildSubmitAttempt(
		audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"sz000701", "buy", 10, 100, now,
	)
	accepted := audit.BuildSubmitResult(
		audit.TypeSubmitAccepted, audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"accepted", "pending", "accepted", now, now.Add(time.Second), "", "", map[string]any{"status": "pending"},
	)
	report1 := audit.BuildReportReceived(
		audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"ckpt-tr-1", "TRADE", "CK1", "new", "applied", now.Add(2*time.Second), map[string]any{},
		map[string]any{"last_qty": 40, "last_price": 10.0},
	)
	fill1 := audit.BuildFillApplied(
		audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"ckpt-tr-1", "CK1", "report_received:ckpt-tr-1",
		40, 10.0, 40, 10.0, 60, 40,
		"pending", "partially_filled", now.Add(2*time.Second),
	)
	report2 := audit.BuildReportReceived(
		audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"ckpt-tr-2", "TRADE", "CK2", "new", "applied", now.Add(3*time.Second), map[string]any{},
		map[string]any{"last_qty": 60, "last_price": 10.1},
	)
	fill2 := audit.BuildFillApplied(
		audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"ckpt-tr-2", "CK2", "report_received:ckpt-tr-2",
		60, 10.1, 100, 10.06, 0, 60,
		"filled", "filled", now.Add(3*time.Second),
	)
	terminal := audit.BuildOrderTerminal(
		audit.BackendRealStub, "req-ckpt-1", "cid-ckpt-1", "1", "1", spec,
		"filled", "filled", 100, 0, 10.06, "ckpt-tr-2", now.Add(3*time.Second), nil,
	)
	return spec, []audit.Event{attempt, accepted, report1, fill1, report2, fill2, terminal}
}

func TestPersistentCheckpointStore_SaveLoadSuccess(t *testing.T) {
	database := setupCheckpointDB(t)
	store := audit.NewPersistentCheckpointStore(database)

	res := audit.ReplayEvents([]audit.Event{
		replayAttempt(replaySpecHash()),
		replayFill(replaySpecHash(), "E1", 100, 10, 100, 0, "filled", "filled"),
	})
	cp, err := audit.AttachSnapshot(audit.Checkpoint{
		Scope:       "client_order_id:cid-r1",
		LastAuditID: 9,
		LastEventID: "fill_applied:E1",
	}, res)
	require.NoError(t, err)
	require.NoError(t, store.Save(cp))

	loaded, err := store.LoadVerified("client_order_id:cid-r1")
	require.NoError(t, err)
	require.Equal(t, uint(9), loaded.LastAuditID)
	require.Equal(t, "fill_applied:E1", loaded.LastEventID)
	require.Equal(t, audit.ReplayContractVersion, loaded.ReplayContractVersion)
	require.Equal(t, cp.SnapshotHash, loaded.SnapshotHash)
	require.NotEmpty(t, loaded.SnapshotJSON)
	require.Equal(t, 1, loaded.Version)

	require.NoError(t, store.Save(cp)) // upsert bumps version
	loaded2, err := store.LoadVerified("client_order_id:cid-r1")
	require.NoError(t, err)
	require.Equal(t, 2, loaded2.Version)

	var rows int64
	require.NoError(t, database.Model(&models.RecoveryCheckpoint{}).Count(&rows).Error)
	require.Equal(t, int64(1), rows)
}

func TestPersistentCheckpointStore_CorruptionDetected(t *testing.T) {
	database := setupCheckpointDB(t)
	store := audit.NewPersistentCheckpointStore(database)

	res := audit.ReplayEvents([]audit.Event{replayAttempt(replaySpecHash())})
	cp, err := audit.AttachSnapshot(audit.Checkpoint{
		Scope: "client_order_id:cid-corrupt", LastAuditID: 1, LastEventID: "e1",
	}, res)
	require.NoError(t, err)
	require.NoError(t, store.Save(cp))

	require.NoError(t, database.Model(&models.RecoveryCheckpoint{}).
		Where("scope = ?", "client_order_id:cid-corrupt").
		Update("snapshot_json", `{"order":{"stockCode":"tampered"}}`).Error)

	_, err = store.LoadVerified("client_order_id:cid-corrupt")
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointCorrupted))

	_, ok := store.Load("client_order_id:cid-corrupt")
	require.False(t, ok, "Load must not return a corrupted checkpoint")
}

func TestPersistentCheckpointStore_ContractMismatchDetected(t *testing.T) {
	database := setupCheckpointDB(t)
	store := audit.NewPersistentCheckpointStore(database)

	res := audit.ReplayEvents([]audit.Event{replayAttempt(replaySpecHash())})
	cp, err := audit.AttachSnapshot(audit.Checkpoint{
		Scope: "client_order_id:cid-contract", LastAuditID: 1, LastEventID: "e1",
	}, res)
	require.NoError(t, err)
	require.NoError(t, store.Save(cp))

	require.NoError(t, database.Model(&models.RecoveryCheckpoint{}).
		Where("scope = ?", "client_order_id:cid-contract").
		Update("replay_contract_version", "audit.replay.v0").Error)

	_, err = store.LoadVerified("client_order_id:cid-contract")
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointContract))
}

func TestReplay_NoCheckpointKeepsLegacyBehavior(t *testing.T) {
	database := setupCheckpointDB(t)
	_, events := sampleLifecycleEvents(t)
	seedAuditEvents(t, database, events)

	// Default memory store, no prior checkpoint.
	replayer := audit.NewReplayer(database)
	full, err := replayer.Replay(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.NoError(t, err)
	require.Equal(t, int64(100), full.FillQty)
	require.Equal(t, 2, full.FillCount())
	require.True(t, full.Order.Terminal)
	require.NotEmpty(t, full.Checkpoint.SnapshotHash)

	// Fresh replayer with empty persistent store → same as full fold.
	fresh := audit.NewReplayer(database).WithCheckpointStore(audit.NewPersistentCheckpointStore(database))
	again, err := fresh.ReplayIncremental(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.NoError(t, err)
	require.Equal(t, full.Order, again.Order)
	require.Equal(t, full.FillQty, again.FillQty)
	require.Equal(t, full.PositionDelta, again.PositionDelta)
	require.Equal(t, full.SpecHash, again.SpecHash)
}

func TestReplay_CheckpointPlusTailEqualsFullReplay(t *testing.T) {
	database := setupCheckpointDB(t)
	spec, events := sampleLifecycleEvents(t)
	seedAuditEvents(t, database, events[:4]) // attempt+accepted+report1+fill1

	store := audit.NewPersistentCheckpointStore(database)
	replayer := audit.NewReplayer(database).WithCheckpointStore(store)

	partial, err := replayer.Replay(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.NoError(t, err)
	require.Equal(t, int64(40), partial.FillQty)
	require.Equal(t, int64(60), partial.Order.LeavesQuantity)
	require.False(t, partial.Order.Terminal)
	require.NotEmpty(t, partial.Checkpoint.SnapshotHash)

	loaded, err := store.LoadVerified("client_order_id:cid-ckpt-1")
	require.NoError(t, err)
	require.Equal(t, partial.Checkpoint.LastAuditID, loaded.LastAuditID)

	// Append remaining audit events (new rows after checkpoint).
	seedAuditEvents(t, database, events[4:])

	// New process: new Replayer + same persistent store.
	resumed := audit.NewReplayer(database).WithCheckpointStore(store)
	incremental, err := resumed.ReplayIncremental(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.NoError(t, err)

	fullOnly := audit.NewReplayer(database) // memory store, no baseline
	full, err := fullOnly.Replay(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.NoError(t, err)

	require.Equal(t, spec, incremental.SpecHash)
	require.Equal(t, full.Order, incremental.Order)
	require.Equal(t, full.FillQty, incremental.FillQty)
	require.Equal(t, full.FillCount(), incremental.FillCount())
	require.Equal(t, full.PositionDelta, incremental.PositionDelta)
	require.Equal(t, full.SubmitOutcome, incremental.SubmitOutcome)
	require.Equal(t, full.EventsApplied, incremental.EventsApplied)
	require.Equal(t, full.Fills, incremental.Fills)
	require.True(t, incremental.Order.Terminal)
	require.InDelta(t, 10.0, incremental.Order.Price, 1e-9, "Frozen Spec projection immutable")
	require.Equal(t, int64(100), incremental.Order.Volume)
}

func TestReplay_CorruptedCheckpointBlocksIncremental(t *testing.T) {
	database := setupCheckpointDB(t)
	_, events := sampleLifecycleEvents(t)
	seedAuditEvents(t, database, events[:4])

	store := audit.NewPersistentCheckpointStore(database)
	replayer := audit.NewReplayer(database).WithCheckpointStore(store)
	_, err := replayer.Replay(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.NoError(t, err)

	require.NoError(t, database.Model(&models.RecoveryCheckpoint{}).
		Where("scope = ?", "client_order_id:cid-ckpt-1").
		Update("snapshot_hash", "deadbeef").Error)

	seedAuditEvents(t, database, events[4:])
	_, err = audit.NewReplayer(database).WithCheckpointStore(store).
		ReplayIncremental(audit.ReplayScope{ClientOrderID: "cid-ckpt-1"})
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointCorrupted))
}

func TestEncodeSnapshot_HashStable(t *testing.T) {
	res := audit.ReplayEvents([]audit.Event{
		replayAttempt(replaySpecHash()),
		replayFill(replaySpecHash(), "E1", 100, 10, 100, 0, "filled", "filled"),
	})
	snap := audit.SnapshotFromResult(res)
	a, ha, err := audit.EncodeSnapshot(snap)
	require.NoError(t, err)
	b, hb, err := audit.EncodeSnapshot(snap)
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Equal(t, ha, hb)

	_, err = audit.DecodeSnapshot(a, "not-the-hash")
	require.Error(t, err)
	require.True(t, errors.Is(err, audit.ErrCheckpointCorrupted))
}
