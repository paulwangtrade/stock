package allocationengine

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func testSnap() *PortfolioSnapshot {
	return &PortfolioSnapshot{
		Found:        true,
		Equity:       1_000_000,
		Cash:         500_000,
		ReservedCash: 0,
		Exposure:     200_000,
	}
}

func TestAllocate_Deterministic(t *testing.T) {
	t.Parallel()
	budget := AllocationBudget{
		AvailableCash: 300_000, AvailableCapital: 300_000, Binding: BindingCash, PolicyGrossPct: 0.85,
	}
	in := EngineInput{
		Selection: SelectionResult{
			Selected: []string{"sz000001", "sz000002"},
			Waitlist: []string{"sz000003"},
		},
		Snapshot: testSnap(),
		Budget:   &budget,
		Resolved: ResolvedConstraints{MaxSingleWeight: 0.20, MinOrderAmount: 1_000},
	}
	a := Allocate(in)
	b := Allocate(in)
	ja, err := json.Marshal(a)
	require.NoError(t, err)
	jb, err := json.Marshal(b)
	require.NoError(t, err)
	require.Equal(t, string(ja), string(jb))
	require.Equal(t, SchemaVersion, a.SchemaVersion)
}

func TestAllocate_AvailableCapitalAndReserve(t *testing.T) {
	t.Parallel()
	snap := &PortfolioSnapshot{Found: true, Equity: 1_000_000, Cash: 100_000, ReservedCash: 0, Exposure: 0}
	got := ComputeBudget(snap, ResolvedConstraints{MaxGrossExposurePct: 0.85, ReserveCashRatio: 0.40})
	require.InDelta(t, 40_000, got.ReserveCash, 1e-6)
	require.InDelta(t, 60_000, got.AvailableCash, 1e-6)
	require.InDelta(t, 60_000, got.AvailableCapital, 1e-6)

	// Gross binding
	snap2 := &PortfolioSnapshot{Found: true, Equity: 1_000_000, Cash: 900_000, Exposure: 800_000}
	g := ComputeBudget(snap2, ResolvedConstraints{MaxGrossExposurePct: 0.85})
	require.InDelta(t, 50_000, g.AvailableCapital, 1e-6)
	require.Equal(t, BindingGross, g.Binding)
}

func TestAllocate_EqualWeight_WaitlistKept(t *testing.T) {
	t.Parallel()
	budget := AllocationBudget{AvailableCapital: 200_000, Binding: BindingOK}
	got := Allocate(EngineInput{
		Selection: SelectionResult{
			Selected: []string{"sz000001", "sz000002"},
			Waitlist: []string{"sz000003"},
		},
		Snapshot: testSnap(),
		Budget:   &budget,
		Resolved: ResolvedConstraints{MaxSingleWeight: 0.50},
	})
	require.Equal(t, MethodEqualWeight, got.Method)
	require.Equal(t, 100_000.0, got.UniformAmount)
	require.Len(t, got.Allocated, 2)
	require.Len(t, got.Waitlist, 1)
	require.Len(t, got.Items, 3)
	require.Equal(t, "sz000001", got.Allocated[0].StockCode)
	require.Equal(t, 100_000.0, got.Allocated[0].TargetAmount)
	require.Equal(t, ReasonEqualSplit, got.Allocated[0].AllocationReason)
	require.Equal(t, int64(0), got.Allocated[0].TargetQuantity)
	require.True(t, got.Allocated[0].InAllocationSet)
	require.Equal(t, "sz000003", got.Waitlist[0].StockCode)
	require.Equal(t, 100_000.0, got.Waitlist[0].TargetAmount)
	require.Equal(t, ReasonWaitlistUniform, got.Waitlist[0].AllocationReason)
	require.False(t, got.Waitlist[0].InAllocationSet)
}

func TestAllocate_SinglePositionCap(t *testing.T) {
	t.Parallel()
	budget := AllocationBudget{AvailableCapital: 800_000}
	got := Allocate(EngineInput{
		Selection: SelectionResult{Selected: []string{"a", "b"}},
		Snapshot:  &PortfolioSnapshot{Found: true, Equity: 1_000_000},
		Budget:    &budget,
		Resolved:  ResolvedConstraints{MaxSingleWeight: 0.20},
	})
	require.Equal(t, 200_000.0, got.UniformAmount)
	require.Equal(t, ReasonCappedSingleWeight, got.Allocated[0].AllocationReason)
	require.Equal(t, 200_000.0, got.Allocated[0].TargetAmount)
}

