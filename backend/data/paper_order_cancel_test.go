package data

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func submitRestingPending(t *testing.T, api *PaperTradingApi, accountID uint) *PaperOrder {
	t.Helper()
	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: accountID,
		StockCode: "sz000001",
		StockName: "平安银行",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  false, // resting：保持 pending 可撤
	})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, PaperOrderStatusPending, order.Status)
	return order
}

func TestCancelPaperOrder_PendingToCancelled_WritesEvent(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order := submitRestingPending(t, api, account.ID)
	require.NoError(t, api.CancelPaperOrder(order.ID))

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusCancelled, dbOrder.Status)

	var ev PaperOrderEvent
	require.NoError(t, db.Dao.Where("order_id = ? AND event_type = ?", order.ID, PaperOrderEventCancelled).First(&ev).Error)
	var payload paperOrderEventPayload
	require.NoError(t, json.Unmarshal([]byte(ev.PayloadJSON), &payload))
	require.Equal(t, order.ID, payload.OrderID)
	require.Equal(t, order.ClientOrderID, payload.ClientOrderID)
	require.Equal(t, PaperExecBackendPaper, payload.ExecBackend)
	require.Equal(t, order.StockCode, payload.Symbol)
	require.Equal(t, order.Side, payload.Side)
	require.InDelta(t, order.Price, payload.Price, 1e-9)
	require.Equal(t, order.Volume, payload.Volume)
	require.Equal(t, PaperOrderStatusCancelled, payload.Status)
	require.Contains(t, ev.PayloadJSON, `"orderId"`)
	require.Contains(t, ev.PayloadJSON, `"clientOrderId"`)
	require.Contains(t, ev.PayloadJSON, `"execBackend"`)
	require.Contains(t, ev.PayloadJSON, `"symbol"`)
}

func TestCancelPaperOrder_FilledFails(t *testing.T) {
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

	err = api.CancelPaperOrder(order.ID)
	require.ErrorIs(t, err, ErrOrderNotCancellable)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusFilled, dbOrder.Status)

	var n int64
	require.NoError(t, db.Dao.Model(&PaperOrderEvent{}).
		Where("order_id = ? AND event_type = ?", order.ID, PaperOrderEventCancelled).
		Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestCancelPaperOrder_RejectedFails(t *testing.T) {
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

	err = api.CancelPaperOrder(order.ID)
	require.ErrorIs(t, err, ErrOrderNotCancellable)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusRejected, dbOrder.Status)
}

func TestCancelPaperOrder_DuplicateSecondFails(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order := submitRestingPending(t, api, account.ID)
	require.NoError(t, api.CancelPaperOrder(order.ID))
	require.ErrorIs(t, api.CancelPaperOrder(order.ID), ErrOrderNotCancellable)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusCancelled, dbOrder.Status)

	var n int64
	require.NoError(t, db.Dao.Model(&PaperOrderEvent{}).
		Where("order_id = ? AND event_type = ?", order.ID, PaperOrderEventCancelled).
		Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestCancelPaperOrder_ConcurrentRaceFails(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order := submitRestingPending(t, api, account.ID)

	// 模拟他途先改写状态（Fill/Reject 等）
	now := time.Now()
	res := db.Dao.Model(&PaperOrder{}).
		Where("id = ? AND status = ?", order.ID, PaperOrderStatusPending).
		Updates(map[string]any{
			"status":     PaperOrderStatusFilled,
			"updated_at": now,
		})
	require.NoError(t, res.Error)
	require.Equal(t, int64(1), res.RowsAffected)

	require.ErrorIs(t, api.CancelPaperOrder(order.ID), ErrOrderNotCancellable)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusFilled, dbOrder.Status)
}

func TestCancelPaperOrder_ParallelOnlyOneWins(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order := submitRestingPending(t, api, account.ID)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- api.CancelPaperOrder(order.ID)
		}()
	}
	wg.Wait()
	close(errs)

	ok, fail := 0, 0
	for e := range errs {
		if e == nil {
			ok++
		} else {
			require.ErrorIs(t, e, ErrOrderNotCancellable)
			fail++
		}
	}
	require.Equal(t, 1, ok)
	require.Equal(t, 7, fail)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusCancelled, dbOrder.Status)
}
