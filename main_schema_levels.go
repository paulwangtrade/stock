package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution"
	"go-stock/backend/instrument"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/strategysnapshot"

	"gorm.io/gorm"
)

// MigrationLevel 迁移级别：启动同步核心表 / 后台扩展表。
type MigrationLevel string

const (
	MigrationCore     MigrationLevel = "core"
	MigrationExtended MigrationLevel = "extended"
)

func coreSchemaModels() []any {
	return []any{
		&data.Settings{},
		&data.AIConfig{},
		&data.FollowedStock{},
		&data.Group{},
		&data.GroupStock{},
		&data.StockInfo{},
		&data.StockBasic{},
		&data.IndexBasic{},
		&models.PromptTemplate{},
		&models.CronTask{},
	}
}

func extendedSchemaModels() []any {
	return []any{
		&db.ChatMemory{},
		&models.StockChangeHistory{},
		&models.AIResponseResult{},
		&models.StockInfoHK{},
		&models.StockInfoUS{},
		&data.FollowedFund{},
		&data.FundBasic{},
		&models.Tags{},
		&models.Telegraph{},
		&models.TelegraphTags{},
		&models.LongTigerRankData{},
		&models.BKDict{},
		&models.WordAnalyze{},
		&models.SentimentResultAnalyze{},
		&models.AiRecommendStocks{},
		&models.AllStockInfo{},
		&models.StockStrategy{},
		&models.StockStrategyRun{},
		&models.SignalScanSnapshot{},
		&models.CandidatePool{},
		&models.CandidatePoolItem{},
		&models.TradePlan{},
		&models.TradePlanItem{},
		&instrument.InstrumentQuantityMeta{},
		&models.ResearchTradeIntent{},
		&models.ManualTradeIntent{},
		&models.AiAssistantSession{},
		&models.GlobalStockIndex{},
		&data.TradingRecord{},
		&data.KLineCacheRecord{},
	}
}

// runCoreSchemaMigrations 启动必须：配置、自选、分组、Paper/RealStub 等。
func runCoreSchemaMigrations() error {
	return runCoreSchemaMigrationsOn(db.Dao)
}

func runCoreSchemaMigrationsOn(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	start := time.Now()
	if err := database.AutoMigrate(coreSchemaModels()...); err != nil {
		return err
	}
	if err := data.MigratePaperTrading(database); err != nil {
		return err
	}
	if err := execution.MigratePaperMargin(database); err != nil {
		return err
	}
	logger.SugaredLogger.Infof("schema migrate [core] done in %s", time.Since(start))
	return nil
}

// runExtendedSchemaMigrations 可延后：快照、新闻、策略、K 线等。
func runExtendedSchemaMigrations() error {
	return runExtendedSchemaMigrationsOn(db.Dao)
}

func runExtendedSchemaMigrationsOn(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	start := time.Now()
	if err := database.AutoMigrate(extendedSchemaModels()...); err != nil {
		return err
	}
	if err := data.MigrateStockKLineTables(database); err != nil {
		return err
	}
	logger.SugaredLogger.Infof("schema migrate [extended] done in %s", time.Since(start))
	return nil
}

func schemaMigrationChecksum(contract string) string {
	sum := sha256.Sum256([]byte(contract))
	return hex.EncodeToString(sum[:])
}

