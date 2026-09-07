package portfoliolayer

import (
	"testing"

	"go-stock/backend/selection"

	"github.com/stretchr/testify/require"
)

func TestObserve_DefaultDisabledHasNoEvaluation(t *testing.T) {
	t.Parallel()
	got := Observe(DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
	}, ObserveOptions{})
	require.Nil(t, got.Evaluation)
	require.False(t, DefaultShadowEnabled)
}

func TestEvaluateShadow_CandidateNameLimitDeltas(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003", "sz000004", "sz000005", "sz000006"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{User: PreferenceLayer{MaxNewNames: i(3)}},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.NotNil(t, got.Evaluation)
	ev := got.Evaluation
	require.True(t, ev.RecordOnly)
	require.Equal(t, EvaluationStatusEvaluated, ev.Status)
	require.Equal(t, -2, ev.Candidate.CandidateCountDelta)
	require.Equal(t, 2, ev.Candidate.SelectedChangeCount)
	require.Equal(t, 2, ev.Candidate.WaitlistCountDelta)
	require.Equal(t, 2, ev.Candidate.WaitlistChangeCount)
	require.Equal(t, -2, ev.Portfolio.PositionCountDelta)
	require.Equal(t, 5, ev.Portfolio.LegacyProjectedNames)
	require.Equal(t, 3, ev.Portfolio.ShadowProjectedNames)

	require.InDelta(t, 5*LegacyFixedAmountPerName, ev.Allocation.LegacyBuyNotional, 1e-6)
	require.InDelta(t, ev.Allocation.ShadowBuyNotional-ev.Allocation.LegacyBuyNotional, ev.Allocation.AmountDelta, 1e-6)
	require.InDelta(t, ev.Allocation.AmountDelta, ev.Allocation.CashUsageDelta, 1e-6)

	foundLimit := false
	for _, r := range ev.Explainability.SelectionReasonSummary {
		if r.Reason == ReasonNameLimit {
			require.Equal(t, 2, r.Count)
			foundLimit = true
		}
	}
	require.True(t, foundLimit)
	foundImpact := false
	for _, imp := range ev.Explainability.ConstraintImpact {
		if imp.Reason == ReasonNameLimit || imp.Field == ReasonNameLimit {
			foundImpact = true
			require.NotEmpty(t, imp.Effect)
		}
	}
	require.True(t, foundImpact)
}

func TestEvaluateShadow_SkipHoldingSelectedChange(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000001", "sz000002"),
		SelectionLimit:   5,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{User: PreferenceLayer{SkipAlreadyHolding: b(true)}},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.Equal(t, 1, got.Evaluation.Candidate.SelectedChangeCount)
	require.Contains(t, reasons(got.Evaluation), ReasonAlreadyHolding)
}

func TestEvaluateShadow_ReserveDelta(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002"),
		SelectionLimit:   1,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{User: PreferenceLayer{ReserveCashRatio: f(0.10)}},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.Greater(t, got.Evaluation.Allocation.ReserveDelta, 0.0)
	require.InDelta(t, 80_000, got.Evaluation.Allocation.ShadowReserve, 1e-6)
	require.InDelta(t, 0, got.Evaluation.Allocation.LegacyReserve, 1e-9)
}

func TestEvaluateShadow_SectorUnavailableWithoutIndustry(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		SelectionLimit:   2,
		Ledger:           FromLedger(testLedger()),
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.False(t, got.Evaluation.Portfolio.SectorModelAvailable)
	require.Equal(t, SectorNoteUnavailable, got.Evaluation.Portfolio.SectorNote)
	require.InDelta(t, 0, got.Evaluation.Portfolio.SectorExposureDelta, 1e-12)
}

func TestEvaluateShadow_SectorDeltaWhenIndustryPresent(t *testing.T) {
	t.Parallel()
	cands := []selection.Candidate{
		{StockCode: "sz000002", Rank: 1, Industry: "bank"},
		{StockCode: "sz000003", Rank: 2, Industry: "bank"},
	}
	in := DecisionInput{
		RankedCandidates: cands,
		SelectionLimit:   2,
		Ledger:           FromLedger(testLedger()),
		Constraints:      ConstraintSet{User: PreferenceLayer{MaxNewNames: i(1)}},
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.True(t, got.Evaluation.Portfolio.SectorModelAvailable)
	require.Equal(t, SectorNoteAvailable, got.Evaluation.Portfolio.SectorNote)
	require.NotZero(t, got.Evaluation.Portfolio.LegacyMaxSectorWeight)
}

func TestEvaluateShadow_ConcentrationRecorded(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002"),
		SelectionLimit:   1,
		Ledger:           FromLedger(testLedger()),
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.Greater(t, got.Evaluation.Portfolio.LegacyMaxNameWeight, 0.0)
	require.Greater(t, got.Evaluation.Portfolio.ShadowMaxNameWeight, 0.0)
	require.InDelta(t,
		got.Evaluation.Portfolio.ShadowMaxNameWeight-got.Evaluation.Portfolio.LegacyMaxNameWeight,
		got.Evaluation.Portfolio.ConcentrationDelta,
		1e-9,
	)
}

func TestEvaluateShadow_RecordOnlyNoAutoDecision(t *testing.T) {
	t.Parallel()
	in := DecisionInput{
		RankedCandidates: ranked("sz000002"),
		SelectionLimit:   1,
		Ledger:           FromLedger(testLedger()),
	}
	got := Observe(in, ObserveOptions{Enabled: true})
	require.True(t, got.Evaluation.RecordOnly)
	require.NotContains(t, got.Evaluation.Status, "approve")
	require.NotContains(t, got.Evaluation.Status, "reject")
	require.NotContains(t, got.Evaluation.Status, "prefer")
}

func TestEvaluateShadow_NilReport(t *testing.T) {
	t.Parallel()
	require.Nil(t, EvaluateShadow(DecisionInput{}, nil))
}

func reasons(ev *ShadowEvaluationReport) []string {
	out := []string{}
	if ev == nil {
		return out
	}
	for _, r := range ev.Explainability.SelectionReasonSummary {
		out = append(out, r.Reason)
	}
	return out
}
