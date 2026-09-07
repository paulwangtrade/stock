package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go-stock/backend/broker"

	"github.com/stretchr/testify/require"
)

type staticLocalSource struct {
	locals []LocalOrderSnapshot
	err    error
	calls  atomic.Int64
}

func (s *staticLocalSource) ListCandidates() ([]LocalOrderSnapshot, error) {
	s.calls.Add(1)
	if s.err != nil {
		return nil, s.err
	}
	out := make([]LocalOrderSnapshot, len(s.locals))
	copy(out, s.locals)
	return out, nil
}

type writeTrackingAdapter struct {
	status      *broker.OrderStatus
	err         error
	submitCalls atomic.Int64
	cancelCalls atomic.Int64
	queryCalls  atomic.Int64
}

func (a *writeTrackingAdapter) Submit(ctx context.Context, req *broker.SubmitRequest) (*broker.SubmitResponse, error) {
	_ = ctx
	_ = req
	a.submitCalls.Add(1)
	return nil, fmt.Errorf("api test: Submit forbidden")
}
func (a *writeTrackingAdapter) Cancel(ctx context.Context, clientOrderID string) error {
	_ = ctx
	_ = clientOrderID
	a.cancelCalls.Add(1)
	return fmt.Errorf("api test: Cancel forbidden")
}
func (a *writeTrackingAdapter) Connect(ctx context.Context) error    { _ = ctx; return nil }
func (a *writeTrackingAdapter) Disconnect(ctx context.Context) error { _ = ctx; return nil }
func (a *writeTrackingAdapter) Query(ctx context.Context, clientOrderID string) (*broker.OrderStatus, error) {
	_ = ctx
	a.queryCalls.Add(1)
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

var _ broker.BrokerAdapter = (*writeTrackingAdapter)(nil)

func apiLocal(id string) LocalOrderSnapshot {
	return LocalOrderSnapshot{
		OrderID:      id,
		OMSStatus:    "pending",
		BrokerStatus: "working",
		FilledQty:    30,
		LeavesQty:    70,
		AvgPrice:     10.0,
		UpdatedAt:    fixedNow().Add(-30 * time.Second),
	}
}

func newAPIHandler(t *testing.T, adapter *writeTrackingAdapter, locals []LocalOrderSnapshot) *Handler {
	t.Helper()
	provider := NewAdapterSnapshotProvider(adapter)
	provider.Now = fixedNow
	svc := NewService(provider, NewMetrics(), Options{Now: fixedNow()})
	h := NewHandler(svc, &staticLocalSource{locals: locals})
	h.Now = fixedNow
	return h
}

func decodeView(t *testing.T, rec *httptest.ResponseRecorder) BrokerReconcileView {
	t.Helper()
	var view BrokerReconcileView
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &view))
	return view
}

// P1: consistent observation → READY, no divergences.
func TestAPI_P1_NoDivergence(t *testing.T) {
	adapter := &writeTrackingAdapter{
		status: &broker.OrderStatus{
			Status: "working", FilledVolume: 30, LeavesQuantity: 70, AvgPrice: 10.0,
		},
	}
	h := newAPIHandler(t, adapter, []LocalOrderSnapshot{apiLocal("cid-1")})
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	view := decodeView(t, rec)
	require.Equal(t, ViewStatusReady, view.Status)
	require.Equal(t, 1, view.TotalOrders)
	require.Equal(t, 0, view.DivergenceCount)
	require.Empty(t, view.Divergences)
	require.Equal(t, fixedNow().UnixNano(), view.CheckedAt.UnixNano())
}

// P2: status mismatch → DEGRADED + divergence view.
func TestAPI_P2_StatusMismatch(t *testing.T) {
	adapter := &writeTrackingAdapter{
		status: &broker.OrderStatus{
			Status: "cancelled", FilledVolume: 30, LeavesQuantity: 0, AvgPrice: 10.0,
		},
	}
	h := newAPIHandler(t, adapter, []LocalOrderSnapshot{apiLocal("cid-2")})
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	view := decodeView(t, rec)
	require.Equal(t, ViewStatusDegraded, view.Status)
	require.Equal(t, 1, view.DivergenceCount)
	require.Equal(t, DivStatusMismatch, view.Divergences[0].Kind)
	require.Equal(t, "cid-2", view.Divergences[0].OrderID)
	require.Equal(t, "warn", view.Divergences[0].Severity)
}

