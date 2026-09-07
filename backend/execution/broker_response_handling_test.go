package execution

// Phase6.5.7.4.4.1 Broker Response Handling MVP tests.
// Focus: RealBroker consumes SubmitResponse; keeps order on adapter error; audit capture.
// No real broker / migration / OMS enum expansion / TradePlan·Intent·Spec mutation.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/data"

	"github.com/stretchr/testify/require"
)

// programmableAdapter captures requests and returns scripted outcomes.
type programmableAdapter struct {
	mu       sync.Mutex
	captured []*broker.SubmitRequest
	resp     *broker.SubmitResponse
	err      error
}

func (a *programmableAdapter) Submit(ctx context.Context, req *broker.SubmitRequest) (*broker.SubmitResponse, error) {
	_ = ctx
	a.mu.Lock()
	defer a.mu.Unlock()
	if req != nil {
		cp := *req
		a.captured = append(a.captured, &cp)
	}
	if a.err != nil {
		return a.resp, a.err
	}
	if a.resp != nil {
		return a.resp, nil
	}
	return &broker.SubmitResponse{
		ClientOrderID: req.ClientOrderID,
		Status:        SubmitOutcomeAccepted,
		Message:       "test accepted",
	}, nil
}

func (a *programmableAdapter) Cancel(ctx context.Context, clientOrderID string) error {
	_ = ctx
	_ = clientOrderID
	return nil
}

func (a *programmableAdapter) Query(ctx context.Context, clientOrderID string) (*broker.OrderStatus, error) {
	_ = ctx
	return &broker.OrderStatus{ClientOrderID: clientOrderID, Status: data.BrokerStatusAccepted}, nil
}

func (a *programmableAdapter) Connect(ctx context.Context) error    { _ = ctx; return nil }
func (a *programmableAdapter) Disconnect(ctx context.Context) error { _ = ctx; return nil }

var _ broker.BrokerAdapter = (*programmableAdapter)(nil)

func testIntent() SubmitIntent {
	return SubmitIntent{
		AccountID: 1,
		StockCode: "sz000001",
		StockName: "平安银行",
		Side:      "buy",
		Price:     10.5,
		Volume:    200,
		SpecHash:  "deadbeefcafebabe0123456789abcdef",
	}
}

func TestBrokerResponse_Accepted(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeAccepted, Message: "ok"},
	}
	real := NewRealBrokerWithAdapter(fake)
	intent := testIntent()
	price, vol := intent.Price, intent.Volume

	order, err := real.Submit(context.Background(), intent)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusAccepted, order.BrokerStatus)
	require.Empty(t, order.BrokerOrderID)
	require.Equal(t, price, order.Price)
	require.Equal(t, vol, order.Volume)
	require.True(t, data.IsPaperOMSStatus(order.Status))

	require.Len(t, fake.captured, 1)
	require.Equal(t, intent.StockCode, fake.captured[0].StockCode)
	require.Equal(t, "buy", fake.captured[0].Side)
	require.InDelta(t, intent.Price, fake.captured[0].Price, 1e-9)
	require.Equal(t, intent.Volume, fake.captured[0].Volume)

	audit, ok := real.LastSubmitAudit(order.ClientOrderID)
	require.True(t, ok)
	require.Equal(t, order.ClientOrderID, audit.BrokerRequestID)
	require.False(t, audit.SubmitTime.IsZero())
	require.False(t, audit.ResponseTime.IsZero())
	require.True(t, !audit.ResponseTime.Before(audit.SubmitTime))
	require.Equal(t, intent.SpecHash, audit.SpecHash)
	require.Equal(t, SubmitOutcomeAccepted, audit.Outcome)
}

func TestBrokerResponse_StubPendingMapsToAccepted(t *testing.T) {
	real := NewRealBroker() // default StubAdapter returns Status=pending
	order, err := real.Submit(context.Background(), testIntent())
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusAccepted, order.BrokerStatus)
}

func TestBrokerResponse_RejectedViaStatus(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeRejected, Message: "risk reject"},
	}
	real := NewRealBrokerWithAdapter(fake)
	intent := testIntent()

	order, err := real.Submit(context.Background(), intent)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBrokerRejected)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusRejected, order.Status)
	require.Equal(t, data.BrokerStatusRejected, order.BrokerStatus)
	require.Equal(t, int64(0), order.LeavesQuantity)
	require.Equal(t, intent.Price, order.Price)
	require.Equal(t, intent.Volume, order.Volume)

	got, qerr := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, qerr)
	require.Equal(t, data.PaperOrderStatusRejected, got.Status)
}

func TestBrokerResponse_TimeoutViaStatus(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeTimeout, Message: "ack timeout"},
	}
	real := NewRealBrokerWithAdapter(fake)

	order, err := real.Submit(context.Background(), testIntent())
	require.ErrorIs(t, err, ErrBrokerTimeout)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusTimeout, order.BrokerStatus)
	require.Empty(t, order.BrokerOrderID)
}

