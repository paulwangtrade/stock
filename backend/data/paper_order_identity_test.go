package data

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func TestPaperOrderIdentity_SubmitSetsClientOrderIDAndBackend(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		StockName: "平安银行",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.NotEmpty(t, order.ClientOrderID)
	require.True(t, strings.HasPrefix(order.ClientOrderID, "paper_"))
	require.Equal(t, PaperExecBackendPaper, order.ExecBackend)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, order.ClientOrderID, dbOrder.ClientOrderID)
	require.Equal(t, PaperExecBackendPaper, dbOrder.ExecBackend)

	// 唯一性：连续两单 ClientOrderID 不同
	order2, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000002",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, order2.ClientOrderID)
	require.True(t, strings.HasPrefix(order2.ClientOrderID, "paper_"))
	require.NotEqual(t, order.ClientOrderID, order2.ClientOrderID)
}

func TestPaperOrderIdentity_LifecycleEventPayloadHasClientOrderID(t *testing.T) {
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
	require.NotEmpty(t, order.ClientOrderID)

	events := filterByOrderID(collectHubEvents(ch, 300*time.Millisecond), paperTradingID(order.ID))
	var sawSubmitted, sawFilled bool
	for _, ev := range events {
		switch ev.Type {
		case broker.EventOrderSubmitted, broker.EventOrderFilled, broker.EventOrderRejected:
			require.NotNil(t, ev.Order)
			require.Equal(t, order.ClientOrderID, ev.Order.ClientOrderID)
			require.Equal(t, PaperExecBackendPaper, ev.Order.ExecBackend)
		}
		switch ev.Type {
		case broker.EventOrderSubmitted:
			sawSubmitted = true
		case broker.EventOrderFilled:
			sawFilled = true
		}
	}
	require.True(t, sawSubmitted)
	require.True(t, sawFilled)

	var audit []PaperOrderEvent
	require.NoError(t, db.Dao.Where("order_id = ?", order.ID).Order("id ASC").Find(&audit).Error)
	require.GreaterOrEqual(t, len(audit), 2)
	for _, ev := range audit {
		if ev.EventType != PaperOrderEventSubmitted && ev.EventType != PaperOrderEventFilled {
			continue
		}
		var payload paperOrderEventPayload
		require.NoError(t, json.Unmarshal([]byte(ev.PayloadJSON), &payload))
		require.Equal(t, order.ClientOrderID, payload.ClientOrderID)
		require.Equal(t, PaperExecBackendPaper, payload.ExecBackend)
	}
}

func TestPaperOrderIdentity_RejectedEventPayloadHasClientOrderID(t *testing.T) {
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
	require.NotEmpty(t, order.ClientOrderID)

	events := filterByOrderID(collectHubEvents(ch, 300*time.Millisecond), paperTradingID(order.ID))
	var sawRejected bool
	for _, ev := range events {
		if ev.Type != broker.EventOrderRejected {
			continue
		}
		sawRejected = true
		require.NotNil(t, ev.Order)
		require.Equal(t, order.ClientOrderID, ev.Order.ClientOrderID)
		require.Equal(t, PaperExecBackendPaper, ev.Order.ExecBackend)
	}
	require.True(t, sawRejected)

	var rejEv PaperOrderEvent
	require.NoError(t, db.Dao.Where("order_id = ? AND event_type = ?", order.ID, PaperOrderEventRejected).First(&rejEv).Error)
	var payload paperOrderEventPayload
	require.NoError(t, json.Unmarshal([]byte(rejEv.PayloadJSON), &payload))
	require.Equal(t, order.ClientOrderID, payload.ClientOrderID)
	require.Equal(t, PaperExecBackendPaper, payload.ExecBackend)
}