func TestAllocate_NilBudget_ComputesFromResolved(t *testing.T) {
	t.Parallel()
	got := Allocate(EngineInput{
		Selection: SelectionResult{Selected: []string{"x", "y"}},
		Snapshot:  testSnap(),
		Budget:    nil,
		Resolved:  ResolvedConstraints{MaxGrossExposurePct: 0.85, MaxSingleWeight: 0.50, ReserveCashRatio: 0.1},
	})
	require.Greater(t, got.Budget.ReserveCash, 0.0)
	require.Greater(t, got.Budget.AvailableCapital, 0.0)
	require.Len(t, got.Allocated, 2)
	require.InDelta(t, got.UniformAmount, got.Allocated[0].TargetAmount, 1e-9)
}

func TestAllocate_BlockNewEntries_ZeroCapital(t *testing.T) {
	t.Parallel()
	got := Allocate(EngineInput{
		Selection: SelectionResult{Selected: []string{"a"}, Waitlist: []string{"w"}},
		Snapshot:  testSnap(),
		Budget:    nil,
		Resolved:  ResolvedConstraints{MaxGrossExposurePct: 0.85, BlockNewEntries: true},
	})
	require.Equal(t, BindingBlocked, got.Budget.Binding)
	require.Equal(t, 0.0, got.Budget.AvailableCapital)
	require.Equal(t, 0.0, got.Allocated[0].TargetAmount)
}

func TestAllocate_NoAccount(t *testing.T) {
	t.Parallel()
	budget := AllocationBudget{AvailableCapital: 999_999}
	got := Allocate(EngineInput{
		Selection: SelectionResult{Selected: []string{"a"}, Waitlist: []string{"b"}},
		Snapshot:  &PortfolioSnapshot{Found: false},
		Budget:    &budget,
		Resolved:  ResolvedConstraints{},
	})
	require.Equal(t, BindingNoAccount, got.Budget.Binding)
	require.Equal(t, ReasonNoAccount, got.Allocated[0].AllocationReason)
	require.Equal(t, ReasonWaitlistUniform, got.Waitlist[0].AllocationReason)
}

func TestAllocate_EmptySelected_WaitlistPreserved(t *testing.T) {
	t.Parallel()
	budget := AllocationBudget{AvailableCapital: 100_000}
	got := Allocate(EngineInput{
		Selection: SelectionResult{Waitlist: []string{"w1", "w2"}},
		Snapshot:  testSnap(),
		Budget:    &budget,
		Resolved:  ResolvedConstraints{},
	})
	require.Empty(t, got.Allocated)
	require.Len(t, got.Waitlist, 2)
	require.Equal(t, ReasonNoAllocationSet, got.Waitlist[0].AllocationReason)
	require.Equal(t, 0.0, got.Waitlist[0].TargetAmount)
}

func TestAllocate_DoesNotMutateInputs(t *testing.T) {
	t.Parallel()
	sel := SelectionResult{Selected: []string{"a", "b"}, Waitlist: []string{"c"}}
	snap := testSnap()
	cashBefore := snap.Cash
	budget := AllocationBudget{AvailableCapital: 100_000}
	_ = Allocate(EngineInput{
		Selection: sel, Snapshot: snap, Budget: &budget,
		Resolved: ResolvedConstraints{MaxSingleWeight: 0.2},
	})
	require.Equal(t, []string{"a", "b"}, sel.Selected)
	require.Equal(t, cashBefore, snap.Cash)
	require.Equal(t, 100_000.0, budget.AvailableCapital)
}

func TestRun_BackwardCompatible(t *testing.T) {
	t.Parallel()
	got := Run(Input{
		Selection: Selection{Selected: []string{"a"}},
		Snapshot:  testSnap(),
		Budget:    Budget{AvailableCapital: 50_000},
		Policy:    Policy{MaxSingleWeight: 0.5},
	})
	require.Len(t, got.Allocated, 1)
	require.False(t, math.IsNaN(got.UniformAmount))
}
