package data

import (
	"encoding/json"
	"testing"

	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func loadOrderEvents(t *testing.T, orderID uint) []PaperOrderEvent {
	t.Helper()
	var events []PaperOrderEvent
	require.NoError(t, db.Dao.Where("order_id = ?", orderID).Order("id ASC").Find(&events).Error)
	return events
}

func parseEventPayload(t *testing.T, ev PaperOrderEvent) paperOrderEventPayload {
	t.Helper()
	var p paperOrderEventPayload
	require.NoError(t, json.Unmarshal([]byte(ev.PayloadJSON), &p))
	return p
}

func TestPaperOrderEventsAutoFillRejected(t *testing.T) {
	setupPaperTradingTestDB(t)
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

	events := loadOrderEvents(t, order.ID)
	require.Len(t, events, 2)
	require.Equal(t, PaperOrderEventSubmitted, events[0].EventType)
	require.Equal(t, PaperOrderEventRejected, events[1].EventType)
	require.Equal(t, PaperOrderRejectCashInsufficient, events[1].Code)
	require.NotEmpty(t, events[1].Message)

	p0 := parseEventPayload(t, events[0])
	require.Equal(t, order.ID, p0.OrderID)
	require.Equal(t, PaperOrderStatusPending, p0.Status)
	require.Equal(t, "buy", p0.Side)
	require.Equal(t, "sh603799", p0.Symbol)

	p1 := parseEventPayload(t, events[1])
	require.Equal(t, PaperOrderStatusRejected, p1.Status)
	require.Equal(t, PaperOrderRejectCashInsufficient, p1.RejectCode)
	require.NotEmpty(t, p1.RejectReason)
	require.Equal(t, 1, p1.FillAttemptCount)

	for _, ev := range events {
		require.NotEqual(t, PaperOrderEventFilled, ev.EventType)
	}
}

func TestPaperOrderEventsAutoFillFilled(t *testing.T) {
	setupPaperTradingTestDB(t)
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

	events := loadOrderEvents(t, order.ID)
	require.Len(t, events, 2)
	require.Equal(t, PaperOrderEventSubmitted, events[0].EventType)
	require.Equal(t, PaperOrderEventFilled, events[1].EventType)

	p1 := parseEventPayload(t, events[1])
	require.Equal(t, PaperOrderStatusFilled, p1.Status)
	require.Equal(t, "sz000001", p1.Symbol)
	require.Equal(t, int64(100), p1.Volume)

	var fillCount int64
	require.NoError(t, db.Dao.Model(&PaperFill{}).Where("order_id = ?", order.ID).Count(&fillCount).Error)
	require.Equal(t, int64(1), fillCount)
}

func TestPaperOrderEventsManualFillFailNoRejectedEvent(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(1_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		Side:      "buy",
		Price:     100,
		Volume:    100,
		AutoFill:  false,
	})
	require.NoError(t, err)

	require.Error(t, api.FillPaperOrder(order.ID, 100))

	events := loadOrderEvents(t, order.ID)
	require.Len(t, events, 1)
	require.Equal(t, PaperOrderEventSubmitted, events[0].EventType)
}
