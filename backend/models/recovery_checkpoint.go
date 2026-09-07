package models

import "time"

// RecoveryCheckpoint persists an audit replay cursor together with the full
// replay snapshot needed to resume incremental replay without re-reading the
// whole audit_events history.
//
// It is Recovery metadata only: it never stores or mutates Order / Fill /
// Position business state, and replay consumers treat it as read-derived
// evidence, not a source of truth.
type RecoveryCheckpoint struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	Scope                 string    `json:"scope" gorm:"size:191;not null;uniqueIndex:uidx_recovery_checkpoint_scope"`
	LastAuditID           uint      `json:"lastAuditId" gorm:"not null"`
	LastEventID           string    `json:"lastEventId" gorm:"size:191;not null"`
	ReplayContractVersion string    `json:"replayContractVersion" gorm:"size:64;not null;index:idx_recovery_checkpoints_contract"`
	SnapshotJSON          string    `json:"snapshotJson" gorm:"type:text;not null"`
	SnapshotHash          string    `json:"snapshotHash" gorm:"size:64;not null"`
	EventCount            int       `json:"eventCount" gorm:"not null"`
	FillCount             int       `json:"fillCount" gorm:"not null"`
	DivergenceCount       int       `json:"divergenceCount" gorm:"not null"`
	SubmitOutcome         string    `json:"submitOutcome" gorm:"size:32"`
	Version               int       `json:"version" gorm:"not null"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

func (RecoveryCheckpoint) TableName() string {
	return "audit_recovery_checkpoints"
}
