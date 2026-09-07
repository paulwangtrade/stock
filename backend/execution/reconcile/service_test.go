package reconcile

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/broker"

	"github.com/stretchr/testify/require"
)

// programmableQuerier is a BrokerAdapter that only implements Query for observation tests.
type programmableQuerier struct {
	status *broker.OrderStatus
	err    error
	calls  map[string]int
}

func (a *programmableQuerier) Submit(ctx context.Context, req *broker.SubmitRequest) (*broker.SubmitResponse, error) {
	_ = ctx
	_ = req
	return nil, fmt.Errorf("reconcile test: Submit not allowed")
}
func (a *programmableQuerier) Cancel(ctx context.Context, clientOrderID string) error {
	_ = ctx
	_ = clientOrderID
	return fmt.Errorf("reconcile test: Cancel not allowed")
}
func (a *programmableQuerier) Connect(ctx context.Context) error    { _ = ctx; return nil }
func (a *programmableQuerier) Disconnect(ctx context.Context) error { _ = ctx; return nil }

func (a *programmableQuerier) Query(ctx context.Context, clientOrderID string) (*broker.OrderStatus, error) {
	_ = ctx
	if a.calls == nil {
		a.calls = map[string]int{}
	}
	a.calls[clientOrderID]++
	if a.err != nil {
		return nil, a.err
	}
	if a.status == nil {
		return nil, fmt.Errorf("broker: stub order %q not found", clientOrderID)
	}
	cp := *a.status
	cp.ClientOrderID = clientOrderID
	return &cp, nil
}

var _ broker.BrokerAdapter = (*programmableQuerier)(nil)

// staticProvider returns a fixed BrokerSnapshot (or error) for Service tests.
type staticProvider struct {
	byID map[string]BrokerSnapshot
	err  error
}

func (p *staticProvider) Snapshot(orderID string) (BrokerSnapshot, error) {
	if p.err != nil {
		return BrokerSnapshot{}, p.err
	}
	if p.byID == nil {
		return BrokerSnapshot{OrderID: orderID, Missing: true, CheckedAt: fixedNow()}, nil
	}
	s, ok := p.byID[orderID]
	if !ok {
		return BrokerSnapshot{OrderID: orderID, Missing: true, CheckedAt: fixedNow()}, nil
	}
	return s, nil
}

func serviceLocal() LocalOrderSnapshot {
	return LocalOrderSnapshot{
		OrderID:       "cid-1",
		BrokerOrderID: "brk-1",
		OMSStatus:     "pending",
		BrokerStatus:  "working",
		FilledQty:     30,
		LeavesQty:     70,
		AvgPrice:      10.0,
		UpdatedAt:     fixedNow().Add(-30 * time.Second),
	}
}

// P1: local == broker → CONSISTENT; metrics check_total increments.
func TestService_P1_LocalEqualsBroker(t *testing.T) {
	now := fixedNow()
	adapter := &programmableQuerier{
		status: &broker.OrderStatus{
			BrokerOrderID:  "brk-1",
			Status:         "working",
			FilledVolume:   30,
			LeavesQuantity: 70,
			AvgPrice:       10.0,
		},
	}
	provider := NewAdapterSnapshotProvider(adapter)
	provider.Now = func() time.Time { return now }
	metrics := NewMetrics()
	svc := NewService(provider, metrics, Options{Now: now})

	local := serviceLocal()
	got := svc.Reconcile(local)

	require.True(t, got.Consistent)
	require.Equal(t, ClassConsistent, got.Classification)
	require.Empty(t, got.Divergences)
	require.Equal(t, int64(1), metrics.Snapshot().CheckTotal)
	require.Equal(t, "working", local.BrokerStatus, "service must not mutate local")
	require.Equal(t, 1, adapter.calls["cid-1"])
}

// P2: broker filled_qty mismatch → filled_qty_mismatch divergence.
func TestService_P2_FilledQtyMismatch(t *testing.T) {
	now := fixedNow()
	adapter := &programmableQuerier{
		status: &broker.OrderStatus{
			BrokerOrderID:  "brk-1",
			Status:         "working",
			FilledVolume:   50,
			LeavesQuantity: 50,
			AvgPrice:       10.0,
		},
	}
	provider := NewAdapterSnapshotProvider(adapter)
	provider.Now = func() time.Time { return now }
	svc := NewService(provider, nil, Options{Now: now})

	got := svc.Reconcile(serviceLocal())

	require.False(t, got.Consistent)
	require.True(t, got.HasKind(DivFilledQtyMismatch))
	require.Equal(t, ClassDivergent, got.Classification)
}

