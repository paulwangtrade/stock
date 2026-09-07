package strategysnapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SQLiteStore persists StrategySnapshot documents and plan→item refs in SQLite.
type SQLiteStore struct {
	db *gorm.DB
}

// NewSQLiteStore wraps an open gorm DB (usually db.Dao).
func NewSQLiteStore(database *gorm.DB) *SQLiteStore {
	return &SQLiteStore{db: database}
}

func (s *SQLiteStore) Save(snap *StrategySnapshot) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("strategysnapshot: sqlite store not initialized")
	}
	if snap == nil || strings.TrimSpace(snap.SnapshotID) == "" {
		return fmt.Errorf("strategysnapshot: invalid snapshot")
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("strategysnapshot: marshal payload: %w", err)
	}
	signalJSON, _ := json.Marshal(snap.SignalResult)
	riskJSON, _ := json.Marshal(snap.RiskDecision)
	entryJSON, _ := json.Marshal(map[string]any{
		"strategy_version": snap.StrategyVersion,
		"market_data_ref":  snap.MarketDataRef,
		"parameters":        snap.Parameters,
	})
	ver := strings.TrimSpace(snap.SchemaVersion)
	if ver == "" {
		ver = SchemaVersion
	}
	now := time.Now().UTC()
	row := StrategySnapshotRow{
		SnapshotID:      snap.SnapshotID,
		SnapshotVersion: ver,
		Scope:           string(snap.Scope),
		PlanID:          snap.PlanID,
		PlanItemID:      snap.PlanItemID,
		TradeDate:       snap.TradeDate,
		PayloadJSON:     string(payload),
		SignalJSON:      string(signalJSON),
		RiskJSON:        string(riskJSON),
		EntryJSON:       string(entryJSON),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if !snap.CapturedAt.IsZero() {
		row.CreatedAt = snap.CapturedAt.UTC()
	}
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "snapshot_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"snapshot_version", "scope", "plan_id", "plan_item_id", "trade_date",
			"payload_json", "signal_json", "risk_json", "entry_json", "updated_at",
		}),
	}).Create(&row).Error
}

func (s *SQLiteStore) Get(snapshotID string) (*StrategySnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("strategysnapshot: sqlite store not initialized")
	}
	snapshotID = strings.TrimSpace(snapshotID)
	if snapshotID == "" {
		return nil, ErrNotFound
	}
	var row StrategySnapshotRow
	err := s.db.Where("snapshot_id = ?", snapshotID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodePayload(row.PayloadJSON)
}

func (s *SQLiteStore) ListByPlan(planID uint) ([]StrategySnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("strategysnapshot: sqlite store not initialized")
	}
	if planID == 0 {
		return nil, fmt.Errorf("strategysnapshot: plan id required")
	}
	var rows []StrategySnapshotRow
	if err := s.db.Where("plan_id = ?", planID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StrategySnapshot, 0, len(rows))
	for _, row := range rows {
		snap, err := decodePayload(row.PayloadJSON)
		if err != nil {
			continue
		}
		out = append(out, *snap)
	}
	return out, nil
}

func (s *SQLiteStore) SavePlanRef(ref *PlanReference) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("strategysnapshot: sqlite store not initialized")
	}
	if ref == nil || ref.PlanID == 0 {
		return fmt.Errorf("strategysnapshot: invalid plan reference")
	}
	now := time.Now().UTC()
	if !ref.CapturedAt.IsZero() {
		now = ref.CapturedAt.UTC()
	}
	rows := make([]PlanStrategyRefRow, 0, 1+len(ref.ItemSnapshotIDs))
	if sid := strings.TrimSpace(ref.PlanSnapshotID); sid != "" {
		rows = append(rows, PlanStrategyRefRow{
			PlanID:             ref.PlanID,
			PlanItemID:         0,
			StrategySnapshotID: sid,
			CreatedAt:          now,
		})
	}
	for itemID, sid := range ref.ItemSnapshotIDs {
		sid = strings.TrimSpace(sid)
		if sid == "" {
			continue
		}
		rows = append(rows, PlanStrategyRefRow{
			PlanID:             ref.PlanID,
			PlanItemID:         itemID,
			StrategySnapshotID: sid,
			CreatedAt:          now,
		})
	}
	if len(rows) == 0 {
		return fmt.Errorf("strategysnapshot: plan reference has no snapshot ids")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			row := rows[i]
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "plan_id"}, {Name: "plan_item_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"strategy_snapshot_id"}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLiteStore) GetPlanRef(planID uint) (*PlanReference, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("strategysnapshot: sqlite store not initialized")
	}
	if planID == 0 {
		return nil, ErrNotFound
	}
	var rows []PlanStrategyRefRow
	if err := s.db.Where("plan_id = ?", planID).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	ref := &PlanReference{
		PlanID:          planID,
		ItemSnapshotIDs: make(map[uint]string),
	}
	for _, row := range rows {
		if row.PlanItemID == 0 {
			ref.PlanSnapshotID = row.StrategySnapshotID
			ref.CapturedAt = row.CreatedAt
			continue
		}
		ref.ItemSnapshotIDs[row.PlanItemID] = row.StrategySnapshotID
		if ref.CapturedAt.IsZero() || row.CreatedAt.Before(ref.CapturedAt) {
			ref.CapturedAt = row.CreatedAt
		}
	}
	// Enrich trade_date / plan_version from any linked snapshot payload when present.
	if sid := ref.PlanSnapshotID; sid != "" {
		if snap, err := s.Get(sid); err == nil && snap != nil {
			ref.TradeDate = snap.TradeDate
			ref.PlanVersion = snap.PlanVersion
			ref.Trigger = snap.Trigger
		}
	} else {
		for _, sid := range ref.ItemSnapshotIDs {
			if snap, err := s.Get(sid); err == nil && snap != nil {
				ref.TradeDate = snap.TradeDate
				ref.PlanVersion = snap.PlanVersion
				ref.Trigger = snap.Trigger
				break
			}
		}
	}
	return ref, nil
}

func decodePayload(raw string) (*StrategySnapshot, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("strategysnapshot: empty payload_json")
	}
	var snap StrategySnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return nil, fmt.Errorf("strategysnapshot: decode payload: %w", err)
	}
	if strings.TrimSpace(snap.SnapshotID) == "" {
		return nil, fmt.Errorf("strategysnapshot: payload missing snapshot_id")
	}
	return &snap, nil
}
