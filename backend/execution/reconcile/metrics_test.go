package reconcile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// P1: consistent observation increments check_total only, no divergences.
func TestMetrics_P1_NoDivergence(t *testing.T) {
	m := NewMetrics()
	now := fixedNow()

	res := Compare(baseLocal(), baseBroker(), Options{Now: now})
	require.True(t, res.Consistent)

	m.RecordAt(res, now)

	snap := m.Snapshot()
	require.Equal(t, int64(1), snap.CheckTotal)
	require.Equal(t, int64(0), snap.DivergenceTotal[DivStatusMismatch])
	require.Equal(t, int64(0), snap.DivergenceTotal[DivMissingOrder])
	require.Equal(t, int64(0), snap.UnknownTimeoutCount)
	require.InDelta(t, 0.0, snap.CancelPendingAgeSeconds, 1e-9)
	require.Equal(t, now.UnixNano(), snap.LastReconcileTimestamp.UnixNano())
}

// P2: status_mismatch increments the correct per-type divergence counter.
func TestMetrics_P2_StatusMismatch(t *testing.T) {
	m := NewMetrics()
	now := fixedNow()

	broker := baseBroker()
	broker.Status = "cancelled"
	broker.BrokerStatus = "cancelled"
	res := Compare(baseLocal(), broker, Options{Now: now})
	require.True(t, res.HasKind(DivStatusMismatch))

	m.RecordAt(res, now)

	snap := m.Snapshot()
	require.Equal(t, int64(1), snap.CheckTotal)
	require.Equal(t, int64(1), snap.DivergenceTotal[DivStatusMismatch])
	require.Equal(t, int64(0), snap.DivergenceTotal[DivFilledQtyMismatch])
}

// P3: cancel_pending_timeout records the observed age in seconds.
func TestMetrics_P3_CancelPendingAge(t *testing.T) {
	m := NewMetrics()
	now := fixedNow()

	local := baseLocal()
	local.BrokerStatus = "cancel_pending"
	local.FilledQty = 0
	local.LeavesQty = 100
	local.AvgPrice = 0
	local.UpdatedAt = now.Add(-6 * time.Minute)

	broker := baseBroker()
	broker.Status = "cancelled"
	broker.BrokerStatus = "cancelled"
	broker.FilledQty = 0
	broker.LeavesQty = 0
	broker.AvgPrice = 0

	res := Compare(local, broker, Options{Now: now, CancelPendingTimeout: 5 * time.Minute})
	require.True(t, res.HasKind(DivCancelPendingTimeout))

	m.RecordAt(res, now)

	snap := m.Snapshot()
	require.Equal(t, int64(1), snap.DivergenceTotal[DivCancelPendingTimeout])
	require.InDelta(t, (6 * time.Minute).Seconds(), snap.CancelPendingAgeSeconds, 1e-6)
}

// P4: unknown_timeout increments the dedicated counter.
func TestMetrics_P4_UnknownTimeout(t *testing.T) {
	m := NewMetrics()
	now := fixedNow()

	local := baseLocal()
	local.BrokerStatus = "unknown"
	local.FilledQty = 0
	local.LeavesQty = 100
	local.AvgPrice = 0
	local.UpdatedAt = now.Add(-3 * time.Minute)

	broker := BrokerSnapshot{
		OrderID:      local.OrderID,
		Status:       "pending",
		BrokerStatus: "pending",
		FilledQty:    0,
		LeavesQty:    100,
		CheckedAt:    now,
	}

	res := Compare(local, broker, Options{Now: now, UnknownTimeout: 2 * time.Minute})
	require.True(t, res.HasKind(DivUnknownTimeout))

	m.RecordAt(res, now)

	snap := m.Snapshot()
	require.Equal(t, int64(1), snap.UnknownTimeoutCount)
	require.Equal(t, int64(1), snap.DivergenceTotal[DivUnknownTimeout])
}

// P5: repeated observation only accumulates metrics; inputs stay unchanged and
// no business state is touched (Record has no return / no side effect surface).
func TestMetrics_P5_RepeatedObservation(t *testing.T) {
	m := NewMetrics()
	now := fixedNow()

	local := baseLocal()
	broker := baseBroker()
	broker.Status = "cancelled"
	broker.BrokerStatus = "cancelled"

	res := Compare(local, broker, Options{Now: now})

	const n = 5
	for i := 0; i < n; i++ {
		m.RecordAt(res, now)
	}

	snap := m.Snapshot()
	require.Equal(t, int64(n), snap.CheckTotal)
	require.Equal(t, int64(n), snap.DivergenceTotal[DivStatusMismatch])

	// Observation-only: recorded result and its inputs are unchanged.
	require.Equal(t, "working", local.BrokerStatus)
	require.Equal(t, "cancelled", broker.BrokerStatus)
	require.Len(t, res.Divergences, 1)
	require.True(t, res.HasKind(DivStatusMismatch))
}

// Nil receiver is safe (defensive; observation must never panic the caller).
func TestMetrics_NilSafe(t *testing.T) {
	var m *Metrics
	require.NotPanics(t, func() {
		m.Record(ReconcileResult{})
		_ = m.Snapshot()
	})
}
