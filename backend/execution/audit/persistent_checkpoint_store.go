package audit

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PersistentCheckpointStore stores recovery checkpoints in audit_recovery_checkpoints.
// It is Recovery metadata only: Save/Load never touch Order / Fill / Position tables.
type PersistentCheckpointStore struct {
	db *gorm.DB
}

// NewPersistentCheckpointStore creates a DB-backed CheckpointStore.
// A nil database makes Save return an error and Load behave as empty.
func NewPersistentCheckpointStore(database *gorm.DB) *PersistentCheckpointStore {
	return &PersistentCheckpointStore{db: database}
}

// Save upserts one checkpoint per scope, including the full replay snapshot + hash.
func (s *PersistentCheckpointStore) Save(cp Checkpoint) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("audit replay: persistent checkpoint store database unavailable")
	}
	scope := strings.TrimSpace(cp.Scope)
	if scope == "" {
		return fmt.Errorf("audit replay: checkpoint scope required")
	}
	if strings.TrimSpace(cp.SnapshotJSON) == "" || strings.TrimSpace(cp.SnapshotHash) == "" {
		return fmt.Errorf("audit replay: checkpoint snapshot_json/snapshot_hash required")
	}
	contract := strings.TrimSpace(cp.ReplayContractVersion)
	if contract == "" {
		contract = ReplayContractVersion
	}
	if contract != ReplayContractVersion {
		return fmt.Errorf("%w: refusing to save %q", ErrCheckpointContract, contract)
	}
	if err := VerifyCheckpointIntegrity(Checkpoint{
		Scope:                 scope,
		ReplayContractVersion: contract,
		SnapshotJSON:          cp.SnapshotJSON,
		SnapshotHash:          cp.SnapshotHash,
	}); err != nil {
		return err
	}

	now := time.Now().UTC()
	row := models.RecoveryCheckpoint{
		Scope:                 scope,
		LastAuditID:           cp.LastAuditID,
		LastEventID:           strings.TrimSpace(cp.LastEventID),
		ReplayContractVersion: contract,
		SnapshotJSON:          cp.SnapshotJSON,
		SnapshotHash:          strings.TrimSpace(cp.SnapshotHash),
		EventCount:            cp.EventsSeen,
		FillCount:             cp.FillCount,
		DivergenceCount:       cp.DivergenceCount,
		SubmitOutcome:         strings.TrimSpace(cp.SubmitOutcom),
		Version:               cp.Version,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if row.Version <= 0 {
		row.Version = 1
	}

	var existing models.RecoveryCheckpoint
	err := s.db.Where("scope = ?", scope).First(&existing).Error
	switch {
	case err == nil:
		row.ID = existing.ID
		row.CreatedAt = existing.CreatedAt
		row.Version = existing.Version + 1
		row.UpdatedAt = now
		if writeErr := s.db.Model(&existing).Select(
			"LastAuditID", "LastEventID", "ReplayContractVersion",
			"SnapshotJSON", "SnapshotHash", "EventCount", "FillCount",
			"DivergenceCount", "SubmitOutcome", "Version", "UpdatedAt",
		).Updates(&row).Error; writeErr != nil {
			return fmt.Errorf("audit replay: update checkpoint: %w", writeErr)
		}
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		if writeErr := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "scope"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"last_audit_id", "last_event_id", "replay_contract_version",
				"snapshot_json", "snapshot_hash", "event_count", "fill_count",
				"divergence_count", "submit_outcome", "version", "updated_at",
			}),
		}).Create(&row).Error; writeErr != nil {
			return fmt.Errorf("audit replay: insert checkpoint: %w", writeErr)
		}
		return nil
	default:
		return fmt.Errorf("audit replay: load checkpoint for save: %w", err)
	}
}

// Load returns a checkpoint when present and intact. Missing or corrupted rows
// yield ok=false so callers without verified loading keep the legacy empty-cursor path.
func (s *PersistentCheckpointStore) Load(scope string) (Checkpoint, bool) {
	cp, err := s.LoadVerified(scope)
	if err != nil {
		return Checkpoint{}, false
	}
	return cp, true
}

// LoadVerified loads and verifies snapshot integrity. Distinguishes missing vs corrupt.
func (s *PersistentCheckpointStore) LoadVerified(scope string) (Checkpoint, error) {
	if s == nil || s.db == nil {
		return Checkpoint{}, ErrCheckpointMissing
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return Checkpoint{}, ErrCheckpointMissing
	}

	var row models.RecoveryCheckpoint
	if err := s.db.Where("scope = ?", scope).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Checkpoint{}, ErrCheckpointMissing
		}
		return Checkpoint{}, fmt.Errorf("audit replay: read checkpoint: %w", err)
	}

	cp := checkpointFromRow(row)
	if err := VerifyCheckpointIntegrity(cp); err != nil {
		return Checkpoint{}, err
	}
	return cp, nil
}

func checkpointFromRow(row models.RecoveryCheckpoint) Checkpoint {
	return Checkpoint{
		Scope:                 row.Scope,
		LastAuditID:           row.LastAuditID,
		LastEventID:           row.LastEventID,
		EventsSeen:            row.EventCount,
		SubmitOutcom:          row.SubmitOutcome,
		ReplayContractVersion: row.ReplayContractVersion,
		SnapshotJSON:          row.SnapshotJSON,
		SnapshotHash:          row.SnapshotHash,
		FillCount:             row.FillCount,
		DivergenceCount:       row.DivergenceCount,
		Version:               row.Version,
	}
}
