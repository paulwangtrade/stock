package audit

import (
	"sync"
)

// Emitter emits L0 audit events with event_id idempotency (first write wins).
type Emitter struct {
	mu   sync.Mutex
	seen map[string]struct{}
	sink AuditSink
}

// NewEmitter wraps a sink. Nil sink → no-op writes after idempotency tracking.
func NewEmitter(sink AuditSink) *Emitter {
	if sink == nil {
		sink = NopSink{}
	}
	return &Emitter{
		seen: make(map[string]struct{}),
		sink: sink,
	}
}

// Emit writes ev once per EventID. Duplicate EventID returns (false, nil).
func (e *Emitter) Emit(ev Event) (emitted bool, err error) {
	if e == nil {
		return false, nil
	}
	id := ev.EventID
	if id == "" {
		return false, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.seen[id]; ok {
		return false, nil
	}
	if err := e.sink.Write(ev); err != nil {
		return false, err
	}
	e.seen[id] = struct{}{}
	return true, nil
}

// Seen reports whether event_id was already emitted (test/helper).
func (e *Emitter) Seen(eventID string) bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.seen[eventID]
	return ok
}