// P3: broker not found → missing_order.
func TestService_P3_BrokerNotFound(t *testing.T) {
	now := fixedNow()
	adapter := &programmableQuerier{
		err: fmt.Errorf("broker: stub order %q not found", "cid-1"),
	}
	provider := NewAdapterSnapshotProvider(adapter)
	provider.Now = func() time.Time { return now }
	metrics := NewMetrics()
	svc := NewService(provider, metrics, Options{Now: now})

	got := svc.Reconcile(serviceLocal())

	require.True(t, got.HasKind(DivMissingOrder))
	require.Equal(t, ClassUnverifiable, got.Classification)
	require.Equal(t, int64(1), metrics.Snapshot().DivergenceTotal[DivMissingOrder])
}

// P4: query timeout → observation timeout snapshot → status_mismatch (no order update / no cancel).
func TestService_P4_QueryTimeout(t *testing.T) {
	now := fixedNow()
	adapter := &programmableQuerier{err: context.DeadlineExceeded}
	provider := NewAdapterSnapshotProvider(adapter)
	provider.Now = func() time.Time { return now }
	svc := NewService(provider, NewMetrics(), Options{Now: now})

	local := serviceLocal()
	got := svc.Reconcile(local)

	require.True(t, got.HasKind(DivStatusMismatch), "local=working vs query-timeout=timeout")
	require.Equal(t, "working", local.BrokerStatus)
	require.Equal(t, ClassDivergent, got.Classification)
	// Provider must not have called Cancel/Submit (programmableQuerier returns error if called).
}

// P5: multiple orders are observed independently; one missing does not taint another.
func TestService_P5_MultipleOrdersIsolation(t *testing.T) {
	now := fixedNow()
	provider := &staticProvider{
		byID: map[string]BrokerSnapshot{
			"a": {
				OrderID: "a", Status: "working", BrokerStatus: "working",
				FilledQty: 10, LeavesQty: 0, AvgPrice: 1, CheckedAt: now,
			},
			// "b" intentionally missing from map → Missing
		},
	}
	metrics := NewMetrics()
	svc := NewService(provider, metrics, Options{Now: now})

	locals := []LocalOrderSnapshot{
		{
			OrderID: "a", BrokerStatus: "working", FilledQty: 10, LeavesQty: 0,
			AvgPrice: 1, UpdatedAt: now.Add(-time.Second),
		},
		{
			OrderID: "b", BrokerStatus: "working", FilledQty: 0, LeavesQty: 100,
			UpdatedAt: now.Add(-time.Second),
		},
	}
	results := svc.ReconcileMany(locals)

	require.Len(t, results, 2)
	require.True(t, results[0].Consistent, "order a must stay consistent")
	require.True(t, results[1].HasKind(DivMissingOrder), "order b missing_order")
	require.False(t, results[0].HasKind(DivMissingOrder), "order a must not inherit b's divergence")

	snap := metrics.Snapshot()
	require.Equal(t, int64(2), snap.CheckTotal)
	require.Equal(t, int64(1), snap.DivergenceTotal[DivMissingOrder])
}

func TestAdapterSnapshotProvider_MapOrderStatus(t *testing.T) {
	now := fixedNow()
	got := MapOrderStatus("cid", &broker.OrderStatus{
		BrokerOrderID:  "brk",
		Status:         "partial",
		FilledVolume:   40,
		LeavesQuantity: 60,
		AvgPrice:       9.5,
	}, now)
	require.Equal(t, "cid", got.OrderID)
	require.Equal(t, "brk", got.BrokerOrderID)
	require.Equal(t, "partial", got.Status)
	require.Equal(t, "partially_filled", got.BrokerStatus)
	require.Equal(t, int64(40), got.FilledQty)
	require.False(t, got.Missing)
}

func TestAdapterSnapshotProvider_UnknownError(t *testing.T) {
	now := fixedNow()
	adapter := &programmableQuerier{err: fmt.Errorf("broker: connection reset")}
	provider := NewAdapterSnapshotProvider(adapter)
	provider.Now = func() time.Time { return now }

	snap, err := provider.Snapshot("cid-x")
	require.NoError(t, err, "query errors convert to observation, not hard failure")
	require.Equal(t, "unknown", snap.BrokerStatus)
	require.False(t, snap.Missing)
}

func TestService_MetricsDoNotAffectResult(t *testing.T) {
	now := fixedNow()
	provider := &staticProvider{
		byID: map[string]BrokerSnapshot{
			"cid-1": {
				OrderID: "cid-1", Status: "working", BrokerStatus: "working",
				FilledQty: 30, LeavesQty: 70, AvgPrice: 10, CheckedAt: now,
			},
		},
	}
	withMetrics := NewService(provider, NewMetrics(), Options{Now: now})
	without := NewService(provider, nil, Options{Now: now})

	a := withMetrics.Reconcile(serviceLocal())
	b := without.Reconcile(serviceLocal())
	require.Equal(t, a.Consistent, b.Consistent)
	require.Equal(t, a.Classification, b.Classification)
	require.Equal(t, a.Divergences, b.Divergences)
}
