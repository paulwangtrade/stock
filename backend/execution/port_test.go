package execution

import (
	"context"
	"errors"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

type stubPort struct {
	lastIntent SubmitIntent
	order      *broker.TradeOrder
	err        error
	cancelErr  error
	calls      int
}

func (s *stubPort) Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error) {
	_ = ctx
	s.calls++
	s.lastIntent = intent
	return s.order, s.err
}

func (s *stubPort) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	_ = ctx
	if s.order != nil && s.order.ID == orderID {
		return s.order, nil
	}
	return nil, errors.New("not found")
}

func (s *stubPort) Cancel(ctx context.Context, orderID string) error {
	_ = ctx
	_ = orderID
	if s.cancelErr != nil {
		return s.cancelErr
	}
	return ErrCancelNotImplemented
}

func TestExecutionService_ExecutePlanItem_MapsToSubmitIntent(t *testing.T) {
	port := &stubPort{
		order: &broker.TradeOrder{ID: "42", Status: data.PaperOrderStatusFilled, StockCode: "sh600000"},
	}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	})
	item := models.TradePlanItem{
		StockCode:    "sh600000",
		StockName:    "浦发银行",
		Side:         "buy",
		LimitPrice:   9.5,
		TargetVolume: 200,
		Reason:       "from-item",
		StrategyName: "snap",
	}
	got, err := svc.ExecutePlanItem(context.Background(), item, ExecutePlanItemOpts{
		AccountID:   1,
		StockName:   "浦发银行-覆盖",
		Price:       10.2,
		Volume:      300,
		Reason:      "runtime-reason",
		StrategyTag: models.PaperStrategyTagTradePlan,
		AutoFill:    true,
	})
	require.NoError(t, err)
	require.Equal(t, "42", got.ID)
	require.Equal(t, 1, port.calls)
	require.Equal(t, "sh600000", port.lastIntent.StockCode)
	require.Equal(t, "浦发银行-覆盖", port.lastIntent.StockName)
	require.Equal(t, "buy", port.lastIntent.Side)
	require.InDelta(t, 10.2, port.lastIntent.Price, 1e-9)
	require.Equal(t, int64(300), port.lastIntent.Volume)
	require.Equal(t, "runtime-reason", port.lastIntent.Reason)
	require.Equal(t, models.PaperStrategyTagTradePlan, port.lastIntent.StrategyTag)
	require.True(t, port.lastIntent.AutoFill)
	require.Equal(t, uint(1), port.lastIntent.AccountID)
}

func TestExecutionService_ExecutePlanItem_FallsBackToItemFields(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "1", Status: data.PaperOrderStatusPending}}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	})
	_, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode:    "sz000001",
		StockName:    "平安银行",
		LimitPrice:   11,
		TargetVolume: 100,
		Reason:       "plan-reason",
		StrategyName: "enhancer-v1",
	}, ExecutePlanItemOpts{})
	require.NoError(t, err)
	require.Equal(t, 1, port.calls)
	require.Equal(t, "buy", port.lastIntent.Side)
	require.InDelta(t, 11.0, port.lastIntent.Price, 1e-9)
	require.Equal(t, int64(100), port.lastIntent.Volume)
	require.Equal(t, "plan-reason", port.lastIntent.Reason)
	require.Equal(t, "enhancer-v1", port.lastIntent.StrategyTag)
	require.False(t, port.lastIntent.AutoFill)
}

func TestExecutionService_NilPort(t *testing.T) {
	svc := NewExecutionService(nil)
	_, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{StockCode: "x"}, ExecutePlanItemOpts{})
	require.Error(t, err)
}

type stubPaperAPI struct {
	lastReq      data.PaperSubmitOrderReq
	submitOrd    *data.PaperOrder
	submitErr    error
	fillCalled   bool
	fillID       uint
	fillPrice    float64
	fillErr      error
	cancelCalled bool
	cancelID     uint
	cancelErr    error
}

func (s *stubPaperAPI) SubmitPaperOrder(req data.PaperSubmitOrderReq) (*data.PaperOrder, error) {
	s.lastReq = req
	return s.submitOrd, s.submitErr
}

func (s *stubPaperAPI) FillPaperOrder(orderID uint, fillPrice float64) error {
	s.fillCalled = true
	s.fillID = orderID
	s.fillPrice = fillPrice
	return s.fillErr
}

func (s *stubPaperAPI) CancelPaperOrder(orderID uint) error {
	s.cancelCalled = true
	s.cancelID = orderID
	return s.cancelErr
}

func TestPaperBroker_SubmitDelegatesToSubmitPaperOrder(t *testing.T) {
	api := &stubPaperAPI{
		submitOrd: &data.PaperOrder{
			ID: 7, AccountID: 1, StockCode: "sh600519", Side: "buy",
			Status: data.PaperOrderStatusFilled, Price: 1800, Volume: 100, FilledVol: 100, FilledPrice: 1800,
			ClientOrderID: "paper_test_cid", ExecBackend: data.PaperExecBackendPaper,
			BrokerStatus: data.PaperBrokerStatusPaper,
		},
	}
	b := NewPaperBroker(api)
	ord, err := b.Submit(context.Background(), SubmitIntent{
		AccountID: 1, StockCode: "sh600519", StockName: "茅台", Side: "buy",
		Price: 1800, Volume: 100, Reason: "t", StrategyTag: "trade_plan", AutoFill: true,
	})
	require.NoError(t, err)
	require.Equal(t, "7", ord.ID)
	require.Equal(t, data.PaperOrderStatusFilled, ord.Status)
	require.Equal(t, "paper_test_cid", ord.ClientOrderID)
	require.Equal(t, data.PaperExecBackendPaper, ord.ExecBackend)
	require.Empty(t, ord.BrokerOrderID)
	require.Empty(t, ord.ExternalOrderID)
	require.Equal(t, data.PaperBrokerStatusPaper, ord.BrokerStatus)
	require.True(t, api.lastReq.AutoFill)
	require.Equal(t, "sh600519", api.lastReq.StockCode)
	require.False(t, api.fillCalled) // AutoFill 由 SubmitPaperOrder 内部处理，外壳不二次 Fill
}

func TestPaperBroker_FillDelegatesToFillPaperOrder(t *testing.T) {
	api := &stubPaperAPI{}
	b := NewPaperBroker(api)
	require.NoError(t, b.Fill(context.Background(), "99", 12.34))
	require.True(t, api.fillCalled)
	require.Equal(t, uint(99), api.fillID)
	require.InDelta(t, 12.34, api.fillPrice, 1e-9)
}

func TestPaperBroker_CancelDelegatesToCancelPaperOrder(t *testing.T) {
	api := &stubPaperAPI{}
	b := NewPaperBroker(api)
	require.NoError(t, b.Cancel(context.Background(), "42"))
	require.True(t, api.cancelCalled)
	require.Equal(t, uint(42), api.cancelID)
}

func TestPaperBroker_CancelPropagatesNotCancellable(t *testing.T) {
	api := &stubPaperAPI{cancelErr: data.ErrOrderNotCancellable}
	b := NewPaperBroker(api)
	err := b.Cancel(context.Background(), "1")
	require.ErrorIs(t, err, data.ErrOrderNotCancellable)
}

func TestExecutionService_CancelDelegatesToPort(t *testing.T) {
	port := &stubPort{}
	svc := NewExecutionService(port)
	require.ErrorIs(t, svc.Cancel(context.Background(), "9"), ErrCancelNotImplemented)
}
