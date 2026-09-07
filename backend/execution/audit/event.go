package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// SchemaVersion L0 audit envelope version (no DB schema).
const SchemaVersion = "execution.audit.v1"

// SpecHashVersion frozen fingerprint algorithm id.
const SpecHashVersion = "spec_hash.v1"

// Event types (Phase6.5.7.6.1 contract).
const (
	TypeSubmitAttempt  = "SUBMIT_ATTEMPT"
	TypeSubmitAccepted = "SUBMIT_ACCEPTED"
	TypeSubmitRejected = "SUBMIT_REJECTED"
	TypeSubmitTimeout  = "SUBMIT_TIMEOUT" // covers timeout|unknown via payload.submit_outcome
	TypeReportReceived = "REPORT_RECEIVED"
	TypeFillApplied    = "FILL_APPLIED"
	TypeOrderTerminal  = "ORDER_TERMINAL"
)

// Sources.
const (
	SourceSafetyGate         = "safety_gate"
	SourceExecution          = "execution"
	SourceBrokerStub         = "broker_stub"
	SourceReportConsumer     = "report_consumer"
	SourcePositionAccounting = "position_accounting"
)

// Exec backends (no real broker in this phase).
const (
	BackendPaper    = "paper"
	BackendRealStub = "real_stub"
)

// Event is the L0 audit envelope (logical only; no migration / no audit table).
type Event struct {
	EventID          string         `json:"event_id"`
	EventType        string         `json:"event_type"`
	SchemaVersion    string         `json:"schema_version"`
	EventTime        time.Time      `json:"event_time"`
	RecordedAt       time.Time      `json:"recorded_at"`
	Source           string         `json:"source"`
	ExecBackend      string         `json:"exec_backend"`
	BrokerRequestID  string         `json:"broker_request_id,omitempty"`
	SpecHash         string         `json:"spec_hash"`
	SpecHashVersion  string         `json:"spec_hash_version"`
	LocalOrderID     string         `json:"local_order_id,omitempty"`
	ClientOrderID    string         `json:"client_order_id,omitempty"`
	BrokerOrderID    string         `json:"broker_order_id,omitempty"`
	ExternalOrderID  string         `json:"external_order_id,omitempty"`
	AccountID        string         `json:"account_id,omitempty"`
	ReportID         string         `json:"report_id,omitempty"`
	ExecID           string         `json:"exec_id,omitempty"`
	CausationEventID string         `json:"causation_event_id,omitempty"`
	Payload          map[string]any `json:"payload,omitempty"`
}

// SpecFingerprint derives Frozen Spec hash: symbol|side|limit_price|target_volume.
// Price/volume must be Spec projections (limit_price / target_volume), never fill prices.
func SpecFingerprint(symbol, side string, limitPrice float64, targetVolume int64) string {
	side = strings.ToLower(strings.TrimSpace(side))
	if side == "" {
		side = "buy"
	}
	raw := fmt.Sprintf("%s|%s|%.8f|%d",
		strings.TrimSpace(symbol), side, limitPrice, targetVolume)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

// EventID helpers (stable idempotency keys).

func IDSubmitAttempt(brokerRequestID string) string {
	return "submit_attempt:" + strings.TrimSpace(brokerRequestID)
}

func IDSubmitResult(brokerRequestID, submitOutcome string) string {
	return fmt.Sprintf("submit_result:%s:%s", strings.TrimSpace(brokerRequestID), strings.TrimSpace(submitOutcome))
}

func IDReportReceived(reportID string) string {
	return "report_received:" + strings.TrimSpace(reportID)
}

func IDFillApplied(execID string) string {
	return "fill_applied:" + strings.TrimSpace(execID)
}

func IDOrderTerminal(localOrderID, terminalStatus, causationID string) string {
	return fmt.Sprintf("order_terminal:%s:%s:%s",
		strings.TrimSpace(localOrderID), strings.TrimSpace(terminalStatus), strings.TrimSpace(causationID))
}

// NewBase builds a partially filled envelope; caller sets type-specific fields.
func NewBase(eventType, source, backend, brokerRequestID, specHash string, eventTime time.Time) Event {
	now := time.Now().UTC()
	if eventTime.IsZero() {
		eventTime = now
	} else {
		eventTime = eventTime.UTC()
	}
	return Event{
		EventType:       eventType,
		SchemaVersion:   SchemaVersion,
		EventTime:       eventTime,
		RecordedAt:      now,
		Source:          source,
		ExecBackend:     backend,
		BrokerRequestID: strings.TrimSpace(brokerRequestID),
		SpecHash:        strings.TrimSpace(specHash),
		SpecHashVersion: SpecHashVersion,
		Payload:         map[string]any{},
	}
}
