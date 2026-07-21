package data

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func TestPaperOrderLifecycleModelMigration(t *testing.T) {
	setupPaperTradingTestDB(t)

	require.True(t, dbHasTable(t, "paper_orders"))
	require.True(t, dbHasTable(t, "paper_order_events"))

	now := time.Now()
	order := PaperOrder{
		AccountID:        1,
		StockCode:        "sh600000",
		StockName:        "浦发",
		Side:             "buy",
		Status:           PaperOrderStatusPending,
		Price:            10,
		Volume:           100,
		RejectCode:       "",
		RejectReason:     "",
		FillAttemptCount: 0,
		ExecMode:         PaperOrderExecModeIOCAutofill,
		ClientOrderID:    newPaperClientOrderID(),
		ExecBackend:      PaperExecBackendPaper,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, db.Dao.Create(&order).Error)
	require.NotZero(t, order.ID)

	ev := PaperOrderEvent{
		OrderID:     order.ID,
		EventType:   PaperOrderEventSubmitted,
		Code:        "",
		Message:     "model-only pr1",
		PayloadJSON: `{"status":"pending"}`,
		CreatedAt:   now,
	}
	require.NoError(t, db.Dao.Create(&ev).Error)
	require.NotZero(t, ev.ID)

	var loaded PaperOrder
	require.NoError(t, db.Dao.First(&loaded, order.ID).Error)
	require.Equal(t, PaperOrderStatusPending, loaded.Status)
	require.Equal(t, PaperOrderExecModeIOCAutofill, loaded.ExecMode)
	require.Equal(t, 0, loaded.FillAttemptCount)

	var events []PaperOrderEvent
	require.NoError(t, db.Dao.Where("order_id = ?", order.ID).Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, PaperOrderEventSubmitted, events[0].EventType)
}

func TestClassifyFillRejectCode(t *testing.T) {
	require.Equal(t, PaperOrderRejectCashInsufficient, classifyFillRejectCode(
		paperFillReject(PaperOrderRejectCashInsufficient, "现金不足：需要 100，可用 1")))
	require.Equal(t, PaperOrderRejectPositionInsufficient, classifyFillRejectCode(
		paperFillReject(PaperOrderRejectPositionInsufficient, "可卖数量不足（T+1）：可卖 0，委托 100")))
	require.Equal(t, PaperOrderRejectPositionInsufficient, classifyFillRejectCode(
		paperFillReject(PaperOrderRejectPositionInsufficient, "无持仓")))
	require.Equal(t, PaperOrderRejectInvalidOrder, classifyFillRejectCode(
		paperFillReject(PaperOrderRejectInvalidOrder, "成交价格无效")))
	require.Equal(t, PaperOrderRejectInvalidOrder, classifyFillRejectCode(
		paperFillReject(PaperOrderRejectInvalidOrder, "订单状态不是 pending: rejected")))
	require.Equal(t, PaperOrderRejectInternal, classifyFillRejectCode(fmt.Errorf("db locked")))
}

func TestSubmitPaperOrderAutoFillFailMarksRejected(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(1_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sh603799",
		StockName: "华友钴业",
		Side:      "buy",
		Price:     38.50,
		Volume:    2500,
		AutoFill:  true,
	})
	require.Error(t, err)
	require.NotNil(t, order)
	require.Equal(t, PaperOrderStatusRejected, order.Status)
	require.Equal(t, PaperOrderRejectCashInsufficient, order.RejectCode)
	require.NotEmpty(t, order.RejectReason)
	require.Equal(t, 1, order.FillAttemptCount)
	require.Equal(t, PaperOrderExecModeIOCAutofill, order.ExecMode)
	require.Equal(t, float64(0), order.FilledPrice)
	require.Equal(t, int64(0), order.FilledVol)
	require.Nil(t, order.FilledAt)

	// 验收：DB 不得残留 pending 或 reject_code 为空的失败单
	var stalePending, emptyReject int64
	require.NoError(t, db.Dao.Model(&PaperOrder{}).Where("status = ?", PaperOrderStatusPending).Count(&stalePending).Error)
	require.NoError(t, db.Dao.Model(&PaperOrder{}).
		Where("status = ? AND (reject_code IS NULL OR reject_code = '')", PaperOrderStatusRejected).
		Count(&emptyReject).Error)
	require.Zero(t, stalePending)
	require.Zero(t, emptyReject)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusRejected, dbOrder.Status)
	require.Equal(t, PaperOrderRejectCashInsufficient, dbOrder.RejectCode)
	require.NotEmpty(t, dbOrder.RejectReason)

	var fillCount, positionCount int64
	require.NoError(t, db.Dao.Model(&PaperFill{}).Count(&fillCount).Error)
	require.NoError(t, db.Dao.Model(&PaperPosition{}).Count(&positionCount).Error)
	require.Zero(t, fillCount)
	require.Zero(t, positionCount)
	require.NoError(t, db.Dao.First(account, account.ID).Error)
	require.Equal(t, 1_000.0, account.Cash)
}

func TestSubmitPaperOrderAutoFillSuccessStillFilled(t *testing.T) {
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
	require.Empty(t, order.RejectCode)
	require.Equal(t, 10.0, order.FilledPrice)
	require.Equal(t, int64(100), order.FilledVol)
	require.NotZero(t, order.FilledAt)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperOrderStatusFilled, dbOrder.Status)
	require.Empty(t, dbOrder.RejectCode)
	require.Equal(t, 10.0, dbOrder.FilledPrice)
	require.Equal(t, int64(100), dbOrder.FilledVol)
}

func TestSubmitPaperOrderAutoFillSellNoPositionRejected(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		Side:      "sell",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.Error(t, err)
	require.Equal(t, PaperOrderStatusRejected, order.Status)
	require.Equal(t, PaperOrderRejectPositionInsufficient, order.RejectCode)
	require.NotEmpty(t, order.RejectReason)

	var stalePending int64
	require.NoError(t, db.Dao.Model(&PaperOrder{}).Where("status = ?", PaperOrderStatusPending).Count(&stalePending).Error)
	require.Zero(t, stalePending)
}

func TestManualFillFailKeepsPending(t *testing.T) {
	// PR2-A：仅 AutoFill 失败闭环；独立 Fill 失败仍 pending（可人工重试）
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
	require.Equal(t, PaperOrderStatusPending, order.Status)

	require.Error(t, api.FillPaperOrder(order.ID, 100))
	require.NoError(t, db.Dao.First(&order, order.ID).Error)
	require.Equal(t, PaperOrderStatusPending, order.Status)
	require.Empty(t, order.RejectCode)
}

func dbHasTable(t *testing.T, name string) bool {
	t.Helper()
	return db.Dao.Migrator().HasTable(name)
}
