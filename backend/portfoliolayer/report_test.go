package portfoliolayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObserve_DefaultDisabledHasNoReport(t *testing.T) {
	t.Parallel()
	got := Observe(DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
	}, ObserveOptions{})
	require.Equal(t, ShadowDisabled, got.Status)
	require.Nil(t, got.Report)
	require.Nil(t, got.Decision)
}

func TestShadowReport_RecordsLegacySetAndSelection(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003", "sz000004", "sz000005", "sz000006"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{User: PreferenceLayer{MaxNewNames: i(3)}},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.Equal(t, ShadowObserved, got.Status)
	require.NotNil(t, got.Report)
	r := got.Report
	require.Len(t, r.Legacy.Ranked, 5)
	require.Len(t, r.Legacy.Basket, 5)
	require.Empty(t, r.Legacy.Waitlist)
	require.InDelta(t, LegacyFixedAmountPerName, r.Legacy.AmountPerName, 1e-9)
	require.Len(t, r.Selection.Selected, 3)
	require.Len(t, r.Selection.Waitlist, 2)
	require.NotNil(t, r.Allocation)
	require.NotEmpty(t, r.ConstraintTrace)

	require.GreaterOrEqual(t, r.Metrics.CandidateDiffCount, 2)
	codes := map[string]CandidateDiff{}
	for _, d := range r.CandidateDiffs {
		codes[d.StockCode] = d
	}
	d4 := codes["sz000005"]
	require.Equal(t, LegacyRoleBasket, d4.LegacyRole)
	require.Equal(t, PortfolioRoleWaitlist, d4.PortfolioRole)
	require.Equal(t, ReasonNameLimit, d4.Reason)

	require.True(t, r.Metrics.QuantityAlwaysZero)
	require.Equal(t, 5, r.Metrics.RankedCount)
	require.Equal(t, 3, r.Metrics.PortfolioSelectedCount)
	require.Equal(t, 2, r.Metrics.PortfolioWaitlistCount)
}

func TestShadowReport_SkipHoldingDiff(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000001", "sz000002"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
		Constraints: ConstraintSet{
			User: PreferenceLayer{SkipAlreadyHolding: b(true)},
		},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.NotNil(t, got.Report)
	var found *CandidateDiff
	for i := range got.Report.CandidateDiffs {
		d := got.Report.CandidateDiffs[i]
		if d.StockCode == "sz000001" {
			found = &d
			break
		}
	}
	require.NotNil(t, found)
	require.Equal(t, LegacyRoleBasket, found.LegacyRole)
	require.Equal(t, PortfolioRoleRejected, found.PortfolioRole)
	require.Equal(t, ReasonAlreadyHolding, found.Reason)

	hit := false
	for _, h := range got.Report.ConstraintHits {
		if h.Reason == ReasonAlreadyHolding {
			hit = true
			require.Equal(t, ConsumerPortfolioSelection, h.Consumer)
		}
	}
	require.True(t, hit, "already_holding should appear in constraint hits")
}

func TestShadowReport_AllocationDiffVsLegacyScalar(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   2,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{},
	}
	got := Observe(in, ObserveOptions{Enabled: true, LegacyAmountPerName: LegacyFixedAmountPerName})
	require.NotNil(t, got.Report)
	require.Greater(t, got.Report.Metrics.AllocationDiffCount, 0)
	require.NotEmpty(t, got.Report.AllocationDiffs)
	for _, d := range got.Report.AllocationDiffs {
		require.InDelta(t, LegacyFixedAmountPerName, d.LegacyAmount, 1e-6)
		require.NotZero(t, d.Delta)
		require.Equal(t, int64(0), got.Report.Allocation.Items[0].TargetQuantity)
	}
	require.True(t, got.Report.Metrics.QuantityAlwaysZero)
}

func TestShadowReport_MatchingAmountHasNoAllocDiff(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002"),
		SelectionLimit:   1,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{},
	}
	obs, err := NewSkeletonFlow().Run(in)
	require.NoError(t, err)
	matched := obs.Allocation.UniformAmount
	require.Greater(t, matched, 0.0)
	report := BuildShadowReport(in, obs, ObserveOptions{LegacyAmountPerName: matched})
	require.Empty(t, report.AllocationDiffs)
	require.Equal(t, 0, report.Metrics.AllocationDiffCount)
	require.Empty(t, report.CandidateDiffs)
}

func TestShadowReport_ConstraintTraceUserTighten(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
		Constraints: ConstraintSet{
			User: PreferenceLayer{MaxNewNames: i(1), MaxSingleWeight: f(0.10)},
			Risk: RiskLayer{MaxSingleNamePct: f(0.20)},
		},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.NotEmpty(t, got.Report.ConstraintTrace)
	foundNames := false
	foundSingle := false
	for _, tr := range got.Report.ConstraintTrace {
		if tr.Field == "max_new_names" && tr.Value == "1" {
			foundNames = true
		}
		if tr.Field == "max_single_weight" && tr.Note == "tightened_within_ceiling" {
			foundSingle = true
		}
	}
	require.True(t, foundNames)
	require.True(t, foundSingle)
	require.Greater(t, got.Report.Metrics.ConstraintHitCount, 0)
}

func TestBuildShadowReport_NoTradingSideEffects(t *testing.T) {
	t.Parallel()
	ledger := FromLedger(testLedger())
	cash := ledger.Cash
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   2,
		Ledger:           ledger,
	}
	obs, err := NewSkeletonFlow().Run(in)
	require.NoError(t, err)
	_ = BuildShadowReport(in, obs, ObserveOptions{Enabled: true})
	require.Equal(t, cash, ledger.Cash)
}
