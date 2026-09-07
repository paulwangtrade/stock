package models

import "time"

// AuditEvent is the append-only persistence envelope for execution audit events.
// It is intentionally independent from execution runtime models and state machines.
type AuditEvent struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	EventID         string    `json:"eventId" gorm:"size:191;not null;uniqueIndex:uidx_audit_event_id"`
	EventType       string    `json:"eventType" gorm:"size:32;not null;index:idx_audit_events_event_type"`
	EventVersion    string    `json:"eventVersion" gorm:"size:32;not null"`
	PlanID          *uint     `json:"planId,omitempty" gorm:"index:idx_audit_events_plan_id"`
	OrderID         *string   `json:"orderId,omitempty" gorm:"size:64;index:idx_audit_events_order_id"`
	SpecHash        string    `json:"specHash" gorm:"size:64;not null;index:idx_audit_events_spec_hash"`
	PayloadJSON     string    `json:"payloadJson" gorm:"type:text;not null"`
	OccurredAt      time.Time `json:"occurredAt" gorm:"not null;index:idx_audit_events_occurred_at"`
	BrokerRequestID string    `json:"brokerRequestId,omitempty" gorm:"size:64;index:idx_audit_events_broker_request_id"`
	ClientOrderID   string    `json:"clientOrderId,omitempty" gorm:"size:64;index:idx_audit_events_client_order_id"`
	ReportID        string    `json:"reportId,omitempty" gorm:"size:128;index:idx_audit_events_report_id"`
	ExecID          string    `json:"execId,omitempty" gorm:"size:128;index:idx_audit_events_exec_id"`
	ExecBackend     string    `json:"execBackend,omitempty" gorm:"size:32;index:idx_audit_events_exec_backend"`
	AccountID       string    `json:"accountId,omitempty" gorm:"size:32;index:idx_audit_events_account_id"`
}

func (AuditEvent) TableName() string {
	return "audit_events"
}
