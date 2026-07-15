package broker

import (
	"sync"
	"time"
)

const defaultSubscriberBuffer = 32

// EventHub 提供进程内、线程安全的交易事件发布订阅。
// 发布不会阻塞执行链路；订阅者缓冲区满时，该订阅者会丢弃本次事件。
type EventHub struct {
	mu          sync.RWMutex
	publishMu   sync.Mutex
	nextID      uint64
	sequence    uint64
	subscribers map[uint64]chan TradingEvent
}

func NewEventHub() *EventHub {
	return &EventHub{subscribers: make(map[uint64]chan TradingEvent)}
}

// Subscribe 创建事件订阅。buffer 可省略，取消函数可安全重复调用。
func (h *EventHub) Subscribe(buffer ...int) (<-chan TradingEvent, func()) {
	size := defaultSubscriberBuffer
	if len(buffer) > 0 && buffer[0] >= 0 {
		size = buffer[0]
	}

	ch := make(chan TradingEvent, size)
	h.mu.Lock()
	if h.subscribers == nil {
		h.subscribers = make(map[uint64]chan TradingEvent)
	}
	h.nextID++
	id := h.nextID
	h.subscribers[id] = ch
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			if subscriber, ok := h.subscribers[id]; ok {
				delete(h.subscribers, id)
				close(subscriber)
			}
			h.mu.Unlock()
		})
	}
	return ch, unsubscribe
}

func (h *EventHub) Publish(event TradingEvent) {
	h.publishMu.Lock()
	defer h.publishMu.Unlock()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	h.sequence++
	event.Sequence = h.sequence

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

var DefaultHub = NewEventHub()
