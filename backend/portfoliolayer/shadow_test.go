package portfoliolayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObserve_DefaultDisabledDoesNotRunDecisionCore(t *testing.T) {
	t.Parallel()
	require.False(t, DefaultShadowEnabled)

	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
	}
	got := Observe(in, ObserveOptions{})
	require.Equal(t, ShadowDisabled, got.Status)
	require.False(t, got.Enabled)
	require.Nil(t, got.Decision)
	require.Nil(t, got.Report)
	require.Nil(t, got.Evaluation)
}

func TestObserve_EnabledRunsShadowWithoutTradePlan(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003", "sz000004"),
		SelectionLimit:   2,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.Equal(t, ShadowObserved, got.Status)
	require.NotNil(t, got.Decision)
	require.NotNil(t, got.Decision.Selection)
	require.NotNil(t, got.Decision.Allocation)
	require.Len(t, got.Decision.Selection.Selected, 2)
	require.Len(t, got.Decision.ScanList, 3)
	for _, item := range got.Decision.Allocation.Items {
		require.Equal(t, int64(0), item.TargetQuantity)
	}
	require.NotNil(t, got.Report)
	require.Len(t, got.Report.Legacy.Basket, 2)
	require.Len(t, got.Report.Legacy.Waitlist, 1)
	require.True(t, got.Report.Metrics.QuantityAlwaysZero)
	require.NotNil(t, got.Evaluation)
	require.True(t, got.Evaluation.RecordOnly)
}

func TestSkeletonFlow_RunIsPure(t *testing.T) {
	t.Parallel()
	ledger := FromLedger(testLedger())
	cashBefore := ledger.Cash
	obs, err := NewSkeletonFlow().Run(DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		Ledger:           ledger,
		Constraints:      ConstraintSet{User: PreferenceLayer{MaxNewNames: i(1)}},
	})
	require.NoError(t, err)
	require.Len(t, obs.Selection.Selected, 1)
	require.Equal(t, cashBefore, ledger.Cash)
	require.Equal(t, ledger.Cash, testLedger().Cash)
}