func applicationMigrationRegistry() (*db.MigrationRegistry, error) {
	return db.NewMigrationRegistry(
		db.Migration{
			Version:  1,
			Name:     "baseline_schema",
			Checksum: schemaMigrationChecksum("v1:baseline_schema:core+extended+paper+margin+kline"),
			Up: func(database *gorm.DB) error {
				if err := runCoreSchemaMigrationsOn(database); err != nil {
					return err
				}
				return runExtendedSchemaMigrationsOn(database)
			},
		},
		db.Migration{
			Version:  2,
			Name:     "add_candidate_decision_id",
			Checksum: schemaMigrationChecksum("v2:candidate_pool_items:add:decision_id:varchar(191):index"),
			Up: func(database *gorm.DB) error {
				if !database.Migrator().HasTable(&models.CandidatePoolItem{}) {
					return fmt.Errorf("candidate_pool_items table is missing")
				}
				if !database.Migrator().HasColumn(&models.CandidatePoolItem{}, "DecisionID") {
					if err := database.Migrator().AddColumn(&models.CandidatePoolItem{}, "DecisionID"); err != nil {
						return fmt.Errorf("add candidate_pool_items.decision_id: %w", err)
					}
				}
				if !database.Migrator().HasIndex(&models.CandidatePoolItem{}, "DecisionID") {
					if err := database.Migrator().CreateIndex(&models.CandidatePoolItem{}, "DecisionID"); err != nil {
						return fmt.Errorf("create decision_id index: %w", err)
					}
				}
				return nil
			},
		},
		db.Migration{
			Version:  3,
			Name:     "add_trade_plan_lifecycle_columns",
			Checksum: schemaMigrationChecksum("v3:trade_plans:add:plan_version,freeze_at,freeze_by,freeze_reason,approved_at,approved_by,approval_reason,source_session"),
			Up: func(database *gorm.DB) error {
				return migrateTradePlanLifecycleColumns(database)
			},
		},
		db.Migration{
			Version:  4,
			Name:     "add_trade_plan_execution_intent_columns",
			Checksum: schemaMigrationChecksum("v4:trade_plans+items:add:default_entry_rule,default_max_slippage,pricing_policy_version,pricing_stage,ref_price,ref_source,ref_as_of,entry_rule,max_slippage,intent_status,open_ref_price,priced_at,priced_by"),
			Up: func(database *gorm.DB) error {
				return migrateTradePlanExecutionIntentColumns(database)
			},
		},
		db.Migration{
			Version:  5,
			Name:     "add_trade_plan_approved_source",
			Checksum: schemaMigrationChecksum("v5:trade_plans:add:approved_source"),
			Up: func(database *gorm.DB) error {
				return migrateTradePlanApprovedSource(database)
			},
		},
		db.Migration{
			Version:  6,
			Name:     "create_audit_events",
			Checksum: schemaMigrationChecksum("v6:audit_events:create:event_id,event_type,event_version,plan_id,order_id,spec_hash,payload_json,occurred_at,broker_request_id,client_order_id,report_id,exec_id,exec_backend,account_id"),
			Up: func(database *gorm.DB) error {
				return db.MigrateAuditEvents(database)
			},
		},
		db.Migration{
			Version:  7,
			Name:     "create_audit_recovery_checkpoints",
			Checksum: schemaMigrationChecksum("v7:audit_recovery_checkpoints:create:scope,last_audit_id,last_event_id,replay_contract_version,snapshot_json,snapshot_hash,event_count,fill_count,divergence_count,submit_outcome,version"),
			Up: func(database *gorm.DB) error {
				return db.MigrateRecoveryCheckpoints(database)
			},
		},
		db.Migration{
			Version:  8,
			Name:     "add_trade_plan_cash_rescale_audit",
			Checksum: schemaMigrationChecksum("v8:trade_plans+items:add:parent_plan_id,source_kind,rescale_mode,available_cash_used,required_cash_before,required_cash_after,scale_ratio,original_target_amount"),
			Up: func(database *gorm.DB) error {
				return migrateTradePlanCashRescaleAuditColumns(database)
			},
		},
		db.Migration{
			Version:  9,
			Name:     "create_instrument_quantity_meta",
			Checksum: schemaMigrationChecksum("v9:instrument_quantity_meta:create:stock_code,security_type,market_segment,qty_unit,lot_size"),
			Up: func(database *gorm.DB) error {
				return migrateInstrumentQuantityMeta(database)
			},
		},
		db.Migration{
			Version:  10,
			Name:     "add_trade_plan_provider_metadata",
			Checksum: schemaMigrationChecksum("v10:trade_plans:add:provider_mode,decision_provider,decision_version,allocation_version"),
			Up: func(database *gorm.DB) error {
				return migrateTradePlanProviderMetadataColumns(database)
			},
		},
		db.Migration{
			Version:  11,
			Name:     "create_strategy_snapshots",
			Checksum: schemaMigrationChecksum("v11:strategy_snapshots+plan_strategy_refs:create:snapshot_id,payload_json,plan_id,plan_item_id,strategy_snapshot_id"),
			Up: func(database *gorm.DB) error {
				return strategysnapshot.MigrateStrategySnapshots(database)
			},
		},
	)
}

// tradePlanLifecycleFields Phase6-A 生命周期列（GORM 字段名，供幂等 AddColumn）。
var tradePlanLifecycleFields = []string{
	"PlanVersion",
	"FreezeAt",
	"FreezeBy",
	"FreezeReason",
	"ApprovedAt",
	"ApprovedBy",
	"ApprovalReason",
	"SourceSession",
}

