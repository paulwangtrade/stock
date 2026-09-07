package data

import (
	"testing"
	"time"

	"go-stock/backend/broker"

	"github.com/stretchr/testify/require"
)

func collectHubEvents(ch <-chan broker.TradingEvent, wait time.Duration) []broker.TradingEvent {
	deadline := time.After(wait)
	var out []broker.TradingEvent
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
}

func filterByOrderID(events []broker.TradingEvent, orderID string) []broker.TradingEvent {
	var out []broker.TradingEvent
	for _, ev := range events {
		if ev.OrderID == orderID {
			out = append(out, ev)
		}
	}
	return out
}

func TestEventHubLifecycle_SubmitAndReject(t *testing.T) {
	setupPaperTradingTestDB(t)
	ch, unsub := broker.DefaultHub.Subscribe(64)
	defer unsub()

	api := NewPaperTradingApi()
	account, err := api.ResetAccount(1_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sh603799",
		Side:      "buy",
		Price:     38.50,
		Volume:    2500,
		AutoFill:  true,
	})
	require.Error(t, err)
	require.Equal(t, PaperOrderStatusRejected, order.Status)

	events := filterByOrderID(collectHubEvents(ch, 300*time.Millisecond), paperTradingID(order.ID))
	types := map[string]int{}
	for _, ev := range events {
		types[ev.Type]++
	}
	require.Equal(t, 1, types[broker.EventOrderSubmitted])
	require.Equal(t, 1, types[broker.EventOrderRejected])
	require.Equal(t, 0, types[broker.EventOrderFilled])
	require.Equal(t, 0, types[broker.EventFill])

	var rejected *broker.TradingEvent
	for i := range events {
		if events[i].Type == broker.EventOrderRejected {
			rejected = &events[i]
			break
		}
	}
	require.NotNil(t, rejected)
	require.NotNil(t, rejected.Order)
	require.Equal(t, PaperOrderStatusRejected, rejected.Order.Status)
	require.Equal(t, "sh603799", rejected.Order.Symbol)
	require.Equal(t, PaperOrderRejectCashInsufficient, rejected.Order.RejectCode)
	require.NotEmpty(t, rejected.Order.RejectReason)
	require.Equal(t, 1, rejected.Order.FillAttemptCount)
	require.Equal(t, "buy", rejected.Order.Side)
	require.Equal(t, 38.50, rejected.Order.Price)
	require.Equal(t, int64(2500), rejected.Order.Volume)
}

func TestEventHubLifecycle_SubmitAndFill(t *testing.T) {
	setupPaperTradingTestDB(t)
	ch, unsub := broker.DefaultHub.Subscribe(64)
	defer unsub()

	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)
	require.Equal(t, PaperOrderStatusFilled, order.Status)

	events := filterByOrderID(collectHubEvents(ch, 300*time.Millisecond), paperTradingID(order.ID))
	types := map[string]int{}
	for _, ev := range events {
		types[ev.Type]++
	}
	require.Equal(t, 1, types[broker.EventOrderSubmitted])
	require.Equal(t, 1, types[broker.EventOrderFilled])
	require.Equal(t, 1, types[broker.EventFill]) // 兼容双发
	require.Equal(t, 0, types[broker.EventOrderRejected])

	var filled *broker.TradingEvent
	for i := range events {
		if events[i].Type == broker.EventOrderFilled {
			filled = &events[i]
			break
		}
	}
	require.NotNil(t, filled)
	require.NotNil(t, filled.Order)
	require.Equal(t, PaperOrderStatusFilled, filled.Order.Status)
	require.Equal(t, "sz000001", filled.Order.Symbol)
	require.Equal(t, paperTradingID(account.ID), filled.AccountID)
	require.Empty(t, filled.Order.RejectCode)
}

func TestEventHubLifecycle_RejectDoesNotEmitFilled(t *testing.T) {
	setupPaperTradingTestDB(t)
	ch, unsub := broker.DefaultHub.Subscribe(64)
	defer unsub()

	api := NewPaperTradingApi()
	account, err := api.ResetAccount(500)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000002",
		Side:      "buy",
		Price:     20,
		Volume:    100,
		AutoFill:  true,
	})
	require.Error(t, err)

	events := filterByOrderID(collectHubEvents(ch, 300*time.Millisecond), paperTradingID(order.ID))
	for _, ev := range events {
		require.NotEqual(t, broker.EventOrderFilled, ev.Type)
		require.NotEqual(t, broker.EventFill, ev.Type)
	}
}
