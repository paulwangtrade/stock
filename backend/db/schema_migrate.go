package db

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// CurrentSchemaVersion 仅保留给尚未迁移到 Registry API 的兼容调用方。
// 生产启动流程不再以该常量决定是否跳过全量 AutoMigrate。
const CurrentSchemaVersion = 2

const SchemaMigrationStatusApplied = "applied"

// SchemaMigration 记录已应用的数据库 schema 版本。
type SchemaMigration struct {
	ID         uint      `gorm:"primaryKey"`
	Version    int       `gorm:"uniqueIndex;not null"`
	Name       string    `gorm:"size:191"`
	Checksum   string    `gorm:"size:128"`
	Status     string    `gorm:"size:32;not null;default:applied"`
	DurationMS int64     `gorm:"not null;default:0"`
	AppliedAt  time.Time `gorm:"not null"`
}

func (SchemaMigration) TableName() string {
	return "schema_migrations"
}

func ensureSchemaMigrationsTable() error {
	if Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return ensureSchemaMigrationsTableFor(Dao)
}

func ensureSchemaMigrationsTableFor(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	// 用原生 DDL，避免每次启动都对 schema_migrations 做 GORM AutoMigrate 开销
	if err := database.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	version INTEGER NOT NULL UNIQUE,
	name TEXT,
	checksum TEXT,
	status TEXT NOT NULL DEFAULT 'applied',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	applied_at DATETIME NOT NULL
)`).Error; err != nil {
		return err
	}
	// 兼容 Phase5-D 前仅含 id/version/applied_at 的版本表。
	for _, field := range []string{"Name", "Checksum", "Status", "DurationMS"} {
		if database.Migrator().HasColumn(&SchemaMigration{}, field) {
			continue
		}
		if err := database.Migrator().AddColumn(&SchemaMigration{}, field); err != nil {
			return fmt.Errorf("upgrade schema_migrations add %s: %w", field, err)
		}
	}
	return nil
}

// GetAppliedSchemaVersion 返回已记录的最高版本；无记录或表不可用时返回 0。
func GetAppliedSchemaVersion() int {
	if Dao == nil {
		return 0
	}
	if err := ensureSchemaMigrationsTable(); err != nil {
		log.Printf("ensure schema_migrations: %v", err)
		return 0
	}
	var ver int
	err := Dao.Raw(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&ver).Error
	if err != nil {
		log.Printf("read schema version: %v", err)
		return 0
	}
	return ver
}

// IsSchemaCurrent 判断库是否已达目标版本（可安全跳过全量迁移）。
func IsSchemaCurrent(targetVersion int) bool {
	return GetAppliedSchemaVersion() >= targetVersion
}

// MarkSchemaVersion 写入已应用版本（幂等：同版本已存在则忽略）。
func MarkSchemaVersion(version int) error {
	if Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := ensureSchemaMigrationsTable(); err != nil {
		return err
	}
	var count int64
	if err := Dao.Model(&SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return Dao.Create(&SchemaMigration{
		Version:   version,
		Name:      "legacy",
		Status:    SchemaMigrationStatusApplied,
		AppliedAt: time.Now(),
	}).Error
}

// MigrateIfNeeded 若已达 targetVersion 则跳过 migrateFn；否则执行并在成功后写入版本。
// skipped=true 表示未执行迁移。migrateFn 失败时不写版本号。
func MigrateIfNeeded(targetVersion int, migrateFn func() error) (skipped bool, err error) {
	if Dao == nil {
		return false, fmt.Errorf("数据库未初始化")
	}
	if targetVersion <= 0 {
		return false, fmt.Errorf("invalid schema version: %d", targetVersion)
	}
	if migrateFn == nil {
		return false, fmt.Errorf("migrateFn is nil")
	}

	applied := GetAppliedSchemaVersion()
	if applied >= targetVersion {
		log.Printf("schema version %d already applied (current=%d), skip migrate", targetVersion, applied)
		return true, nil
	}

	log.Printf("schema migrate: applied=%d target=%d, running...", applied, targetVersion)
	if err := migrateFn(); err != nil {
		return false, err
	}
	if err := MarkSchemaVersion(targetVersion); err != nil {
		return false, fmt.Errorf("mark schema version %d: %w", targetVersion, err)
	}
	log.Printf("schema migrate done, version=%d", targetVersion)
	return false, nil
}

// ResetSchemaVersionForTest 仅测试用：清空版本记录。
func ResetSchemaVersionForTest() error {
	if Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := ensureSchemaMigrationsTable(); err != nil {
		return err
	}
	return Dao.Exec(`DELETE FROM schema_migrations`).Error
}
