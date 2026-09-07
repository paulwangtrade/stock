package tradingevent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// Sink receives emitted events. Production uses structured log; tests may capture.
type Sink func(TradingEvent)

var (
	sinkMu sync.RWMutex
	sink   Sink = defaultLogSink
)

// SetSink replaces the emit destination. Pass nil to restore the log sink.
// Intended for tests; production must not swap sinks.
func SetSink(s Sink) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	if s == nil {
		sink = defaultLogSink
		return
	}
	sink = s
}

// Emit publishes a TradingEvent via the current sink.
// Always appends to the in-process observation buffer first (Day Monitor read path).
// Never panics; never returns an error to callers (observation must not affect trading).
func Emit(ev TradingEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	if ev.EventID == "" {
		ev.EventID = newEventID(ev)
	}
	AppendToBuffer(ev)
	sinkMu.RLock()
	s := sink
	sinkMu.RUnlock()
	if s == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	s(ev)
}

func defaultLogSink(ev TradingEvent) {
	payload, err := json.Marshal(ev)
	if err != nil {
		if logger.SugaredLogger != nil {
			logger.SugaredLogger.Warnf("TradingEvent marshal failed event_type=%s err=%v", ev.EventType, err)
		}
		return
	}
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Infof("TradingEvent %s", string(payload))
	}
}

func newEventID(ev TradingEvent) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%s-%s-%d-%s",
		ev.Source, ev.EventType, ev.TradeDate, ev.PlanID, hex.EncodeToString(b[:]))
}

// EmitFields is a convenience builder used by call sites.
func EmitFields(ev TradingEvent) {
	Emit(ev)
}
