package broker

import (
	"sync"
	"testing"
	"time"
)

func TestEventHubPublishAndUnsubscribe(t *testing.T) {
	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe(1)

	hub.Publish(TradingEvent{Type: EventFill})
	event := <-events
	if event.Type != EventFill {
		t.Fatalf("event type = %q, want %q", event.Type, EventFill)
	}
	if event.OccurredAt.IsZero() {
		t.Fatal("OccurredAt was not populated")
	}

	unsubscribe()
	unsubscribe()
	if _, ok := <-events; ok {
		t.Fatal("subscription channel remains open")
	}
}

func TestEventHubPublishDoesNotBlockOnFullSubscriber(t *testing.T) {
	hub := NewEventHub()
	_, unsubscribe := hub.Subscribe(1)
	defer unsubscribe()

	hub.Publish(TradingEvent{Type: EventOrderSubmitted})
	done := make(chan struct{})
	go func() {
		hub.Publish(TradingEvent{Type: EventFill})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber")
	}
}

func TestEventHubConcurrentPublishAndUnsubscribe(t *testing.T) {
	hub := NewEventHub()
	_, unsubscribe := hub.Subscribe(8)

	var publishers sync.WaitGroup
	for range 16 {
		publishers.Add(1)
		go func() {
			defer publishers.Done()
			for range 100 {
				hub.Publish(TradingEvent{Type: EventPositionChanged})
			}
		}()
	}
	unsubscribe()
	publishers.Wait()
}

func TestEventHubAssignsMonotonicSequence(t *testing.T) {
	hub := NewEventHub()
	events, unsubscribe := hub.Subscribe(2)
	defer unsubscribe()

	hub.Publish(TradingEvent{Type: EventOrderSubmitted})
	hub.Publish(TradingEvent{Type: EventFill})
	first := <-events
	second := <-events
	if first.Sequence == 0 || second.Sequence != first.Sequence+1 {
		t.Fatalf("event sequences = %d, %d; want consecutive non-zero values", first.Sequence, second.Sequence)
	}
}
