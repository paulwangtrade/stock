package db

import (
	"fmt"

	"go-stock/backend/models"

	"gorm.io/gorm"
)

// MigrateAuditEvents creates only the append-only audit_events table.
// AutoMigrate makes repeated calls idempotent; no historical rows are backfilled.
func MigrateAuditEvents(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return database.AutoMigrate(&models.AuditEvent{})
}
