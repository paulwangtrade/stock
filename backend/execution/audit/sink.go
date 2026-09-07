package audit

import (
	"encoding/json"
	"sync"

	"go-stock/backend/logger"

	"gorm.io/gorm"
)

// AuditSink receives L0 audit events (append-only persistence / in-process trail).
type AuditSink interface {
	Write(Event) error
}

// Sink is a backward-compatible alias for AuditSink.
type Sink = AuditSink

// NopSink discards events.
type NopSink struct{}

func (NopSink) Write(Event) error { return nil }

// MemorySink stores events in order (test / in-process audit trail).
type MemorySink struct {
	mu     sync.Mutex
	events []Event
}

// MemoryAuditSink is the persistence-layer name for MemorySink.
type MemoryAuditSink = MemorySink

func NewMemorySink() *MemorySink {
	return &MemorySink{events: make([]Event, 0, 32)}
}

// NewMemoryAuditSink constructs a MemoryAuditSink (same behavior as NewMemorySink).
func NewMemoryAuditSink() *MemoryAuditSink {
	return NewMemorySink()
}

func (s *MemorySink) Write(ev Event) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	return nil
}

// Events returns a copy of stored events in append order.
func (s *MemorySink) Events() []Event {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// Len returns event count.
func (s *MemorySink) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// LogSink writes structured JSON logs (no DB / no migration).
type LogSink struct{}

func NewLogSink() LogSink { return LogSink{} }

func (LogSink) Write(ev Event) error {
	if logger.SugaredLogger == nil {
		return nil
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		logger.SugaredLogger.Warnw("execution audit marshal failed", "error", err, "event_type", ev.EventType)
		return nil
	}
	logger.SugaredLogger.Infow("execution_audit_event",
		"event_id", ev.EventID,
		"event_type", ev.EventType,
		"spec_hash", ev.SpecHash,
		"broker_request_id", ev.BrokerRequestID,
		"report_id", ev.ReportID,
		"exec_id", ev.ExecID,
		"payload", json.RawMessage(raw),
	)
	return nil
}

// MultiSink fans out to all sinks; first error wins after attempting all.
type MultiSink struct {
	sinks []Sink
}

func NewMultiSink(sinks ...Sink) *MultiSink {
	out := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			out = append(out, s)
		}
	}
	return &MultiSink{sinks: out}
}

func (m *MultiSink) Write(ev Event) error {
	if m == nil {
		return nil
	}
	var first error
	for _, s := range m.sinks {
		if err := s.Write(ev); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// NewDefaultEmitter memory + structured log (MVP persistence without tables).
func NewDefaultEmitter(mem *MemorySink) *Emitter {
	if mem == nil {
		mem = NewMemorySink()
	}
	return NewEmitter(NewMultiSink(mem, NewLogSink()))
}

// NewPersistenceEmitter memory + log + DBAuditSink for durable Submit audit.
// database may be nil; DB writes then degrade without affecting Emit callers.
func NewPersistenceEmitter(mem *MemorySink, database *gorm.DB) *Emitter {
	if mem == nil {
		mem = NewMemorySink()
	}
	return NewEmitter(NewMultiSink(mem, NewLogSink(), NewDBAuditSink(database)))
}
