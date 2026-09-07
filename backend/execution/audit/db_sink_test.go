package audit_test

import (
	"encoding/json"
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

func setupAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:audit_db_sink_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.MigrateAuditEvents(database))
	return database
}

func sampleAuditEvent(eventID string) audit.Event {
	ev := audit.BuildSubmitAttempt(
		audit.BackendRealStub, "req-db-1", "client-1", "local-9", "42",
		audit.SpecFingerprint("sz000001", "buy", 10.5, 200),
		"sz000001", "buy", 10.5, 200, time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC),
	)
	ev.EventID = eventID
	ev.ReportID = "rpt-1"
	ev.ExecID = "exec-1"
	ev.Payload["plan_id"] = uint(77)
	ev.Payload["note"] = "round-trip"
	return ev
}

func TestMemoryAuditSink_BehaviorUnchanged(t *testing.T) {
	mem := audit.NewMemoryAuditSink()
	ev := sampleAuditEvent("mem-1")

	require.NoError(t, mem.Write(ev))
	require.NoError(t, mem.Write(ev)) // MemorySink itself is append; Emitter owns event_id dedup
	require.Equal(t, 2, mem.Len())

	got := mem.Events()
	require.Len(t, got, 2)
	require.Equal(t, "mem-1", got[0].EventID)
	require.Equal(t, audit.TypeSubmitAttempt, got[0].EventType)
	require.Equal(t, ev.SpecHash, got[0].SpecHash)

	// Emitter idempotency path still works with MemoryAuditSink.
	em := audit.NewEmitter(audit.NewMemoryAuditSink())
	ok, err := em.Emit(ev)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = em.Emit(ev)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestDBAuditSink_WriteSuccess(t *testing.T) {
	database := setupAuditDB(t)
	sink := audit.NewDBAuditSink(database)
	ev := sampleAuditEvent("db-success-1")

	require.NoError(t, sink.Write(ev))

	var rows []models.AuditEvent
	require.NoError(t, database.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "db-success-1", rows[0].EventID)
	require.Equal(t, audit.TypeSubmitAttempt, rows[0].EventType)
	require.Equal(t, audit.SchemaVersion, rows[0].EventVersion)
	require.Equal(t, ev.SpecHash, rows[0].SpecHash)
	require.NotNil(t, rows[0].PlanID)
	require.Equal(t, uint(77), *rows[0].PlanID)
	require.NotNil(t, rows[0].OrderID)
	require.Equal(t, "local-9", *rows[0].OrderID)
	require.Equal(t, "req-db-1", rows[0].BrokerRequestID)
	require.Equal(t, "client-1", rows[0].ClientOrderID)
	require.Equal(t, "rpt-1", rows[0].ReportID)
	require.Equal(t, "exec-1", rows[0].ExecID)
	require.Equal(t, audit.BackendRealStub, rows[0].ExecBackend)
	require.Equal(t, "42", rows[0].AccountID)
	require.False(t, rows[0].OccurredAt.IsZero())
}

func TestDBAuditSink_DuplicateEventID_NoSecondRow(t *testing.T) {
	database := setupAuditDB(t)
	sink := audit.NewDBAuditSink(database)
	ev := sampleAuditEvent("db-dup-1")

	require.NoError(t, sink.Write(ev))
	require.NoError(t, sink.Write(ev))

	var count int64
	require.NoError(t, database.Model(&models.AuditEvent{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestDBAuditSink_PayloadJSONRoundTrip(t *testing.T) {
	database := setupAuditDB(t)
	sink := audit.NewDBAuditSink(database)
	ev := sampleAuditEvent("db-rt-1")

	require.NoError(t, sink.Write(ev))

	var row models.AuditEvent
	require.NoError(t, database.Where("event_id = ?", "db-rt-1").First(&row).Error)
	require.NotEmpty(t, row.PayloadJSON)

	var decoded audit.Event
	require.NoError(t, json.Unmarshal([]byte(row.PayloadJSON), &decoded))
	require.Equal(t, ev.EventID, decoded.EventID)
	require.Equal(t, ev.EventType, decoded.EventType)
	require.Equal(t, ev.SpecHash, decoded.SpecHash)
	require.Equal(t, ev.BrokerRequestID, decoded.BrokerRequestID)
	require.Equal(t, ev.LocalOrderID, decoded.LocalOrderID)
	require.Equal(t, "round-trip", decoded.Payload["note"])
}

func TestDBAuditSink_WriteFailureDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		require.NoError(t, audit.NewDBAuditSink(nil).Write(sampleAuditEvent("db-nil-1")))
	})

	var nilSink *audit.DBAuditSink
	require.NotPanics(t, func() {
		require.NoError(t, nilSink.Write(sampleAuditEvent("db-nil-2")))
	})

	// Closed / unusable DB: still no panic, no error to caller.
	database := setupAuditDB(t)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	sink := audit.NewDBAuditSink(database)
	require.NotPanics(t, func() {
		require.NoError(t, sink.Write(sampleAuditEvent("db-closed-1")))
	})
}
