package diagnostic

import (
	"go-stock/backend/db"
)

// MigrationSnapshot is a privacy-safe database schema migration projection.
type MigrationSnapshot struct {
	CurrentVersion  int      `json:"current_version"`
	RequiredVersion int      `json:"required_version"`
	Status          string   `json:"status"`
	MissingTables   []string `json:"missing_tables,omitempty"`
	MissingColumns  []string `json:"missing_columns,omitempty"`
	MissingIndexes  []string `json:"missing_indexes,omitempty"`
	TradingAllowed  bool     `json:"trading_allowed"`
}

// MigrationProvider supplies schema validation snapshot (wired from main startup).
type MigrationProvider func() MigrationSnapshot

// ConfigRedactedProvider returns redacted settings JSON for bundle export.
type ConfigRedactedProvider func() string

var (
	migrationProvider      MigrationProvider
	configRedactedProvider ConfigRedactedProvider
)

// SetMigrationProvider wires startup schema validation into diagnostics (read-only).
func SetMigrationProvider(p MigrationProvider) {
	migrationProvider = p
}

// ResetMigrationProviderForTest clears the migration provider.
func ResetMigrationProviderForTest() {
	migrationProvider = nil
}

// SetConfigRedactedProvider wires redacted settings export into bundle zip.
func SetConfigRedactedProvider(p ConfigRedactedProvider) {
	configRedactedProvider = p
}

// ResetConfigRedactedProviderForTest clears the config provider.
func ResetConfigRedactedProviderForTest() {
	configRedactedProvider = nil
}

func collectMigration(tradingAllowed bool) MigrationSnapshot {
	if migrationProvider != nil {
		return migrationProvider()
	}
	return MigrationSnapshot{
		CurrentVersion:  db.GetAppliedSchemaVersion(),
		RequiredVersion: 0,
		Status:          db.SchemaValidationBlocked,
		TradingAllowed:  tradingAllowed,
	}
}

func MapSchemaValidation(v db.SchemaValidationResult, tradingAllowed bool) MigrationSnapshot {
	return MigrationSnapshot{
		CurrentVersion:  v.CurrentVersion,
		RequiredVersion: v.RequiredVersion,
		Status:          v.Status,
		MissingTables:   append([]string(nil), v.MissingTables...),
		MissingColumns:  append([]string(nil), v.MissingColumns...),
		MissingIndexes:  append([]string(nil), v.MissingIndexes...),
		TradingAllowed:  tradingAllowed,
	}
}
