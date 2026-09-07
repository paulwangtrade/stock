package strategysnapshot

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateStrategySnapshots creates strategy_snapshots + plan_strategy_refs (idempotent).
// Does not alter trade_plans / trade_plan_items or backfill historical rows.
func MigrateStrategySnapshots(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("strategysnapshot: database is nil")
	}
	if err := database.AutoMigrate(&StrategySnapshotRow{}, &PlanStrategyRefRow{}); err != nil {
		return fmt.Errorf("strategysnapshot migrate: %w", err)
	}
	return nil
}
