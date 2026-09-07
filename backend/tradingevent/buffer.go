package tradingevent

import (
	"strings"
	"sync"
)

const defaultBufferCap = 2048

var (
	bufMu   sync.RWMutex
	bufRing []TradingEvent
	bufCap  = defaultBufferCap
)

// AppendToBuffer stores an event for in-process Day Monitor reads (not persisted, not cron LastSteps).
func AppendToBuffer(ev TradingEvent) {
	bufMu.Lock()
	defer bufMu.Unlock()
	if bufCap <= 0 {
		bufCap = defaultBufferCap
	}
	if len(bufRing) >= bufCap {
		// drop oldest
		copy(bufRing, bufRing[1:])
		bufRing = bufRing[:bufCap-1]
	}
	bufRing = append(bufRing, ev)
}

// ListByTradeDate returns a copy of buffered events for tradeDate (chronological).
func ListByTradeDate(tradeDate string) []TradingEvent {
	td := strings.TrimSpace(tradeDate)
	bufMu.RLock()
	defer bufMu.RUnlock()
	out := make([]TradingEvent, 0)
	for _, ev := range bufRing {
		if ev.TradeDate == td {
			out = append(out, ev)
		}
	}
	return out
}

// ResetBufferForTest clears the in-memory event buffer.
func ResetBufferForTest() {
	bufMu.Lock()
	defer bufMu.Unlock()
	bufRing = nil
}
