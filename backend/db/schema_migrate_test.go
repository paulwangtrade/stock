package db

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSchemaMigrateTestDB(t *testing.T) {
	t.Helper()
	original := Dao
	dsn := fmt.Sprintf("file:schema_mig_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	Dao = testDB
	t.Cleanup(func() {
		Dao = original
		_ = sqlDB.Close()
	})
}

func TestMigrateIfNeeded_RunsThenSkips(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	calls := 0
	skipped, err := MigrateIfNeeded(1, func() error {
		calls++
		return Dao.Exec(`CREATE TABLE IF NOT EXISTS demo_tbl (id INTEGER PRIMARY KEY)`).Error
	})
	require.NoError(t, err)
	require.False(t, skipped)
	require.Equal(t, 1, calls)
	require.Equal(t, 1, GetAppliedSchemaVersion())
	require.True(t, IsSchemaCurrent(1))

	skipped, err = MigrateIfNeeded(1, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	require.True(t, skipped)
	require.Equal(t, 1, calls) // 未再次执行
}

func TestMigrateIfNeeded_BumpsOnHigherVersion(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	_, err := MigrateIfNeeded(1, func() error { return nil })
	require.NoError(t, err)

	ran := false
	skipped, err := MigrateIfNeeded(2, func() error {
		ran = true
		return nil
	})
	require.NoError(t, err)
	require.False(t, skipped)
	require.True(t, ran)
	require.Equal(t, 2, GetAppliedSchemaVersion())
	require.True(t, IsSchemaCurrent(2))
	require.True(t, IsSchemaCurrent(1))
}

func TestMigrateIfNeeded_FailedMigrateDoesNotMark(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	skipped, err := MigrateIfNeeded(1, func() error {
		return errors.New("boom")
	})
	require.Error(t, err)
	require.False(t, skipped)
	require.Equal(t, 0, GetAppliedSchemaVersion())
	require.False(t, IsSchemaCurrent(1))
}

func TestMigrateIfNeeded_FreshDBCreatesTables(t *testing.T) {
	setupSchemaMigrateTestDB(t)

	type DemoModel struct {
		ID   uint   `gorm:"primaryKey"`
		Name string `gorm:"size:32"`
	}
	_, err := MigrateIfNeeded(CurrentSchemaVersion, func() error {
		return Dao.AutoMigrate(&DemoModel{})
	})
	require.NoError(t, err)

	require.NoError(t, Dao.Create(&DemoModel{Name: "ok"}).Error)
	var n int64
	require.NoError(t, Dao.Model(&DemoModel{}).Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestMarkSchemaVersion_Idempotent(t *testing.T) {
	setupSchemaMigrateTestDB(t)
	require.NoError(t, MarkSchemaVersion(1))
	require.NoError(t, MarkSchemaVersion(1))
	require.Equal(t, 1, GetAppliedSchemaVersion())

	var rows []SchemaMigration
	require.NoError(t, Dao.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.False(t, rows[0].AppliedAt.IsZero())
	require.WithinDuration(t, time.Now(), rows[0].AppliedAt, time.Minute)
}

func TestInit_DoesNotAutoMigrateChatMemory(t *testing.T) {
	original := Dao
	dsn := fmt.Sprintf("file:init_no_mig_%s?mode=memory&cache=shared", t.Name())
	t.Cleanup(func() { Dao = original })

	Init(dsn)
	require.NotNil(t, Dao)

	// Init 不再迁移业务表；schema_migrations / chat_memory 均不应因 Init 自动出现
	require.False(t, Dao.Migrator().HasTable("chat_memory"))
	require.False(t, Dao.Migrator().HasTable(&SchemaMigration{}))
}
