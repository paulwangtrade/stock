package main

import (
	"fmt"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMainSchemaTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:main_schema_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestApplicationMigrationRegistry_FreshDatabaseReady(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, runSchemaMigrations())
	require.True(t, db.Dao.Migrator().HasTable(&data.FollowedStock{}))
	require.True(t, db.Dao.Migrator().HasTable(&data.Settings{}))
	require.True(t, db.Dao.Migrator().HasTable(&models.SignalScanSnapshot{}))
	require.True(t, db.Dao.Migrator().HasColumn(&models.CandidatePoolItem{}, "DecisionID"))
	require.Equal(t, 2, db.GetAppliedSchemaVersion())
	require.True(t, validateApplicationSchema().Ready())
}

func TestApplicationMigrationRegistry_UpgradesLegacyCandidateDecisionID(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, runCoreSchemaMigrations())
	require.NoError(t, runExtendedSchemaMigrations())
	require.NoError(t, db.MarkSchemaVersion(1))

	require.NoError(t, db.Dao.Exec(`DROP TABLE candidate_pool_items`).Error)
	require.NoError(t, db.Dao.Exec(`
CREATE TABLE candidate_pool_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	pool_id INTEGER NOT NULL,
	trade_date TEXT,
	stock_code TEXT,
	stock_name TEXT,
	rank INTEGER,
	score REAL,
	reason TEXT,
	strategy_name TEXT,
	strategy_version TEXT,
	industry TEXT,
	tags_json TEXT,
	signal_tag TEXT,
	signal_score REAL,
	signal_snapshot_id INTEGER,
	created_at DATETIME
)`).Error)
	require.False(t, db.Dao.Migrator().HasColumn(&models.CandidatePoolItem{}, "DecisionID"))

	first, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, []int{2}, first.Applied)
	require.True(t, db.Dao.Migrator().HasColumn(&models.CandidatePoolItem{}, "DecisionID"))
	require.True(t, db.Dao.Migrator().HasIndex(&models.CandidatePoolItem{}, "DecisionID"))

	pool := &models.CandidatePool{TradeDate: "2026-07-21"}
	err = data.NewCandidatePoolRepo().CreatePoolWithItems(pool, []models.CandidatePoolItem{{
		StockCode: "sz000001", Rank: 1, Score: 1, DecisionID: "decision-1",
	}})
	require.NoError(t, err)
	require.Equal(t, "decision-1", pool.Items[0].DecisionID)
	require.True(t, validateApplicationSchema().Ready())
}

func TestApplicationMigrationRegistry_RepeatedApplyIsIdempotent(t *testing.T) {
	setupMainSchemaTestDB(t)
	first, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, first.Applied)

	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2}, second.Skipped)

	var count int64
	require.NoError(t, db.Dao.Model(&db.SchemaMigration{}).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestTradingPreflightCheck_BlocksInvalidSchemaButAppContinues(t *testing.T) {
	setupMainSchemaTestDB(t)
	app := NewApp()
	t.Cleanup(func() { app.cron.Stop() })

	preflight := TradingPreflightCheck()
	require.False(t, preflight.Ready)
	require.Equal(t, TradingStatusBlockedSchemaInvalid, preflight.Status)

	app.InitPaperOpenBuyJobs()
	require.Empty(t, app.cron.Entries(), "schema invalid must prevent trading cron registration")
}
