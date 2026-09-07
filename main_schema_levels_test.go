package main

import (
	"fmt"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/instrument"
	"go-stock/backend/models"
	"go-stock/backend/strategysnapshot"

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
	for _, field := range tradePlanIntentDefaultFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	for _, field := range tradePlanItemIntentFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlanItem{}, field), field)
	}
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "ApprovedSource"))
	require.True(t, db.Dao.Migrator().HasTable(&models.AuditEvent{}))
	require.True(t, db.Dao.Migrator().HasTable(&models.RecoveryCheckpoint{}))
	require.True(t, db.Dao.Migrator().HasTable(&instrument.InstrumentQuantityMeta{}))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "ProviderMode"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "DecisionProvider"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "DecisionVersion"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "AllocationVersion"))
	require.True(t, db.Dao.Migrator().HasTable(&strategysnapshot.StrategySnapshotRow{}))
	require.True(t, db.Dao.Migrator().HasTable(&strategysnapshot.PlanStrategyRefRow{}))
	require.Equal(t, 11, db.GetAppliedSchemaVersion())
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
	require.Equal(t, []int{2, 3, 4, 5, 6, 7, 8, 9, 10}, first.Applied)
	require.True(t, db.Dao.Migrator().HasColumn(&models.CandidatePoolItem{}, "DecisionID"))
	require.True(t, db.Dao.Migrator().HasIndex(&models.CandidatePoolItem{}, "DecisionID"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "FreezeAt"))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "PricingPolicyVersion"))
	require.Equal(t, 11, db.GetAppliedSchemaVersion())

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
	require.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, first.Applied)

	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, second.Skipped)

	var count int64
	require.NoError(t, db.Dao.Model(&db.SchemaMigration{}).Count(&count).Error)
	require.Equal(t, int64(11), count)
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
	require.Equal(t, []int{3, 4, 5, 6, 7, 8, 9, 10}, first.Applied)
	for _, field := range tradePlanLifecycleFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	for _, field := range tradePlanIntentDefaultFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	for _, field := range tradePlanItemIntentFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlanItem{}, field), field)
	}
	require.Equal(t, 11, db.GetAppliedSchemaVersion())
	require.True(t, validateApplicationSchema().Ready())

	// Idempotent re-apply: skip v3+, columns remain.
	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, second.Skipped)
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
	require.Equal(t, 11, ready.RequiredVersion)
	require.Equal(t, 11, ready.CurrentVersion)

	require.NoError(t, db.Dao.Migrator().DropColumn(&models.TradePlan{}, "FreezeAt"))
	blocked := validateApplicationSchema()
	require.False(t, blocked.Ready())
	require.Equal(t, db.SchemaValidationBlocked, blocked.Status)
	require.Contains(t, blocked.MissingColumns, "trade_plans.freeze_at")
}

func TestApplicationMigrationRegistry_UpgradesV3ToExecutionIntentV4(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, runCoreSchemaMigrations())
	require.NoError(t, runExtendedSchemaMigrations())

	plan := models.TradePlan{
		TradeDate: "2026-07-24", Status: models.TradePlanStatusDraft,
		AmountPerStock: 100000, RiskStatus: "passed", PlanVersion: 1, SourceSession: "after_close",
	}
	require.NoError(t, db.Dao.Create(&plan).Error)
	item := models.TradePlanItem{
		PlanID: plan.ID, TradeDate: plan.TradeDate, StockCode: "sz001309",
		Side: "buy", Priority: 1, TargetAmount: 100000, LimitPrice: 0, TargetVolume: 0,
		Status: models.TradePlanItemPending,
	}
	require.NoError(t, db.Dao.Create(&item).Error)

	// Strip Intent columns to simulate stock.db stopped at lifecycle v3.
	for _, field := range tradePlanIntentDefaultFields {
		if db.Dao.Migrator().HasColumn(&models.TradePlan{}, field) {
			require.NoError(t, db.Dao.Migrator().DropColumn(&models.TradePlan{}, field), field)
		}
	}
	for _, field := range tradePlanItemIntentFields {
		if db.Dao.Migrator().HasColumn(&models.TradePlanItem{}, field) {
			require.NoError(t, db.Dao.Migrator().DropColumn(&models.TradePlanItem{}, field), field)
		}
	}
	require.NoError(t, db.MarkSchemaVersion(1))
	require.NoError(t, db.MarkSchemaVersion(2))
	require.NoError(t, db.MarkSchemaVersion(3))

	for _, field := range tradePlanIntentDefaultFields {
		require.False(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	blocked := validateApplicationSchema()
	require.False(t, blocked.Ready())
	require.Contains(t, blocked.MissingColumns, "trade_plans.pricing_policy_version")

	first, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, []int{4, 5, 6, 7, 8, 9, 10}, first.Applied)
	require.Equal(t, 11, db.GetAppliedSchemaVersion())
	for _, field := range tradePlanIntentDefaultFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	for _, field := range tradePlanItemIntentFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlanItem{}, field), field)
	}
	require.True(t, validateApplicationSchema().Ready())

	// No backfill: historical plan/item business fields unchanged.
	var gotPlan models.TradePlan
	require.NoError(t, db.Dao.First(&gotPlan, plan.ID).Error)
	require.Equal(t, "passed", gotPlan.RiskStatus)
	require.Equal(t, 0, gotPlan.PricingPolicyVersion)
	require.Equal(t, "", gotPlan.DefaultEntryRule)
	require.Nil(t, gotPlan.DefaultMaxSlippage)
	require.Equal(t, "", gotPlan.PricingStage)

	var gotItem models.TradePlanItem
	require.NoError(t, db.Dao.First(&gotItem, item.ID).Error)
	require.Equal(t, float64(0), gotItem.LimitPrice)
	require.Equal(t, int64(0), gotItem.TargetVolume)
	require.Equal(t, float64(0), gotItem.RefPrice)
	require.Equal(t, "", gotItem.IntentStatus)
	require.Nil(t, gotItem.MaxSlippage)

	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, second.Skipped)
}

