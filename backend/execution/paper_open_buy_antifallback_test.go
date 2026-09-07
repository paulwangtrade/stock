package execution

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// fakeExecutionPort 记录 Submit，不调用 SubmitPaperOrder；直接落一条最小 PaperOrder 供 adapter 回读。
type fakeExecutionPort struct {
	submitCalls int
	lastIntent  SubmitIntent
}

func (f *fakeExecutionPort) Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error) {
	_ = ctx
	f.submitCalls++
	f.lastIntent = intent
	now := time.Now()
	order := data.PaperOrder{
		AccountID:     1,
		StockCode:     intent.StockCode,
		StockName:     intent.StockName,
		Side:          intent.Side,
		Status:        data.PaperOrderStatusFilled,
		Price:         intent.Price,
		Volume:        intent.Volume,
		FilledPrice:   intent.Price,
		FilledVol:     intent.Volume,
		Reason:        intent.Reason,
		StrategyTag:   intent.StrategyTag,
		ClientOrderID: "paper_fake_" + fmt.Sprintf("%d", f.submitCalls),
		ExecBackend:   data.PaperExecBackendPaper,
		CreatedAt:     now,
		UpdatedAt:     now,
		FilledAt:      &now,
	}
	requireDB := db.Dao
	if requireDB == nil {
		return nil, fmt.Errorf("db nil")
	}
	if err := requireDB.Create(&order).Error; err != nil {
		return nil, err
	}
	out := mapPaperOrderToTradeOrder(order)
	return &out, nil
}

func (f *fakeExecutionPort) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	_ = ctx
	_ = orderID
	return nil, fmt.Errorf("not used")
}

func (f *fakeExecutionPort) Cancel(ctx context.Context, orderID string) error {
	_ = ctx
	_ = orderID
	return ErrCancelNotImplemented
}

type recordingSubmitAPI struct {
	submitCalls int
}

func (r *recordingSubmitAPI) SubmitPaperOrder(req data.PaperSubmitOrderReq) (*data.PaperOrder, error) {
	r.submitCalls++
	return nil, fmt.Errorf("SubmitPaperOrder must not be called in antifallback test")
}

func (r *recordingSubmitAPI) FillPaperOrder(orderID uint, fillPrice float64) error {
	return fmt.Errorf("FillPaperOrder must not be called")
}

func (r *recordingSubmitAPI) FillPaperOrderQty(orderID uint, fillPrice float64, fillQty int64) error {
	return fmt.Errorf("FillPaperOrderQty must not be called")
}

func (r *recordingSubmitAPI) RejectPaperOrderSim(orderID uint, reason, message string) error {
	return fmt.Errorf("RejectPaperOrderSim must not be called")
}

func (r *recordingSubmitAPI) CancelPaperOrder(orderID uint) error {
	return fmt.Errorf("CancelPaperOrder must not be called")
}

func TestRunPaperOpenBuyUsesExecutionPort(t *testing.T) {
	setupCutoverTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	recAPI := &recordingSubmitAPI{}
	_ = NewPaperBroker(recAPI) // 确保旁路 Broker 存在也不会被开盘路径使用

	fakePort := &fakeExecutionPort{}
	svc := NewExecutionService(fakePort)
	data.SetPlanItemExecutor(newPlanItemExecutorAdapter(svc))

	data.SetOpenBuyQuoteFetcherForTest(func(codes ...string) (*[]data.StockInfo, error) {
		out := make([]data.StockInfo, 0, len(codes))
		for _, c := range codes {
			out = append(out, data.StockInfo{Code: c, Name: "测试股", Price: "10.00"})
		}
		return &out, nil
	})
	t.Cleanup(func() {
		data.SetOpenBuyQuoteFetcherForTest(nil)
	})

	tradeDate := time.Now().Format("2006-01-02")
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		Status:         models.TradePlanStatusReady,
		Side:           "buy",
		AmountPerStock: 10_000,
		EnableExecute:  true,
		FreezeAt:       &now,
		FreezeBy:       "antifallback",
		ApprovedAt:     &now,
		ApprovedBy:     "antifallback",
	}
	items := []models.TradePlanItem{{
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		Status:       models.TradePlanItemPending,
		Priority:     1,
		LimitPrice:   10.0,
		TargetVolume: 1000,
		TargetAmount: 10_000,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))

	res := data.RunPaperOpenBuyOnce(false)
	require.Contains(t, res.Message, "done ok=")
	require.Equal(t, 1, fakePort.submitCalls, "ExecutionPort.Submit must be called")
	require.Equal(t, 0, recAPI.submitCalls, "SubmitPaperOrder must not be called on side path")
	require.Equal(t, "sz000001", fakePort.lastIntent.StockCode)
	require.Equal(t, 10.0, fakePort.lastIntent.Price, "must use Frozen limit_price, not live quote")
	require.Equal(t, int64(1000), fakePort.lastIntent.Volume, "must use Frozen target_volume")
	require.True(t, fakePort.lastIntent.AutoFill)
	require.Len(t, res.Items, 1)
	require.True(t, res.Items[0].OK)
	require.NotZero(t, res.Items[0].OrderID)
}

func TestRunPaperOpenBuy_NoExecutorFailsWithoutSubmitFallback(t *testing.T) {
	setupCutoverTestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	data.SetPlanItemExecutor(nil)

	data.SetOpenBuyQuoteFetcherForTest(func(codes ...string) (*[]data.StockInfo, error) {
		out := []data.StockInfo{{Code: codes[0], Name: "x", Price: "10.00"}}
		return &out, nil
	})
	t.Cleanup(func() { data.SetOpenBuyQuoteFetcherForTest(nil) })

	tradeDate := time.Now().Format("2006-01-02")
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: tradeDate, Status: models.TradePlanStatusReady, Side: "buy", AmountPerStock: 10_000,
		ApprovedAt: &now, ApprovedBy: "antifallback",
		FreezeAt: &now, FreezeBy: "antifallback",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	res := data.RunPaperOpenBuyOnce(false)
	require.Contains(t, res.Message, "executor not configured")
	require.Contains(t, strings.ToLower(res.Message), "refuse silent")
	require.Empty(t, res.Items)
}
