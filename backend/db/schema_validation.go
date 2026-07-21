package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	SchemaValidationReady   = "READY"
	SchemaValidationBlocked = "BLOCKED_SCHEMA_INVALID"
)

// SchemaRequirement 声明交易启动前必须存在的表、列和索引。
type SchemaRequirement struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns,omitempty"`
	Indexes []string `json:"indexes,omitempty"`
}

// SchemaValidationResult 是启动 schema 探测的结构化结果。
type SchemaValidationResult struct {
	Status          string   `json:"status"`
	CurrentVersion  int      `json:"currentVersion"`
	RequiredVersion int      `json:"requiredVersion"`
	MissingTables   []string `json:"missingTables,omitempty"`
	MissingColumns  []string `json:"missingColumns,omitempty"`
	MissingIndexes  []string `json:"missingIndexes,omitempty"`
	Errors          []string `json:"errors,omitempty"`
}

func (r SchemaValidationResult) Ready() bool {
	return r.Status == SchemaValidationReady
}

// ValidateSchema 校验 registry 版本历史与业务关键 schema，不修改业务表。
func ValidateSchema(database *gorm.DB, registry *MigrationRegistry, requirements []SchemaRequirement) SchemaValidationResult {
	result := SchemaValidationResult{Status: SchemaValidationBlocked}
	if registry != nil {
		result.RequiredVersion = registry.LatestVersion()
	}
	if database == nil {
		result.Errors = append(result.Errors, "database is not initialized")
		return result
	}
	if registry == nil || registry.LatestVersion() == 0 {
		result.Errors = append(result.Errors, "migration registry is not configured")
		return result
	}

	applied, err := loadAppliedMigrations(database)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	for _, row := range applied {
		if row.Version > result.CurrentVersion {
			result.CurrentVersion = row.Version
		}
	}
	if err := registry.ValidateComplete(database); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if result.CurrentVersion < result.RequiredVersion {
		result.Errors = append(result.Errors,
			fmt.Sprintf("migration version behind: current=%d required=%d", result.CurrentVersion, result.RequiredVersion))
	}

	for _, requirement := range requirements {
		table := strings.TrimSpace(requirement.Table)
		if table == "" {
			result.Errors = append(result.Errors, "schema requirement has empty table")
			continue
		}
		if !database.Migrator().HasTable(table) {
			result.MissingTables = append(result.MissingTables, table)
			continue
		}
		for _, column := range requirement.Columns {
			column = strings.TrimSpace(column)
			if column != "" && !database.Migrator().HasColumn(table, column) {
				result.MissingColumns = append(result.MissingColumns, table+"."+column)
			}
		}
		for _, index := range requirement.Indexes {
			index = strings.TrimSpace(index)
			if index != "" && !database.Migrator().HasIndex(table, index) {
				result.MissingIndexes = append(result.MissingIndexes, table+"."+index)
			}
		}
	}

	if len(result.Errors) == 0 &&
		len(result.MissingTables) == 0 &&
		len(result.MissingColumns) == 0 &&
		len(result.MissingIndexes) == 0 {
		result.Status = SchemaValidationReady
	}
	return result
}
