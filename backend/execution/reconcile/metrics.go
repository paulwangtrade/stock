package reconcile

// Reconcile Metrics (Phase 6.5.9.2.1.2).
//
// Observation-only, in-process atomic counters. Input is a ReconcileResult;
// output is a MetricsSnapshot. Recording metrics never triggers retry / cancel
// / repair / order update, and never mutates the comparator or its inputs.
// No Prometheus dependency; no persistence; no migration.

import (
	"math"
	"strings"
	"sync/atomic"
	"time"
)

// MetricsSnapshot is an immutable read-only view of reconcile observation metrics.
type MetricsSnapshot struct {
	// reconcile_check_total: number of Compare results recorded.
	CheckTotal int64 `json:"reconcile_check_total"`
	// reconcile_divergence_total{type}: per-kind divergence counters.
	DivergenceTotal map[string]int64 `json:"reconcile_divergence_total"`
	// cancel_pending_age_seconds: age of the most recent cancel_pending_timeout observation.
	CancelPendingAgeSeconds float64 `json:"cancel_pending_age_seconds"`
	// unknown_timeout_count: number of unknown_timeout observations.
	UnknownTimeoutCount int64 `json:"unknown_timeout_count"`
	// last_reconcile_timestamp: wall clock of the most recent recorded observation.
	LastReconcileTimestamp time.Time `json:"last_reconcile_timestamp"`
}

// Metrics accumulates reconcile observation results. Zero value is ready to use
// and is safe for concurrent Record / Snapshot calls.
type Metrics struct {
	checkTotal atomic.Int64

	divMissingOrder         atomic.Int64
	divStatusMismatch       atomic.Int64
	divFilledQtyMismatch    atomic.Int64
	divAvgPriceMismatch     atomic.Int64
	divCancelPendingTimeout atomic.Int64
	divUnknownTimeout       atomic.Int64

	unknownTimeoutCount         atomic.Int64
	cancelPendingAgeSecondsBits atomic.Uint64 // float64 bits; last observed gauge
	lastReconcileUnixNano       atomic.Int64
}

// NewMetrics returns a ready-to-use Metrics accumulator.
func NewMetrics() *Metrics { return &Metrics{} }

// Record ingests one ReconcileResult and updates observation counters using the
// current wall clock. It is report-only: no runtime / business state changes.
func (m *Metrics) Record(result ReconcileResult) {
	m.RecordAt(result, time.Now())
}

// RecordAt is Record with an injectable clock for deterministic tests.
func (m *Metrics) RecordAt(result ReconcileResult, now time.Time) {
	if m == nil {
		return
	}
	m.checkTotal.Add(1)
	if !now.IsZero() {
		m.lastReconcileUnixNano.Store(now.UnixNano())
	}
	for _, d := range result.Divergences {
		switch d.Kind {
		case DivMissingOrder:
			m.divMissingOrder.Add(1)
		case DivStatusMismatch:
			m.divStatusMismatch.Add(1)
		case DivFilledQtyMismatch:
			m.divFilledQtyMismatch.Add(1)
		case DivAvgPriceMismatch:
			m.divAvgPriceMismatch.Add(1)
		case DivCancelPendingTimeout:
			m.divCancelPendingTimeout.Add(1)
			if secs, ok := parseAgeSeconds(d.Detail); ok {
				storeAtomicFloat64(&m.cancelPendingAgeSecondsBits, secs)
			}
		case DivUnknownTimeout:
			m.divUnknownTimeout.Add(1)
			m.unknownTimeoutCount.Add(1)
		}
	}
}

// Snapshot returns a point-in-time copy of the accumulated metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{DivergenceTotal: map[string]int64{}}
	}
	out := MetricsSnapshot{
		CheckTotal: m.checkTotal.Load(),
		DivergenceTotal: map[string]int64{
			DivMissingOrder:         m.divMissingOrder.Load(),
			DivStatusMismatch:       m.divStatusMismatch.Load(),
			DivFilledQtyMismatch:    m.divFilledQtyMismatch.Load(),
			DivAvgPriceMismatch:     m.divAvgPriceMismatch.Load(),
			DivCancelPendingTimeout: m.divCancelPendingTimeout.Load(),
			DivUnknownTimeout:       m.divUnknownTimeout.Load(),
		},
		CancelPendingAgeSeconds: math.Float64frombits(m.cancelPendingAgeSecondsBits.Load()),
		UnknownTimeoutCount:     m.unknownTimeoutCount.Load(),
	}
	if ns := m.lastReconcileUnixNano.Load(); ns > 0 {
		out.LastReconcileTimestamp = time.Unix(0, ns)
	}
	return out
}

// parseAgeSeconds extracts the "age=<duration>" token from a Divergence.Detail.
// It reads only; it does not depend on comparator internals beyond the format.
func parseAgeSeconds(detail string) (float64, bool) {
	for _, tok := range strings.Fields(detail) {
		if strings.HasPrefix(tok, "age=") {
			d, err := time.ParseDuration(strings.TrimPrefix(tok, "age="))
			if err != nil {
				return 0, false
			}
			return d.Seconds(), true
		}
	}
	return 0, false
}

func storeAtomicFloat64(addr *atomic.Uint64, v float64) {
	addr.Store(math.Float64bits(v))
}
