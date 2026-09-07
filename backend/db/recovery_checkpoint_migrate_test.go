package db

import (
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestMigrateRecoveryCheckpoints_FreshDatabase(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	require.NoError(t, MigrateRecoveryCheckpoints(Dao))
	require.True(t, Dao.Migrator().HasTable(&models.RecoveryCheckpoint{}))

	var count int64
	require.NoError(t, Dao.Model(&models.RecoveryCheckpoint{}).Count(&count).Error)
	require.Zero(t, count, "migration must not backfill historical rows")
}

func TestMigrateRecoveryCheckpoints_DuplicateMigrationIsIdempotent(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	require.NoError(t, MigrateRecoveryCheckpoints(Dao))
	require.NoError(t, MigrateRecoveryCheckpoints(Dao))
	require.True(t, Dao.Migrator().HasTable(&models.RecoveryCheckpoint{}))
}

func TestMigrateRecoveryCheckpoints_TableHasRequiredSchema(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	require.NoError(t, MigrateRecoveryCheckpoints(Dao))

	for _, column := range []string{
		"ID", "Scope", "LastAuditID", "LastEventID", "ReplayContractVersion",
		"SnapshotJSON", "SnapshotHash", "EventCount", "FillCount",
		"DivergenceCount", "SubmitOutcome", "Version", "CreatedAt", "UpdatedAt",
	} {
		require.True(t, Dao.Migrator().HasColumn(&models.RecoveryCheckpoint{}, column), column)
	}
	for _, index := range []string{
		"uidx_recovery_checkpoint_scope",
		"idx_recovery_checkpoints_contract",
	} {
		require.True(t, Dao.Migrator().HasIndex(&models.RecoveryCheckpoint{}, index), index)
	}
}

func TestMigrateRecoveryCheckpoints_UniqueScope(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	require.NoError(t, MigrateRecoveryCheckpoints(Dao))

	row := models.RecoveryCheckpoint{
		Scope:                 "client_order_id:cid-1",
		LastAuditID:           10,
		LastEventID:           "fill_applied:E1",
		ReplayContractVersion: "audit.replay.v1",
		SnapshotJSON:          `{"order":{}}`,
		SnapshotHash:          "hash-1",
		Version:               1,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	require.NoError(t, Dao.Create(&row).Error)

	duplicate := row
	duplicate.ID = 0
	require.Error(t, Dao.Create(&duplicate).Error)
}
