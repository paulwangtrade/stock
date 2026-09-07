package audit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go-stock/backend/logger"
	"go-stock/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBAuditSink appends L0 audit events to the audit_events table.
// Write failures and duplicate event_id conflicts are degraded (logged, never panic);
// callers always observe a nil error so business flows are not blocked.
type DBAuditSink struct {
	db *gorm.DB
}

// NewDBAuditSink creates a DB-backed AuditSink. A nil database is allowed and
// Write becomes a no-op degrade path (log + nil error).
func NewDBAuditSink(database *gorm.DB) *DBAuditSink {
	return &DBAuditSink{db: database}
}

// Write appends one audit_events row. Duplicate event_id is ignored (idempotent).
// Errors are logged and swallowed — never returned, never panicked.
func (s *DBAuditSink) Write(ev Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			warnAuditDB("panic recovered", fmt.Errorf("%v", r), ev.EventID, ev.EventType)
			err = nil
		}
	}()

	if s == nil || s.db == nil {
		warnAuditDB("database unavailable", fmt.Errorf("db is nil"), ev.EventID, ev.EventType)
		return nil
	}
	if strings.TrimSpace(ev.EventID) == "" {
		warnAuditDB("empty event_id", fmt.Errorf("event_id required"), ev.EventID, ev.EventType)
		return nil
	}

	row, buildErr := eventToAuditRow(ev)
	if buildErr != nil {
		warnAuditDB("build row failed", buildErr, ev.EventID, ev.EventType)
		return nil
	}

	// Append-only + event_id UNIQUE: conflict → do nothing (no second row).
	if writeErr := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&row).Error; writeErr != nil {
		warnAuditDB("insert failed", writeErr, ev.EventID, ev.EventType)
		return nil
	}
	return nil
}

func eventToAuditRow(ev Event) (models.AuditEvent, error) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return models.AuditEvent{}, err
	}
	version := strings.TrimSpace(ev.SchemaVersion)
	if version == "" {
		version = SchemaVersion
	}
	occurred := ev.EventTime
	if occurred.IsZero() {
		occurred = ev.RecordedAt
	}

	row := models.AuditEvent{
		EventID:         strings.TrimSpace(ev.EventID),
		EventType:       strings.TrimSpace(ev.EventType),
		EventVersion:    version,
		PlanID:          extractPlanID(ev.Payload),
		OrderID:         optionalString(ev.LocalOrderID),
		SpecHash:        strings.TrimSpace(ev.SpecHash),
		PayloadJSON:     string(payload),
		OccurredAt:      occurred,
		BrokerRequestID: strings.TrimSpace(ev.BrokerRequestID),
		ClientOrderID:   strings.TrimSpace(ev.ClientOrderID),
		ReportID:        strings.TrimSpace(ev.ReportID),
		ExecID:          strings.TrimSpace(ev.ExecID),
		ExecBackend:     strings.TrimSpace(ev.ExecBackend),
		AccountID:       strings.TrimSpace(ev.AccountID),
	}
	return row, nil
}

func extractPlanID(payload map[string]any) *uint {
	if payload == nil {
		return nil
	}
	raw, ok := payload["plan_id"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case uint:
		return &v
	case uint64:
		u := uint(v)
		return &u
	case int:
		if v < 0 {
			return nil
		}
		u := uint(v)
		return &u
	case int64:
		if v < 0 {
			return nil
		}
		u := uint(v)
		return &u
	case float64:
		if v < 0 {
			return nil
		}
		u := uint(v)
		return &u
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return nil
		}
		u := uint(n)
		return &u
	default:
		return nil
	}
}

func optionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func warnAuditDB(msg string, err error, eventID, eventType string) {
	if logger.SugaredLogger == nil {
		return
	}
	logger.SugaredLogger.Warnw("execution audit db sink: "+msg,
		"error", err,
		"event_id", eventID,
		"event_type", eventType,
	)
}
