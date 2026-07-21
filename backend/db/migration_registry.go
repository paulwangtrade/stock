package db

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Migration 是一个不可变、单调递增的数据库变更。
type Migration struct {
	Version  int
	Name     string
	Checksum string
	Up       func(*gorm.DB) error
}

// MigrationDescriptor 是可用于 golden/report 的只读 registry 元数据。
type MigrationDescriptor struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
}

// MigrationApplyResult 描述一次启动迁移的执行结果。
type MigrationApplyResult struct {
	FromVersion int      `json:"fromVersion"`
	ToVersion   int      `json:"toVersion"`
	Applied     []int    `json:"applied"`
	Skipped     []int    `json:"skipped"`
	Errors      []string `json:"errors,omitempty"`
}

// MigrationRegistry 按声明顺序保存完整、连续的 migration 历史。
type MigrationRegistry struct {
	migrations []Migration
}

var migrationApplyMu sync.Mutex

// NewMigrationRegistry 校验版本、名称和 checksum，拒绝乱序或重复声明。
func NewMigrationRegistry(migrations ...Migration) (*MigrationRegistry, error) {
	if len(migrations) == 0 {
		return nil, fmt.Errorf("migration registry is empty")
	}
	seenNames := make(map[string]struct{}, len(migrations))
	lastVersion := 0
	for i, migration := range migrations {
		migration.Name = strings.TrimSpace(migration.Name)
		migration.Checksum = strings.TrimSpace(migration.Checksum)
		if migration.Version <= 0 {
			return nil, fmt.Errorf("migration[%d] has invalid version %d", i, migration.Version)
		}
		if migration.Version <= lastVersion {
			return nil, fmt.Errorf("migration versions must be strictly increasing: %d after %d", migration.Version, lastVersion)
		}
		if migration.Name == "" {
			return nil, fmt.Errorf("migration version %d has empty name", migration.Version)
		}
		if _, exists := seenNames[migration.Name]; exists {
			return nil, fmt.Errorf("duplicate migration name %q", migration.Name)
		}
		if migration.Checksum == "" {
			return nil, fmt.Errorf("migration version %d has empty checksum", migration.Version)
		}
		if migration.Up == nil {
			return nil, fmt.Errorf("migration version %d has nil Up", migration.Version)
		}
		seenNames[migration.Name] = struct{}{}
		lastVersion = migration.Version
	}
	out := append([]Migration(nil), migrations...)
	return &MigrationRegistry{migrations: out}, nil
}

func (r *MigrationRegistry) LatestVersion() int {
	if r == nil || len(r.migrations) == 0 {
		return 0
	}
	return r.migrations[len(r.migrations)-1].Version
}

func (r *MigrationRegistry) Descriptors() []MigrationDescriptor {
	if r == nil {
		return nil
	}
	out := make([]MigrationDescriptor, 0, len(r.migrations))
	for _, migration := range r.migrations {
		out = append(out, MigrationDescriptor{
			Version:  migration.Version,
			Name:     migration.Name,
			Checksum: migration.Checksum,
		})
	}
	return out
}

func (r *MigrationRegistry) migrationByVersion(version int) (Migration, bool) {
	if r == nil {
		return Migration{}, false
	}
	for _, migration := range r.migrations {
		if migration.Version == version {
			return migration, true
		}
	}
	return Migration{}, false
}

func loadAppliedMigrations(database *gorm.DB) ([]SchemaMigration, error) {
	if err := ensureSchemaMigrationsTableFor(database); err != nil {
		return nil, err
	}
	var applied []SchemaMigration
	if err := database.Order("version ASC").Find(&applied).Error; err != nil {
		return nil, err
	}
	return applied, nil
}

// ValidateApplied 校验数据库迁移历史未领先、未漂移且没有失败状态。
func (r *MigrationRegistry) ValidateApplied(database *gorm.DB) error {
	if r == nil || r.LatestVersion() == 0 {
		return fmt.Errorf("migration registry is not configured")
	}
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	applied, err := loadAppliedMigrations(database)
	if err != nil {
		return err
	}
	for _, row := range applied {
		migration, exists := r.migrationByVersion(row.Version)
		if !exists {
			return fmt.Errorf("database migration version %d is unknown to this binary", row.Version)
		}
		if row.Status != "" && row.Status != SchemaMigrationStatusApplied {
			return fmt.Errorf("migration version %d has non-applied status %q", row.Version, row.Status)
		}
		// Phase5-D 前的记录无 name/checksum；允许 v1 legacy 行，后续版本必须冻结。
		if row.Name != "" && row.Name != "legacy" && row.Name != migration.Name {
			return fmt.Errorf("migration version %d name drift: database=%q registry=%q", row.Version, row.Name, migration.Name)
		}
		if row.Checksum != "" && row.Checksum != migration.Checksum {
			return fmt.Errorf("migration version %d checksum drift: database=%q registry=%q", row.Version, row.Checksum, migration.Checksum)
		}
	}
	return nil
}

// ValidateComplete 在历史未漂移的基础上，要求 registry 中每个版本均已应用。
func (r *MigrationRegistry) ValidateComplete(database *gorm.DB) error {
	if err := r.ValidateApplied(database); err != nil {
		return err
	}
	applied, err := loadAppliedMigrations(database)
	if err != nil {
		return err
	}
	appliedVersions := make(map[int]struct{}, len(applied))
	for _, row := range applied {
		appliedVersions[row.Version] = struct{}{}
	}
	for _, migration := range r.migrations {
		if _, exists := appliedVersions[migration.Version]; !exists {
			return fmt.Errorf("required migration version %d %s is not applied", migration.Version, migration.Name)
		}
	}
	return nil
}

// Apply 按版本逐个事务执行缺失 migration；同一 registry 可安全重复执行。
func (r *MigrationRegistry) Apply(database *gorm.DB) (MigrationApplyResult, error) {
	result := MigrationApplyResult{ToVersion: r.LatestVersion()}
	if database == nil {
		err := fmt.Errorf("数据库未初始化")
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	migrationApplyMu.Lock()
	defer migrationApplyMu.Unlock()

	if err := r.ValidateApplied(database); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	appliedRows, err := loadAppliedMigrations(database)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	applied := make(map[int]SchemaMigration, len(appliedRows))
	for _, row := range appliedRows {
		applied[row.Version] = row
		if row.Version > result.FromVersion {
			result.FromVersion = row.Version
		}
	}

	for _, migration := range r.migrations {
		if _, exists := applied[migration.Version]; exists {
			result.Skipped = append(result.Skipped, migration.Version)
			continue
		}
		started := time.Now()
		err := database.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return err
			}
			row := SchemaMigration{
				Version:    migration.Version,
				Name:       migration.Name,
				Checksum:   migration.Checksum,
				Status:     SchemaMigrationStatusApplied,
				DurationMS: time.Since(started).Milliseconds(),
				AppliedAt:  time.Now(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("record migration version %d: %w", migration.Version, err)
			}
			return nil
		})
		if err != nil {
			wrapped := fmt.Errorf("apply migration %d %s: %w", migration.Version, migration.Name, err)
			result.Errors = append(result.Errors, wrapped.Error())
			return result, wrapped
		}
		result.Applied = append(result.Applied, migration.Version)
	}
	if err := r.ValidateComplete(database); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	return result, nil
}
