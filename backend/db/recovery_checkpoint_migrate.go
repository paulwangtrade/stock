package db

import (
	"fmt"

	"go-stock/backend/models"

	"gorm.io/gorm"
)

// MigrateRecoveryCheckpoints creates only the audit_recovery_checkpoints table.
// AutoMigrate makes repeated calls idempotent; it never touches Order / Fill /
// Position business tables and never backfills historical rows.
func MigrateRecoveryCheckpoints(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return database.AutoMigrate(&models.RecoveryCheckpoint{})
}
