package db

import (
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestMigrateAuditEvents_FreshDatabase(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	require.NoError(t, MigrateAuditEvents(Dao))
	require.True(t, Dao.Migrator().HasTable(&models.AuditEvent{}))

	var count int64
	require.NoError(t, Dao.Model(&models.AuditEvent{}).Count(&count).Error)
	require.Zero(t, count, "migration must not backfill historical rows")
}

func TestMigrateAuditEvents_DuplicateMigrationIsIdempotent(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	require.NoError(t, MigrateAuditEvents(Dao))
	require.NoError(t, MigrateAuditEvents(Dao))
	require.True(t, Dao.Migrator().HasTable(&models.AuditEvent{}))
}

func TestMigrateAuditEvents_TableHasRequiredSchema(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	require.NoError(t, MigrateAuditEvents(Dao))

	for _, column := range []string{
		"ID", "EventID", "EventType", "EventVersion", "PlanID", "OrderID",
		"SpecHash", "PayloadJSON", "OccurredAt", "BrokerRequestID",
		"ClientOrderID", "ReportID", "ExecID", "ExecBackend", "AccountID",
	} {
		require.True(t, Dao.Migrator().HasColumn(&models.AuditEvent{}, column), column)
	}
	for _, index := range []string{
		"uidx_audit_event_id",
		"idx_audit_events_event_type",
		"idx_audit_events_plan_id",
		"idx_audit_events_order_id",
		"idx_audit_events_spec_hash",
		"idx_audit_events_occurred_at",
		"idx_audit_events_broker_request_id",
		"idx_audit_events_client_order_id",
		"idx_audit_events_report_id",
		"idx_audit_events_exec_id",
		"idx_audit_events_exec_backend",
		"idx_audit_events_account_id",
	} {
		require.True(t, Dao.Migrator().HasIndex(&models.AuditEvent{}, index), index)
	}
}

func TestMigrateAuditEvents_UniqueEventID(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	require.NoError(t, MigrateAuditEvents(Dao))

	event := models.AuditEvent{
		EventID:      "submit_attempt:req-1",
		EventType:    "SUBMIT_ATTEMPT",
		EventVersion: "execution.audit.v1",
		SpecHash:     "spec-hash-1",
		PayloadJSON:  `{"event_id":"submit_attempt:req-1"}`,
		OccurredAt:   time.Now(),
	}
	require.NoError(t, Dao.Create(&event).Error)

	duplicate := event
	duplicate.ID = 0
	require.Error(t, Dao.Create(&duplicate).Error)
}
