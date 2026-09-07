package reconcile

// Reconcile Service + Adapter Snapshot Builder (Phase 6.5.9.2.1.3.1).
//
// Observation orchestration only:
//   LocalOrderSnapshot ⊕ BrokerSnapshotProvider.Snapshot → Compare → ReconcileResult
// optional Metrics.Record (must not alter the result).
//
// Channel observation source is BrokerAdapter.Query (OrderStatus), NOT
// ExecutionPort.QueryOrder (OMS TradeOrder). RealBroker.QueryOrder is local OMS
// and must not be used as BrokerSnapshot — that would make local≡broker tautology.
//
// Never writes Order / Fill / Position / TradePlan / Frozen Spec / OMS.
// Does not modify BrokerAdapter, Execution paths, or comparator.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/broker"
)

// BrokerSnapshotProvider builds a read-only BrokerSnapshot for one order key.
// Implementations must not mutate OMS / Order / Fill / Position.
type BrokerSnapshotProvider interface {
	Snapshot(orderID string) (BrokerSnapshot, error)
}

// Service orchestrates one-order observation: provider → Compare → optional metrics.
type Service struct {
	Provider BrokerSnapshotProvider
	Metrics  *Metrics
	Options  Options
}

// NewService returns a ready-to-use observation service.
func NewService(provider BrokerSnapshotProvider, metrics *Metrics, opt Options) *Service {
	return &Service{Provider: provider, Metrics: metrics, Options: opt}
}

// Reconcile compares local observation against a channel snapshot.
// Metrics.Record is called after Compare; it never feeds back into the result.
func (s *Service) Reconcile(local LocalOrderSnapshot) ReconcileResult {
	opt := s.options()
	now := opt.Now

	brokerSnap := BrokerSnapshot{
		OrderID:      local.OrderID,
		Status:       "unknown",
		BrokerStatus: "unknown",
		CheckedAt:    now,
	}
	if s != nil && s.Provider != nil {
		got, err := s.Provider.Snapshot(local.OrderID)
		if err != nil {
			// Unexpected provider failure → observation unknown (no order update).
			brokerSnap = BrokerSnapshot{
				OrderID:      local.OrderID,
				Status:       "unknown",
				BrokerStatus: "unknown",
				CheckedAt:    now,
			}
		} else {
			brokerSnap = got
			if brokerSnap.OrderID == "" {
				brokerSnap.OrderID = local.OrderID
			}
			if brokerSnap.CheckedAt.IsZero() {
				brokerSnap.CheckedAt = now
			}
		}
	}

	result := Compare(local, brokerSnap, opt)
	if s != nil && s.Metrics != nil {
		s.Metrics.RecordAt(result, now)
	}
	return result
}

// ReconcileMany observes each local snapshot independently (no shared mutation).
func (s *Service) ReconcileMany(locals []LocalOrderSnapshot) []ReconcileResult {
	out := make([]ReconcileResult, len(locals))
	for i := range locals {
		out[i] = s.Reconcile(locals[i])
	}
	return out
}

func (s *Service) options() Options {
	if s == nil {
		return Options{}.withDefaults()
	}
	return s.Options.withDefaults()
}

// ---------------------------------------------------------------------------
// AdapterSnapshotProvider — BrokerAdapter.Query → BrokerSnapshot
// ---------------------------------------------------------------------------

// AdapterSnapshotProvider reads BrokerAdapter.Query and maps OrderStatus to
// BrokerSnapshot. Query errors become observation snapshots (Missing / timeout /
// unknown); they never update orders.
type AdapterSnapshotProvider struct {
	Adapter broker.BrokerAdapter
	// Ctx is used for Query; nil → context.Background().
	Ctx context.Context
	// Now overrides wall clock; nil → time.Now.
	Now func() time.Time
}

// NewAdapterSnapshotProvider wraps a BrokerAdapter for observation-only snapshots.
func NewAdapterSnapshotProvider(adapter broker.BrokerAdapter) *AdapterSnapshotProvider {
	return &AdapterSnapshotProvider{Adapter: adapter}
}

// Snapshot calls BrokerAdapter.Query and converts the response or error into a
// BrokerSnapshot. Always observation-only.
func (p *AdapterSnapshotProvider) Snapshot(orderID string) (BrokerSnapshot, error) {
	if p == nil || p.Adapter == nil {
		return BrokerSnapshot{}, fmt.Errorf("reconcile: nil BrokerAdapter")
	}
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	ctx := p.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	cid := strings.TrimSpace(orderID)
	st, err := p.Adapter.Query(ctx, cid)
	if err != nil {
		return mapQueryError(cid, err, now), nil
	}
	if st == nil {
		return BrokerSnapshot{
			OrderID:   cid,
			Missing:   true,
			CheckedAt: now,
		}, nil
	}
	return MapOrderStatus(cid, st, now), nil
}

// MapOrderStatus converts a channel OrderStatus into a BrokerSnapshot.
// It does not write OMS state.
func MapOrderStatus(orderID string, st *broker.OrderStatus, checkedAt time.Time) BrokerSnapshot {
	if st == nil {
		return BrokerSnapshot{OrderID: orderID, Missing: true, CheckedAt: checkedAt}
	}
	status := strings.TrimSpace(st.Status)
	return BrokerSnapshot{
		OrderID:       orderID,
		BrokerOrderID: strings.TrimSpace(st.BrokerOrderID),
		Status:        status,
		BrokerStatus:  NormalizeChannelStatus(status),
		FilledQty:     st.FilledVolume,
		LeavesQty:     st.LeavesQuantity,
		AvgPrice:      st.AvgPrice,
		CheckedAt:     checkedAt,
	}
}

// mapQueryError turns Query failures into observation snapshots.
// not found → Missing; timeout → status=timeout; else → status=unknown.
func mapQueryError(orderID string, err error, checkedAt time.Time) BrokerSnapshot {
	switch {
	case isNotFoundError(err):
		return BrokerSnapshot{
			OrderID:   orderID,
			Missing:   true,
			CheckedAt: checkedAt,
		}
	case isTimeoutError(err):
		return BrokerSnapshot{
			OrderID:      orderID,
			Status:       "timeout",
			BrokerStatus: "timeout",
			CheckedAt:    checkedAt,
		}
	default:
		return BrokerSnapshot{
			OrderID:      orderID,
			Status:       "unknown",
			BrokerStatus: "unknown",
			CheckedAt:    checkedAt,
		}
	}
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "notfound")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

var _ BrokerSnapshotProvider = (*AdapterSnapshotProvider)(nil)
