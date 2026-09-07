package portfoliolayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocationShadow_LegacyFixedVsEngine(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003", "sz000004"),
		SelectionLimit:   2,
		Ledger:           FromLedger(testLedger()),
		Constraints: ConstraintSet{
			User: PreferenceLayer{ReserveCashRatio: f(0.10), MaxSingleWeight: f(0.15)},
			Risk: RiskLayer{MaxGrossExposurePct: f(0.85), MaxSingleNamePct: f(0.20)},
		},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.NotNil(t, got.Report)
	require.NotNil(t, got.Report.AllocationShadow)
	ash := got.Report.AllocationShadow

	require.True(t, ash.RecordOnly)
	require.True(t, ash.NotATradePlan)
	require.True(t, ash.NotAProviderSwitch)
	require.Equal(t, "fixed_amount", ash.ChainProvider)
	require.Equal(t, "constant", ash.LegacyAmountSource)
	require.True(t, ash.Comparable)

	require.Equal(t, "fixed_amount", ash.Legacy.Method)
	require.Equal(t, 2, ash.Legacy.AllocationSetCount)
	require.Equal(t, 1, ash.Legacy.WaitlistCount)
	require.InDelta(t, LegacyFixedAmountPerName, ash.Legacy.UniformOrScalar, 1e-9)
	require.InDelta(t, 2*LegacyFixedAmountPerName, ash.Legacy.SumAllocationSet, 1e-6)
	require.Nil(t, ash.Legacy.Budget)

	require.Equal(t, MethodEqualWeight, ash.Portfolio.Method)
	require.NotNil(t, ash.Portfolio.Budget)
	require.Greater(t, ash.Portfolio.Budget.ReserveCash, 0.0)

	require.False(t, ash.BudgetDiff.LegacyHasBudget)
	require.Equal(t, "legacy_no_generation_budget", ash.BudgetDiff.Note)
	require.InDelta(t, 2*LegacyFixedAmountPerName, ash.BudgetDiff.ImpliedLegacyNotional, 1e-6)

	require.InDelta(t, LegacyFixedAmountPerName, ash.UniformDiff.LegacyScalar, 1e-9)
	require.InDelta(t, ash.Portfolio.UniformOrScalar, ash.UniformDiff.PortfolioUniform, 1e-9)
	require.InDelta(t, ash.UniformDiff.PortfolioUniform-ash.UniformDiff.LegacyScalar, ash.UniformDiff.Delta, 1e-9)

	require.InDelta(t, 0, ash.ReserveDiff.LegacyReserve, 1e-9)
	require.Greater(t, ash.ReserveDiff.PortfolioReserve, 0.0)
	require.InDelta(t, ash.ReserveDiff.PortfolioReserve, ash.ReserveDiff.Delta, 1e-9)

	require.True(t, ash.RiskCutDiff.LegacySizerIgnoresCaps)
	require.Equal(t, ash.Portfolio.Binding, ash.RiskCutDiff.PortfolioBinding)

	require.Equal(t, "copy_uniform", ash.WaitlistAmountDiff.Mode)
	require.InDelta(t, LegacyFixedAmountPerName, ash.WaitlistAmountDiff.LegacyWaitlistAmount, 1e-9)
	require.Equal(t, 1, ash.WaitlistAmountDiff.LegacyWaitlistCount)
	require.Contains(t, ash.WaitlistAmountDiff.Note, "waitlist_excluded")
}

func TestAllocationShadow_SingleCapReflected(t *testing.T) {
	t.Parallel()
	// Tiny equity headroom so equal-weight is large, single cap bites.
	snap := FromLedger(testLedger())
	snap.Equity = 100_000
	snap.Cash = 90_000
	snap.Exposure = 0
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   2,
		Ledger:           snap,
		Constraints: ConstraintSet{
			Risk: RiskLayer{MaxGrossExposurePct: f(0.95), MaxSingleNamePct: f(0.10)},
		},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	ash := got.Report.AllocationShadow
	require.NotNil(t, ash)
	require.True(t, ash.RiskCutDiff.SingleCapApplied || ash.UniformDiff.PortfolioCapped)
	require.True(t, ash.Metrics.SingleCapHit || ash.UniformDiff.PortfolioCapped)
	require.Less(t, ash.UniformDiff.PortfolioUniform, ash.UniformDiff.LegacyScalar)
}

func TestAllocationShadow_DisabledObserveHasNoReport(t *testing.T) {
	t.Parallel()
	got := Observe(DecisionInput{
		RankedCandidates: ranked("sz000002"),
		Ledger:           FromLedger(testLedger()),
	}, ObserveOptions{})
	require.Nil(t, got.Report)
}

func TestAllocationShadow_DoesNotChangeWriteChainDefaults(t *testing.T) {
	t.Parallel()
	require.False(t, DefaultShadowEnabled)
}
