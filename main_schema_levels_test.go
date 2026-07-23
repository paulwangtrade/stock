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
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "FreezeAt"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "PlanVersion"))
	require.Equal(t, 3, db.GetAppliedSchemaVersion())
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
	require.Equal(t, []int{2, 3}, first.Applied)
	require.True(t, db.Dao.Migrator().HasColumn(&models.CandidatePoolItem{}, "DecisionID"))
	require.True(t, db.Dao.Migrator().HasIndex(&models.CandidatePoolItem{}, "DecisionID"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "FreezeAt"))
	require.Equal(t, 3, db.GetAppliedSchemaVersion())

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
	require.Equal(t, []int{1, 2, 3}, first.Applied)

	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2, 3}, second.Skipped)

	var count int64
	require.NoError(t, db.Dao.Model(&db.SchemaMigration{}).Count(&count).Error)
	require.Equal(t, int64(3), count)
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

func TestApplicationMigrationRegistry_UpgradesLegacyTradePlanLifecycleColumns(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, runCoreSchemaMigrations())
	require.NoError(t, runExtendedSchemaMigrations())
	require.NoError(t, db.MarkSchemaVersion(1))
	require.NoError(t, db.MarkSchemaVersion(2))

	// Simulate pre-Phase6 stock.db: trade_plans without lifecycle columns.
	require.NoError(t, db.Dao.Exec(`DROP TABLE IF EXISTS trade_plan_items`).Error)
	require.NoError(t, db.Dao.Exec(`DROP TABLE IF EXISTS trade_plans`).Error)
	require.NoError(t, db.Dao.Exec(`
CREATE TABLE trade_plans (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	trade_date TEXT NOT NULL,
	pool_id INTEGER,
	generated_at DATETIME,
	status TEXT,
	side TEXT,
	amount_per_stock REAL,
	max_names INTEGER,
	enable_execute NUMERIC,
	message TEXT,
	checked_at DATETIME,
	executed_at DATETIME,
	created_at DATETIME,
	updated_at DATETIME,
	risk_status TEXT,
	market_level INTEGER,
	risk_filtered_count INTEGER,
	risk_accepted_count INTEGER,
	risk_summary TEXT,
	risk_snapshot_json TEXT
)`).Error)
	require.NoError(t, db.Dao.Exec(`
CREATE TABLE trade_plan_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	plan_id INTEGER NOT NULL,
	status TEXT,
	order_id INTEGER,
	fill_id INTEGER
)`).Error)

	for _, field := range tradePlanLifecycleFields {
		require.False(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	blocked := validateApplicationSchema()
	require.False(t, blocked.Ready())
	require.Equal(t, db.SchemaValidationBlocked, blocked.Status)
	require.Contains(t, blocked.MissingColumns, "trade_plans.freeze_at")
	require.Contains(t, blocked.MissingColumns, "trade_plans.plan_version")

	first, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, []int{3}, first.Applied)
	for _, field := range tradePlanLifecycleFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	require.Equal(t, 3, db.GetAppliedSchemaVersion())
	require.True(t, validateApplicationSchema().Ready())

	// Idempotent re-apply: skip v3, columns remain.
	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2, 3}, second.Skipped)
	require.True(t, validateApplicationSchema().Ready())
}

func TestApplicationMigrationRegistry_TradePlanLifecycleAddColumnIdempotent(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.TradePlan{}, &models.TradePlanItem{}))
	for _, field := range tradePlanLifecycleFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	require.NoError(t, migrateTradePlanLifecycleColumns(db.Dao))
	require.NoError(t, migrateTradePlanLifecycleColumns(db.Dao))
}

func TestApplicationSchemaValidation_TradePlanLifecycleBlockedAndReady(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, runSchemaMigrations())
	ready := validateApplicationSchema()
	require.True(t, ready.Ready())
	require.Equal(t, db.SchemaValidationReady, ready.Status)
	require.Equal(t, 3, ready.RequiredVersion)
	require.Equal(t, 3, ready.CurrentVersion)

	require.NoError(t, db.Dao.Migrator().DropColumn(&models.TradePlan{}, "FreezeAt"))
	blocked := validateApplicationSchema()
	require.False(t, blocked.Ready())
	require.Equal(t, db.SchemaValidationBlocked, blocked.Status)
	require.Contains(t, blocked.MissingColumns, "trade_plans.freeze_at")
}