func TestBrokerResponse_UnknownViaStatus(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeUnknown, Message: "wire reset"},
	}
	real := NewRealBrokerWithAdapter(fake)

	order, err := real.Submit(context.Background(), testIntent())
	require.ErrorIs(t, err, ErrBrokerUnknown)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusUnknown, order.BrokerStatus)
}

func TestBrokerResponse_AdapterErrorKeepsOrder_Unknown(t *testing.T) {
	fake := &programmableAdapter{err: errors.New("network reset by peer")}
	real := NewRealBrokerWithAdapter(fake)

	order, err := real.Submit(context.Background(), testIntent())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBrokerUnknown)
	require.NotNil(t, order, "adapter error must not delete created order")
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusUnknown, order.BrokerStatus)

	got, qerr := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, qerr)
	require.Equal(t, order.ID, got.ID)
}

func TestBrokerResponse_AdapterErrorKeepsOrder_Timeout(t *testing.T) {
	fake := &programmableAdapter{err: context.DeadlineExceeded}
	real := NewRealBrokerWithAdapter(fake)

	order, err := real.Submit(context.Background(), testIntent())
	require.ErrorIs(t, err, ErrBrokerTimeout)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, data.BrokerStatusTimeout, order.BrokerStatus)
}

func TestBrokerResponse_AdapterErrorKeepsOrder_Rejected(t *testing.T) {
	fake := &programmableAdapter{err: errors.New("broker reject: limit breached")}
	real := NewRealBrokerWithAdapter(fake)

	order, err := real.Submit(context.Background(), testIntent())
	require.ErrorIs(t, err, ErrBrokerRejected)
	require.NotNil(t, order)
	require.Equal(t, data.PaperOrderStatusRejected, order.Status)
	require.Equal(t, data.BrokerStatusRejected, order.BrokerStatus)
}

func TestBrokerResponse_AuditFields(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeAccepted},
	}
	real := NewRealBrokerWithAdapter(fake)
	intent := testIntent()
	before := time.Now()

	order, err := real.Submit(context.Background(), intent)
	require.NoError(t, err)
	after := time.Now()

	audit, ok := real.LastSubmitAudit(order.ClientOrderID)
	require.True(t, ok)
	require.NotEmpty(t, audit.BrokerRequestID)
	require.Equal(t, order.ClientOrderID, audit.BrokerRequestID)
	require.False(t, audit.SubmitTime.Before(before.Add(-time.Second)))
	require.False(t, audit.ResponseTime.After(after.Add(time.Second)))
	require.True(t, !audit.ResponseTime.Before(audit.SubmitTime))
	require.Equal(t, intent.SpecHash, audit.SpecHash)
	require.Equal(t, order.ID, audit.LocalOrderID)
}

func TestBrokerResponse_DoesNotMutateIntentPriceVolume(t *testing.T) {
	fake := &programmableAdapter{
		resp: &broker.SubmitResponse{Status: SubmitOutcomeRejected, Message: "no"},
	}
	real := NewRealBrokerWithAdapter(fake)
	intent := testIntent()
	wantPrice, wantVol := intent.Price, intent.Volume

	order, err := real.Submit(context.Background(), intent)
	require.Error(t, err)
	require.NotNil(t, order)
	require.InDelta(t, wantPrice, order.Price, 1e-9)
	require.Equal(t, wantVol, order.Volume)
	require.InDelta(t, wantPrice, intent.Price, 1e-9)
	require.Equal(t, wantVol, intent.Volume)
	require.InDelta(t, wantPrice, fake.captured[0].Price, 1e-9)
	require.Equal(t, wantVol, fake.captured[0].Volume)
}

func TestBrokerResponse_OMSEnumUnchanged(t *testing.T) {
	cases := []struct {
		name   string
		resp   *broker.SubmitResponse
		err    error
		oms    string
		broker string
	}{
		{"accepted", &broker.SubmitResponse{Status: "accepted"}, nil, data.PaperOrderStatusPending, data.BrokerStatusAccepted},
		{"rejected", &broker.SubmitResponse{Status: "rejected"}, nil, data.PaperOrderStatusRejected, data.BrokerStatusRejected},
		{"timeout", &broker.SubmitResponse{Status: "timeout"}, nil, data.PaperOrderStatusPending, data.BrokerStatusTimeout},
		{"unknown", &broker.SubmitResponse{Status: "unknown"}, nil, data.PaperOrderStatusPending, data.BrokerStatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &programmableAdapter{resp: tc.resp, err: tc.err}
			real := NewRealBrokerWithAdapter(fake)
			order, _ := real.Submit(context.Background(), testIntent())
			require.NotNil(t, order)
			require.Equal(t, tc.oms, order.Status)
			require.Equal(t, tc.broker, order.BrokerStatus)
			require.True(t, data.IsPaperOMSStatus(order.Status))
			require.NotEqual(t, "timeout", order.Status)
			require.NotEqual(t, "unknown", order.Status)
			require.NotEqual(t, "accepted", order.Status)
		})
	}
}
