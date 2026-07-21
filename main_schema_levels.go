package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution"
	"go-stock/backend/logger"
	"go-stock/backend/models"

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
	)
}

func applicationSchemaRequirements() []db.SchemaRequirement {
	return []db.SchemaRequirement{
		{Table: "candidate_pools", Columns: []string{"id", "trade_date", "status", "item_count"}},
		{
			Table:   "candidate_pool_items",
			Columns: []string{"id", "pool_id", "stock_code", "rank", "score", "decision_id"},
			Indexes: []string{"idx_candidate_pool_items_decision_id"},
		},
		{Table: "trade_plans", Columns: []string{"id", "trade_date", "status", "enable_execute"}},
		{Table: "trade_plan_items", Columns: []string{"id", "plan_id", "status", "order_id", "fill_id"}},
		{Table: "paper_orders", Columns: []string{"id", "client_order_id", "status"}},
		{Table: "paper_fills", Columns: []string{"id", "order_id"}},
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
