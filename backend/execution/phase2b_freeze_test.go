package execution

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// Phase2-B Final Review：架构冻结防回退（只断言既有契约，不引入新能力）。

func TestPhase2B_OpenBuySourceHasNoDirectSubmitPaperOrder(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "..", "data", "paper_open_buy.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)
	require.NotContains(t, body, "SubmitPaperOrder(",
		"RunPaperOpenBuyOnce 生产路径不得直连 SubmitPaperOrder")
	require.Contains(t, body, "executor.ExecutePlanItem",
		"开盘路径必须经 PlanItemExecutor")
	require.Contains(t, body, "refuse silent SubmitPaperOrder fallback")
}

func TestPhase2B_ExecutePlanItemAlwaysPreTradeChecksBeforePort(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "x"}}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1}, nil // 故意不足
	})
	_, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sh600000", Side: "buy", LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, AutoFill: true})
	require.Error(t, err)
	require.Equal(t, data.PaperOrderRejectCashInsufficient, PreTradeRejectCode(err))
	require.Equal(t, 0, port.calls, "PreTradeCheck 失败后不得调用 ExecutionPort")
}

func TestPhase2B_CancelGoesThroughExecutionPort(t *testing.T) {
	port := &recordingCancelPort{}
	svc := NewExecutionService(port)
	require.NoError(t, svc.Cancel(context.Background(), "77"))
	require.Equal(t, 1, port.cancelCalls)
	require.Equal(t, "77", port.lastCancelID)
}

func TestPhase2B_PaperBrokerSurfaceFrozen(t *testing.T) {
	// 编译期：PaperBroker 实现 ExecutionPort（Submit/QueryOrder/Cancel）。
	var _ ExecutionPort = (*PaperBroker)(nil)
	// Fill 是显式会计包装，不属于 ExecutionPort；策略/风控/UI 不在此类型上。
	b := NewPaperBroker(&stubPaperAPI{})
	require.NotNil(t, b)
}

func TestPhase2B_StatusModelFrozen(t *testing.T) {
	require.Equal(t, "pending", data.PaperOrderStatusPending)
	require.Equal(t, "filled", data.PaperOrderStatusFilled)
	require.Equal(t, "rejected", data.PaperOrderStatusRejected)
	require.Equal(t, "cancelled", data.PaperOrderStatusCancelled)
	// 禁止业务终态 accepted
	require.NotEqual(t, "accepted", data.PaperOrderStatusPending)
	require.NotEqual(t, "accepted", data.PaperOrderStatusFilled)
	require.NotEqual(t, "accepted", data.PaperOrderStatusRejected)
	require.NotEqual(t, "accepted", data.PaperOrderStatusCancelled)
}

func TestPhase2B_EventContractsFrozen(t *testing.T) {
	// DB audit
	require.Equal(t, "order_submitted", data.PaperOrderEventSubmitted)
	require.Equal(t, "order_filled", data.PaperOrderEventFilled)
	require.Equal(t, "order_rejected", data.PaperOrderEventRejected)
	require.Equal(t, "order_cancelled", data.PaperOrderEventCancelled)

	// EventHub：无 cancelled
	hubTypes := []string{
		broker.EventOrderSubmitted,
		broker.EventOrderFilled,
		broker.EventOrderRejected,
		broker.EventFill,
	}
	for _, typ := range hubTypes {
		require.NotEqual(t, "order_cancelled", typ)
	}
	joined := strings.Join(hubTypes, ",")
	require.NotContains(t, joined, "cancelled")
}

func TestPhase2B_PaperIdentityFrozen(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	acc, err := api.ResetAccount(100_000)
	require.NoError(t, err)
	order, err := api.SubmitPaperOrder(data.PaperSubmitOrderReq{
		AccountID: acc.ID, StockCode: "sz000001", Side: "buy",
		Price: 10, Volume: 100, AutoFill: true,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(order.ClientOrderID, "paper_"))
	require.Equal(t, data.PaperExecBackendPaper, order.ExecBackend)
	require.Empty(t, order.BrokerOrderID)
	require.Empty(t, order.ExternalOrderID)
	require.Equal(t, data.PaperBrokerStatusPaper, order.BrokerStatus)
	require.NotEqual(t, "accepted", order.Status)
}

type recordingCancelPort struct {
	cancelCalls  int
	lastCancelID string
}

func (r *recordingCancelPort) Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error) {
	_ = ctx
	_ = intent
	return nil, ErrCancelNotImplemented
}

func (r *recordingCancelPort) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	_ = ctx
	_ = orderID
	return nil, ErrCancelNotImplemented
}

func (r *recordingCancelPort) Cancel(ctx context.Context, orderID string) error {
	_ = ctx
	r.cancelCalls++
	r.lastCancelID = orderID
	return nil
}