func TestApplicationMigrationRegistry_ExecutionIntentAddColumnIdempotent(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.TradePlan{}, &models.TradePlanItem{}))
	for _, field := range tradePlanIntentDefaultFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	for _, field := range tradePlanItemIntentFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlanItem{}, field), field)
	}
	// Already-present columns: migrate is a no-op (idempotent).
	require.NoError(t, migrateTradePlanExecutionIntentColumns(db.Dao))
	require.NoError(t, migrateTradePlanExecutionIntentColumns(db.Dao))
}

func TestApplicationMigrationRegistry_ApprovedSourceAddColumnIdempotent(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.TradePlan{}))
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "ApprovedSource"))
	require.NoError(t, migrateTradePlanApprovedSource(db.Dao))
	require.NoError(t, migrateTradePlanApprovedSource(db.Dao))
}

func TestApplicationSchemaValidation_ExecutionIntentBlockedAndReady(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, runSchemaMigrations())
	ready := validateApplicationSchema()
	require.True(t, ready.Ready())
	require.Equal(t, 11, ready.RequiredVersion)

	require.NoError(t, db.Dao.Migrator().DropColumn(&models.TradePlanItem{}, "RefPrice"))
	blocked := validateApplicationSchema()
	require.False(t, blocked.Ready())
	require.Contains(t, blocked.MissingColumns, "trade_plan_items.ref_price")
}

func TestApplicationMigrationRegistry_ProviderMetadataAddColumnIdempotent(t *testing.T) {
	setupMainSchemaTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.TradePlan{}))
	for _, field := range tradePlanProviderMetadataFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}
	require.NoError(t, migrateTradePlanProviderMetadataColumns(db.Dao))
	require.NoError(t, migrateTradePlanProviderMetadataColumns(db.Dao))
}

func TestApplicationMigrationRegistry_ProviderMetadataNoBackfill(t *testing.T) {
	setupMainSchemaTestDB(t)
	_, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, 11, db.GetAppliedSchemaVersion())

	plan := models.TradePlan{
		TradeDate: "2026-08-21", Status: models.TradePlanStatusDraft,
		AmountPerStock: 100000, RiskStatus: "passed", PlanVersion: 1, SourceSession: "after_close",
		// Explicit empty: pre-metadata / unrecorded semantics.
		ProviderMode: "", DecisionProvider: "", DecisionVersion: "", AllocationVersion: "",
	}
	require.NoError(t, db.Dao.Create(&plan).Error)

	for _, field := range tradePlanProviderMetadataFields {
		require.NoError(t, db.Dao.Migrator().DropColumn(&models.TradePlan{}, field), field)
		require.False(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}

	// Re-ADD columns (same as v10 Up). Must not UPDATE historical rows to fixed_amount.
	require.NoError(t, migrateTradePlanProviderMetadataColumns(db.Dao))
	for _, field := range tradePlanProviderMetadataFields {
		require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, field), field)
	}

	var got models.TradePlan
	require.NoError(t, db.Dao.First(&got, plan.ID).Error)
	require.Equal(t, "", got.ProviderMode)
	require.Equal(t, "", got.DecisionProvider)
	require.Equal(t, "", got.DecisionVersion)
	require.Equal(t, "", got.AllocationVersion)
	require.Equal(t, "passed", got.RiskStatus)
	require.NotEqual(t, models.TradePlanDecisionProviderFixed, got.DecisionProvider)
}

func TestApplicationMigrationRegistry_UpgradesV10ToStrategySnapshotsV11(t *testing.T) {
	setupMainSchemaTestDB(t)
	_, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, 11, db.GetAppliedSchemaVersion())

	// Simulate stock.db stopped at v10 (provider metadata applied, no snapshot tables).
	require.NoError(t, db.Dao.Migrator().DropTable(&strategysnapshot.StrategySnapshotRow{}))
	require.NoError(t, db.Dao.Migrator().DropTable(&strategysnapshot.PlanStrategyRefRow{}))
	require.NoError(t, db.Dao.Where("version = ?", 11).Delete(&db.SchemaMigration{}).Error)
	require.Equal(t, 10, db.GetAppliedSchemaVersion())
	require.False(t, db.Dao.Migrator().HasTable(&strategysnapshot.StrategySnapshotRow{}))

	first, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Equal(t, []int{11}, first.Applied)
	require.True(t, db.Dao.Migrator().HasTable(&strategysnapshot.StrategySnapshotRow{}))
	require.True(t, db.Dao.Migrator().HasTable(&strategysnapshot.PlanStrategyRefRow{}))
	require.Equal(t, 11, db.GetAppliedSchemaVersion())
	require.True(t, validateApplicationSchema().Ready())

	second, err := applyApplicationMigrations()
	require.NoError(t, err)
	require.Empty(t, second.Applied)
	require.Contains(t, second.Skipped, 11)
}