func migrateTradePlanLifecycleColumns(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if !database.Migrator().HasTable(&models.TradePlan{}) {
		return fmt.Errorf("trade_plans table is missing")
	}
	for _, field := range tradePlanLifecycleFields {
		if database.Migrator().HasColumn(&models.TradePlan{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlan{}, field); err != nil {
			return fmt.Errorf("add trade_plans.%s: %w", field, err)
		}
	}
	return nil
}

// tradePlanIntentDefaultFields Phase6.5.6 Execution Intent Plan 头列（GORM 字段名）。
var tradePlanIntentDefaultFields = []string{
	"DefaultEntryRule",
	"DefaultMaxSlippage",
	"PricingPolicyVersion",
	"PricingStage",
}

// tradePlanItemIntentFields Phase6.5.6 Execution Intent Item 列（GORM 字段名）。
var tradePlanItemIntentFields = []string{
	"RefPrice",
	"RefSource",
	"RefAsOf",
	"EntryRule",
	"MaxSlippage",
	"IntentStatus",
	"OpenRefPrice",
	"PricedAt",
	"PricedBy",
}

// tradePlanApprovedSourceFields Phase6.5.6.15 Approve 渠道审计列（GORM 字段名）。
var tradePlanApprovedSourceFields = []string{
	"ApprovedSource",
}

func migrateTradePlanApprovedSource(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if !database.Migrator().HasTable(&models.TradePlan{}) {
		return fmt.Errorf("trade_plans table is missing")
	}
	for _, field := range tradePlanApprovedSourceFields {
		if database.Migrator().HasColumn(&models.TradePlan{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlan{}, field); err != nil {
			return fmt.Errorf("add trade_plans.%s: %w", field, err)
		}
	}
	return nil
}

// tradePlanCashRescaleAuditFields Phase11 cash rescale structured audit (plan header).
var tradePlanCashRescaleAuditFields = []string{
	"ParentPlanID",
	"SourceKind",
	"RescaleMode",
	"AvailableCashUsed",
	"RequiredCashBefore",
	"RequiredCashAfter",
	"ScaleRatio",
}

// tradePlanItemCashRescaleAuditFields Phase11 trim audit on items.
var tradePlanItemCashRescaleAuditFields = []string{
	"OriginalTargetAmount",
}

func migrateTradePlanCashRescaleAuditColumns(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if !database.Migrator().HasTable(&models.TradePlan{}) {
		return fmt.Errorf("trade_plans table is missing")
	}
	if !database.Migrator().HasTable(&models.TradePlanItem{}) {
		return fmt.Errorf("trade_plan_items table is missing")
	}
	for _, field := range tradePlanCashRescaleAuditFields {
		if database.Migrator().HasColumn(&models.TradePlan{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlan{}, field); err != nil {
			return fmt.Errorf("add trade_plans.%s: %w", field, err)
		}
	}
	for _, field := range tradePlanItemCashRescaleAuditFields {
		if database.Migrator().HasColumn(&models.TradePlanItem{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlanItem{}, field); err != nil {
			return fmt.Errorf("add trade_plan_items.%s: %w", field, err)
		}
	}
	return nil
}

func migrateInstrumentQuantityMeta(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return instrument.EnsureQuantityMetaTable(database)
}

// tradePlanProviderMetadataFields G.13 provider identity columns (GORM field names).
var tradePlanProviderMetadataFields = []string{
	"ProviderMode",
	"DecisionProvider",
	"DecisionVersion",
	"AllocationVersion",
}

func migrateTradePlanProviderMetadataColumns(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if !database.Migrator().HasTable(&models.TradePlan{}) {
		return fmt.Errorf("trade_plans table is missing")
	}
	for _, field := range tradePlanProviderMetadataFields {
		if database.Migrator().HasColumn(&models.TradePlan{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlan{}, field); err != nil {
			return fmt.Errorf("add trade_plans.%s: %w", field, err)
		}
	}
	return nil
}

func migrateTradePlanExecutionIntentColumns(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if !database.Migrator().HasTable(&models.TradePlan{}) {
		return fmt.Errorf("trade_plans table is missing")
	}
	if !database.Migrator().HasTable(&models.TradePlanItem{}) {
		return fmt.Errorf("trade_plan_items table is missing")
	}
	for _, field := range tradePlanIntentDefaultFields {
		if database.Migrator().HasColumn(&models.TradePlan{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlan{}, field); err != nil {
			return fmt.Errorf("add trade_plans.%s: %w", field, err)
		}
	}
	for _, field := range tradePlanItemIntentFields {
		if database.Migrator().HasColumn(&models.TradePlanItem{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&models.TradePlanItem{}, field); err != nil {
			return fmt.Errorf("add trade_plan_items.%s: %w", field, err)
		}
	}
	return nil
}

func applicationSchemaRequirements() []db.SchemaRequirement {
	return []db.SchemaRequirement{
		{Table: "candidate_pools", Columns: []string{"id", "trade_date", "status", "item_count"}},
		{
			Table:   "candidate_pool_items",
			Columns: []string{"id", "pool_id", "stock_code", "rank", "score", "decision_id"},
			Indexes: []string{"idx_candidate_pool_items_decision_id"},
		},
		{
			Table: "trade_plans",
			Columns: []string{
				"id", "trade_date", "status", "enable_execute",
				"plan_version", "freeze_at", "freeze_by", "freeze_reason",
				"approved_at", "approved_by", "approval_reason", "approved_source", "source_session",
				"default_entry_rule", "default_max_slippage", "pricing_policy_version", "pricing_stage",
				"parent_plan_id", "source_kind", "rescale_mode",
				"available_cash_used", "required_cash_before", "required_cash_after", "scale_ratio",
				"provider_mode", "decision_provider", "decision_version", "allocation_version",
			},
		},
		{
			Table: "trade_plan_items",
			Columns: []string{
				"id", "plan_id", "status", "order_id", "fill_id",
				"ref_price", "ref_source", "ref_as_of", "entry_rule", "max_slippage",
				"intent_status", "open_ref_price", "priced_at", "priced_by",
				"original_target_amount",
			},
		},
		{Table: "paper_orders", Columns: []string{"id", "client_order_id", "status"}},
		{Table: "paper_fills", Columns: []string{"id", "order_id"}},
		{
			Table: "audit_events",
			Columns: []string{
				"id", "event_id", "event_type", "event_version", "plan_id", "order_id",
				"spec_hash", "payload_json", "occurred_at", "broker_request_id",
				"client_order_id", "report_id", "exec_id", "exec_backend", "account_id",
			},
			Indexes: []string{
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
			},
		},
		{
			Table: "audit_recovery_checkpoints",
			Columns: []string{
				"id", "scope", "last_audit_id", "last_event_id", "replay_contract_version",
				"snapshot_json", "snapshot_hash", "event_count", "fill_count",
				"divergence_count", "submit_outcome", "version", "created_at", "updated_at",
			},
			Indexes: []string{
				"uidx_recovery_checkpoint_scope",
				"idx_recovery_checkpoints_contract",
			},
		},
		{
			Table: "instrument_quantity_meta",
			Columns: []string{
				"id", "stock_code", "security_type", "market_segment", "qty_unit", "lot_size",
			},
			Indexes: []string{"uidx_instrument_quantity_meta_stock_code"},
		},
		{
			Table: "strategy_snapshots",
			Columns: []string{
				"id", "snapshot_id", "snapshot_version", "scope", "plan_id", "plan_item_id",
				"trade_date", "payload_json", "created_at", "updated_at",
			},
			Indexes: []string{"idx_strategy_snapshots_snapshot_id"},
		},
		{
			Table: "plan_strategy_refs",
			Columns: []string{
				"id", "plan_id", "plan_item_id", "strategy_snapshot_id", "created_at",
			},
			Indexes: []string{"uidx_plan_strategy_ref"},
		},
	}
}

func applyApplicationMigrations() (db.MigrationApplyResult, error) {
	registry, err := applicationMigrationRegistry()
	if err != nil {
		return db.MigrationApplyResult{}, err
	}
	result, err := registry.Apply(db.Dao)
	if err != nil {
		return result, err
	}
	logger.SugaredLogger.Infof("schema migration registry: from=%d to=%d applied=%v skipped=%v",
		result.FromVersion, result.ToVersion, result.Applied, result.Skipped)
	return result, nil
}

func validateApplicationSchema() db.SchemaValidationResult {
	registry, err := applicationMigrationRegistry()
	if err != nil {
		return db.SchemaValidationResult{
			Status: db.SchemaValidationBlocked,
			Errors: []string{err.Error()},
		}
	}
	return db.ValidateSchema(db.Dao, registry, applicationSchemaRequirements())
}

// EnsureMigrationLevel 按需确保某级别表已迁移（扩展功能入口调用）。
func EnsureMigrationLevel(level MigrationLevel) {
	if db.Dao == nil {
		return
	}
	if _, err := applyApplicationMigrations(); err != nil {
		logger.SugaredLogger.Errorf("EnsureMigrationLevel %s: %v", level, err)
	}
}
