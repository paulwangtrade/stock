package broker

import (
	"testing"
	"time"
)

func TestPaperEventBridgeLifecycleEvents(t *testing.T) {
	hub := NewEventHub()
	bridge := NewPaperEventBridge(hub)
	ch, unsub := hub.Subscribe(8)
	defer unsub()

	order := TradeOrder{
		ID:               "1",
		AccountID:        "9",
		StockCode:        "sz000001",
		Symbol:           "sz000001",
		Side:             "buy",
		Status:           "pending",
		Price:            10,
		Volume:           100,
		RejectCode:       "",
		FillAttemptCount: 0,
	}
	bridge.OrderSubmitted(order)
	order.Status = "filled"
	bridge.OrderFilled(order)
	order.Status = "rejected"
	order.RejectCode = "CASH_INSUFFICIENT"
	order.RejectReason = "现金不足"
	order.FillAttemptCount = 1
	bridge.OrderRejected(order)

	got := map[string]TradingEvent{}
	deadline := time.After(time.Second)
	for len(got) < 3 {
		select {
		case ev := <-ch:
			got[ev.Type] = ev
		case <-deadline:
			t.Fatalf("timeout waiting events, got=%v", got)
		}
	}

	if got[EventOrderSubmitted].Order == nil || got[EventOrderSubmitted].Order.Status != "pending" {
		t.Fatalf("submitted: %+v", got[EventOrderSubmitted])
	}
	if got[EventOrderFilled].Order == nil || got[EventOrderFilled].Order.Status != "filled" {
		t.Fatalf("filled: %+v", got[EventOrderFilled])
	}
	rej := got[EventOrderRejected]
	if rej.Order == nil || rej.Order.RejectCode != "CASH_INSUFFICIENT" || rej.Order.FillAttemptCount != 1 {
		t.Fatalf("rejected: %+v", rej)
	}
	if rej.Order.Symbol != "sz000001" || rej.AccountID != "9" {
		t.Fatalf("payload incomplete: %+v", rej.Order)
	}
}
