package db

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func testRegistry(t *testing.T, migrations ...Migration) *MigrationRegistry {
	t.Helper()
	registry, err := NewMigrationRegistry(migrations...)
	require.NoError(t, err)
	return registry
}

func TestMigrationRegistry_AppliesInOrderAndIsIdempotent(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	var order []int
	registry := testRegistry(t,
		Migration{Version: 1, Name: "one", Checksum: "sum-one", Up: func(tx *gorm.DB) error {
			order = append(order, 1)
			return tx.Exec(`CREATE TABLE registry_one (id INTEGER PRIMARY KEY)`).Error
		}},
		Migration{Version: 2, Name: "two", Checksum: "sum-two", Up: func(tx *gorm.DB) error {
			order = append(order, 2)
			return tx.Exec(`CREATE TABLE registry_two (id INTEGER PRIMARY KEY)`).Error
		}},
	)

	first, err := registry.Apply(Dao)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, order)
	require.Equal(t, []int{1, 2}, first.Applied)
	require.Equal(t, 2, GetAppliedSchemaVersion())

	second, err := registry.Apply(Dao)
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2}, second.Skipped)
	require.Equal(t, []int{1, 2}, order, "Up must not run twice")

	var count int64
	require.NoError(t, Dao.Model(&SchemaMigration{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestMigrationRegistry_FailureRollsBackAndDoesNotRecord(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	registry := testRegistry(t,
		Migration{Version: 1, Name: "fail", Checksum: "sum-fail", Up: func(tx *gorm.DB) error {
			require.NoError(t, tx.Exec(`CREATE TABLE rolled_back (id INTEGER PRIMARY KEY)`).Error)
			return errors.New("boom")
		}},
	)

	_, err := registry.Apply(Dao)
	require.ErrorContains(t, err, "boom")
	require.False(t, Dao.Migrator().HasTable("rolled_back"))
	require.Equal(t, 0, GetAppliedSchemaVersion())
}

func TestMigrationRegistry_DetectsChecksumDriftAndFutureVersion(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	registry := testRegistry(t,
		Migration{Version: 1, Name: "one", Checksum: "registry-sum", Up: func(*gorm.DB) error { return nil }},
	)
	require.NoError(t, ensureSchemaMigrationsTable())
	require.NoError(t, Dao.Create(&SchemaMigration{
		Version: 1, Name: "one", Checksum: "database-sum", Status: SchemaMigrationStatusApplied,
	}).Error)
	require.ErrorContains(t, registry.ValidateApplied(Dao), "checksum drift")

	require.NoError(t, Dao.Where("version = ?", 1).Delete(&SchemaMigration{}).Error)
	require.NoError(t, Dao.Create(&SchemaMigration{
		Version: 2, Name: "future", Checksum: "future-sum", Status: SchemaMigrationStatusApplied,
	}).Error)
	require.ErrorContains(t, registry.ValidateApplied(Dao), "unknown")
}

func TestSchemaMigrationTable_UpgradesLegacyMetadataColumns(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	require.NoError(t, Dao.Exec(`
CREATE TABLE schema_migrations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	version INTEGER NOT NULL UNIQUE,
	applied_at DATETIME NOT NULL
)`).Error)
	require.NoError(t, Dao.Exec(`
INSERT INTO schema_migrations(version, applied_at) VALUES (1, CURRENT_TIMESTAMP)
`).Error)

	require.NoError(t, ensureSchemaMigrationsTable())
	for _, field := range []string{"Name", "Checksum", "Status", "DurationMS"} {
		require.True(t, Dao.Migrator().HasColumn(&SchemaMigration{}, field), field)
	}
	require.Equal(t, 1, GetAppliedSchemaVersion())
}

func TestValidateSchema_ReportsMissingColumnAndVersion(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	registry := testRegistry(t,
		Migration{Version: 1, Name: "one", Checksum: "sum-one", Up: func(*gorm.DB) error { return nil }},
		Migration{Version: 2, Name: "two", Checksum: "sum-two", Up: func(*gorm.DB) error { return nil }},
	)
	require.NoError(t, MarkSchemaVersion(1))
	require.NoError(t, Dao.Exec(`CREATE TABLE candidate_pool_items (id INTEGER PRIMARY KEY, rank INTEGER)`).Error)

	result := ValidateSchema(Dao, registry, []SchemaRequirement{{
		Table: "candidate_pool_items", Columns: []string{"rank", "decision_id"},
	}})
	require.Equal(t, SchemaValidationBlocked, result.Status)
	require.Equal(t, 1, result.CurrentVersion)
	require.Equal(t, 2, result.RequiredVersion)
	require.Contains(t, result.MissingColumns, "candidate_pool_items.decision_id")
	require.NotEmpty(t, result.Errors)
}