// P3: missing order → ATTENTION + missing_order.
func TestAPI_P3_MissingOrder(t *testing.T) {
	adapter := &writeTrackingAdapter{
		err: fmt.Errorf("broker: stub order %q not found", "cid-3"),
	}
	h := newAPIHandler(t, adapter, []LocalOrderSnapshot{apiLocal("cid-3")})
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	view := decodeView(t, rec)
	require.Equal(t, ViewStatusAttention, view.Status)
	require.Equal(t, 1, view.DivergenceCount)
	require.Equal(t, DivMissingOrder, view.Divergences[0].Kind)
	require.Equal(t, "attention", view.Divergences[0].Severity)
}

// P4: query timeout → observation (status_mismatch; Core does not invent unknown_timeout).
// Timeout snapshot carries zero filled qty, so filled_qty_mismatch may also appear → ATTENTION.
func TestAPI_P4_QueryTimeout(t *testing.T) {
	adapter := &writeTrackingAdapter{err: context.DeadlineExceeded}
	h := newAPIHandler(t, adapter, []LocalOrderSnapshot{apiLocal("cid-4")})
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	view := decodeView(t, rec)
	require.Equal(t, ViewStatusAttention, view.Status)
	kinds := map[string]bool{}
	for _, d := range view.Divergences {
		kinds[d.Kind] = true
	}
	require.True(t, kinds[DivStatusMismatch], "query timeout maps to broker_status=timeout → status_mismatch")
	require.False(t, kinds[DivUnknownTimeout], "query timeout must not invent unknown_timeout")
}

// P5: non-GET → 405.
func TestAPI_P5_MethodNotAllowed(t *testing.T) {
	h := NewHandler(nil, nil)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, brokerReconcilePath, nil))
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code, method)
	}
}

// P6: API path never triggers Submit/Cancel (write ops).
func TestAPI_P6_NoWriteOperations(t *testing.T) {
	adapter := &writeTrackingAdapter{
		status: &broker.OrderStatus{
			Status: "working", FilledVolume: 30, LeavesQuantity: 70, AvgPrice: 10.0,
		},
	}
	locals := []LocalOrderSnapshot{apiLocal("cid-6")}
	h := newAPIHandler(t, adapter, locals)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, int64(0), adapter.submitCalls.Load(), "API must not Submit")
	require.Equal(t, int64(0), adapter.cancelCalls.Load(), "API must not Cancel")
	require.GreaterOrEqual(t, adapter.queryCalls.Load(), int64(1), "read Query is allowed via Service/Provider")
	require.Equal(t, "working", locals[0].BrokerStatus, "local snapshot must remain unchanged")
}

func TestAPI_EmptySource_NoAutoSnapshot(t *testing.T) {
	h := NewHandler(nil, nil)
	h.Now = fixedNow
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	view := decodeView(t, rec)
	require.Equal(t, ViewStatusReady, view.Status)
	require.Equal(t, 0, view.TotalOrders)
	require.Empty(t, view.Divergences)
}

func TestAPI_SourceError_ObservationError(t *testing.T) {
	h := NewHandler(NewService(nil, nil, Options{Now: fixedNow()}), &staticLocalSource{
		err: fmt.Errorf("candidate source unavailable"),
	})
	h.Now = fixedNow
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	view := decodeView(t, rec)
	require.Equal(t, ViewStatusObservationError, view.Status)
	require.Contains(t, view.ObservationError, "candidate source unavailable")
}

func TestAPI_ViewDoesNotExposeInternalSnapshots(t *testing.T) {
	adapter := &writeTrackingAdapter{
		status: &broker.OrderStatus{Status: "working", FilledVolume: 30, LeavesQuantity: 70, AvgPrice: 10},
	}
	h := newAPIHandler(t, adapter, []LocalOrderSnapshot{apiLocal("cid-x")})
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, brokerReconcilePath, nil))
	body := rec.Body.String()
	require.NotContains(t, body, "BrokerSnapshot")
	require.NotContains(t, body, "LocalOrderSnapshot")
	require.NotContains(t, body, `"oms_status"`)
	require.NotContains(t, body, `"filled_qty"`)
	require.Contains(t, body, `"status"`)
	require.Contains(t, body, `"divergences"`)
}
