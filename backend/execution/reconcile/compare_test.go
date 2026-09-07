package reconcile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fixedNow() time.Time {
	return time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
}

func baseLocal() LocalOrderSnapshot {
	return LocalOrderSnapshot{
		OrderID:       "1",
		BrokerOrderID: "real_stub_x",
		OMSStatus:     "pending",
		BrokerStatus:  "working",
		FilledQty:     30,
		LeavesQty:     70,
		AvgPrice:      10.0,
		UpdatedAt:     fixedNow().Add(-30 * time.Second),
	}
}

func baseBroker() BrokerSnapshot {
	return BrokerSnapshot{
		OrderID:       "1",
		BrokerOrderID: "real_stub_x",
		Status:        "working",
		BrokerStatus:  "working",
		FilledQty:     30,
		LeavesQty:     70,
		AvgPrice:      10.0,
		CheckedAt:     fixedNow(),
	}
}

func TestCompare_P1_FullyConsistent(t *testing.T) {
	local := baseLocal()
	broker := baseBroker()

	got := Compare(local, broker, Options{Now: fixedNow()})

	require.True(t, got.Consistent)
	require.Equal(t, ClassConsistent, got.Classification)
	require.Empty(t, got.Divergences)
	// Inputs must remain untouched (observation-only).
	require.Equal(t, int64(30), local.FilledQty)
	require.Equal(t, "working", local.BrokerStatus)
	require.Equal(t, int64(30), broker.FilledQty)
	require.Equal(t, "working", broker.BrokerStatus)
}

func TestCompare_P2_StatusMismatch(t *testing.T) {
	local := baseLocal()
	broker := baseBroker()
	broker.Status = "cancelled"
	broker.BrokerStatus = "cancelled"

	got := Compare(local, broker, Options{Now: fixedNow()})

	require.False(t, got.Consistent)
	require.Equal(t, ClassDivergent, got.Classification)
	require.True(t, got.HasKind(DivStatusMismatch))
	require.False(t, got.HasKind(DivFilledQtyMismatch))
	require.Equal(t, "working", local.BrokerStatus, "comparator must not mutate local")
	require.Equal(t, "cancelled", broker.BrokerStatus, "comparator must not mutate broker")
}

func TestCompare_P3_FilledQtyMismatch(t *testing.T) {
	local := baseLocal()
	broker := baseBroker()
	broker.FilledQty = 50
	broker.LeavesQty = 50

	got := Compare(local, broker, Options{Now: fixedNow()})

	require.False(t, got.Consistent)
	require.True(t, got.HasKind(DivFilledQtyMismatch))
	require.Equal(t, ClassDivergent, got.Classification)
	require.Equal(t, int64(30), local.FilledQty)
	require.Equal(t, int64(50), broker.FilledQty)
}

func TestCompare_P4_AvgPriceMismatch(t *testing.T) {
	local := baseLocal()
	broker := baseBroker()
	broker.AvgPrice = 10.5

	got := Compare(local, broker, Options{Now: fixedNow()})

	require.False(t, got.Consistent)
	require.True(t, got.HasKind(DivAvgPriceMismatch))
	require.False(t, got.HasKind(DivFilledQtyMismatch))
	require.InDelta(t, 10.0, local.AvgPrice, 1e-12)
	require.InDelta(t, 10.5, broker.AvgPrice, 1e-12)
}

func TestCompare_P5_MissingOrder(t *testing.T) {
	local := baseLocal()
	broker := BrokerSnapshot{
		OrderID:   local.OrderID,
		CheckedAt: fixedNow(),
		Missing:   true,
	}

	got := Compare(local, broker, Options{Now: fixedNow()})

	require.False(t, got.Consistent)
	require.True(t, got.HasKind(DivMissingOrder))
	require.Equal(t, ClassUnverifiable, got.Classification)
	require.False(t, got.HasKind(DivStatusMismatch), "missing order skips channel field compares")
	require.False(t, got.HasKind(DivFilledQtyMismatch))
	require.Equal(t, int64(30), local.FilledQty)
}

func TestCompare_P6_CancelPendingTimeout(t *testing.T) {
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
	broker.CheckedAt = now

	got := Compare(local, broker, Options{
		Now:                  now,
		CancelPendingTimeout: 5 * time.Minute,
	})

	require.False(t, got.Consistent)
	require.True(t, got.HasKind(DivCancelPendingTimeout))
	require.True(t, got.HasKind(DivStatusMismatch), "stub-like cancel_pending vs cancelled is still reported")
	require.Equal(t, ClassStale, got.Classification)
	require.Equal(t, "cancel_pending", local.BrokerStatus)
	require.Equal(t, "cancelled", broker.BrokerStatus)
}

func TestCompare_UnknownTimeout(t *testing.T) {
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

	got := Compare(local, broker, Options{
		Now:            now,
		UnknownTimeout: 2 * time.Minute,
	})

	require.True(t, got.HasKind(DivUnknownTimeout))
	require.True(t, got.HasKind(DivStatusMismatch))
	require.Equal(t, ClassStale, got.Classification)
}

func TestNormalizeChannelStatus(t *testing.T) {
	require.Equal(t, "working", NormalizeChannelStatus("working"))
	require.Equal(t, "cancelled", NormalizeChannelStatus("canceled"))
	require.Equal(t, "partially_filled", NormalizeChannelStatus("partial"))
	require.Equal(t, "unknown", NormalizeChannelStatus("weird"))
}
