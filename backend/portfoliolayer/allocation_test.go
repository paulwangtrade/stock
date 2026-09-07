package portfoliolayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocate_EqualSplitFloorQuantityAlwaysZero(t *testing.T) {
	t.Parallel()
	snap := FromLedger(testLedger())
	resolved := ConstraintSet{Risk: RiskLayer{MaxGrossExposurePct: f(0.85)}}.Resolve()
	sel := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000002", "sz000003", "sz000004"),
		Snapshot:         snap,
		Constraints:      PortfolioConstraints{MaxNewNames: 2},
	})
	require.Len(t, sel.Selected, 2)
	require.Len(t, sel.Waitlist, 1)

	got := Allocate(sel, snap, resolved)
	require.Equal(t, MethodEqualWeight, got.Method)
	require.Greater(t, got.UniformAmount, 0.0)
	require.Len(t, got.Items, 3)
	for _, item := range got.Items {
		require.Equal(t, int64(0), item.TargetQuantity, "F.1 v1 must not compute lots")
	}
	require.True(t, got.Items[0].InAllocationSet)
	require.False(t, got.Items[2].InAllocationSet)
	require.InDelta(t, got.UniformAmount, got.Items[0].TargetAmount, 1e-6)
	require.InDelta(t, got.UniformAmount, got.Items[2].TargetAmount, 1e-6)
	require.Equal(t, AllocReasonWaitlistUniform, got.Items[2].AllocationReason)
	require.Equal(t, snap.Cash, FromLedger(testLedger()).Cash, "Allocate must not mutate caller ledger")
}

func TestAllocate_NoAccountZeroBudget(t *testing.T) {
	t.Parallel()
	sel := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000002"),
		Snapshot:         FromLedger(nil),
		Constraints:      PortfolioConstraints{MaxNewNames: 5},
	})
	got := Allocate(sel, FromLedger(nil), ConstraintSet{}.Resolve())
	require.Equal(t, "no_account", got.Budget.Binding)
	require.InDelta(t, 0.0, got.Budget.AvailableCapital, 1e-9)
	require.Equal(t, int64(0), got.Items[0].TargetQuantity)
	require.InDelta(t, 0.0, got.Items[0].TargetAmount, 1e-9)
}

func TestAllocate_BelowMinOrderZerosAmount(t *testing.T) {
	t.Parallel()
	snap := FromLedger(testLedger())
	resolved := ConstraintSet{
		Portfolio: PreferenceLayer{MinOrderAmount: f(10_000_000)},
		Risk:      RiskLayer{MaxGrossExposurePct: f(0.85)},
	}.Resolve()
	sel := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000002"),
		Snapshot:         snap,
		Constraints:      resolved.AsPortfolioConstraints(),
	})
	got := Allocate(sel, snap, resolved)
	require.Equal(t, AllocReasonBelowMinOrder, got.Items[0].AllocationReason)
	require.InDelta(t, 0.0, got.Items[0].TargetAmount, 1e-9)
	require.Equal(t, int64(0), got.Items[0].TargetQuantity)
}

func TestAllocate_DoesNotUsePlanFilterRiskCodes(t *testing.T) {
	t.Parallel()
	got := Allocate(nil, FromLedger(nil), ConstraintSet{}.Resolve())
	require.NotEqual(t, "CASH_INSUFFICIENT", got.Budget.Binding)
}
