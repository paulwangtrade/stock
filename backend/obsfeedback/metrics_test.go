package obsfeedback

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func rec(id, state string, px float64) Record {
	p := px
	return Record{ObservationID: id, Symbol: id, ObservationDate: "2026-08-01", DecisionState: state, MarketPrice: &p}
}

func evalOut(id string, ret, alpha float64) Outcome {
	return Outcome{
		ObservationID: id, Horizon: HorizonT5, Status: OutcomeEvaluated,
		FutureReturn: fptr(ret), Alpha: fptr(alpha), MaxDrawdown: fptr(-0.02),
	}
}

func TestAggregateMetrics_ByState(t *testing.T) {
	records := []Record{
		rec("n1", StateHoldNormal, 10),
		rec("n2", StateHoldNormal, 10),
		rec("w1", StateHoldWatch, 10),
		rec("w2", StateHoldWatch, 10),
		rec("r1", StateHoldReview, 10),
		rec("pending", StateHoldNormal, 10),
		rec("miss", StateHoldWatch, 10),
	}
	outcomes := []Outcome{
		evalOut("n1", 0.08, 0.05),
		evalOut("n2", -0.01, -0.02),
		evalOut("w1", -0.03, -0.01),
		evalOut("w2", 0.01, 0.00),
		evalOut("r1", -0.08, -0.06),
		{ObservationID: "pending", Horizon: HorizonT5, Status: OutcomePending},
		{ObservationID: "miss", Horizon: HorizonT5, Status: OutcomeInsufficient},
	}
	view := AggregateMetrics(records, outcomes, HorizonT5, BenchmarkCSI300)
	require.Equal(t, ActionNone, view.Action)
	require.Equal(t, Disclaimer, view.Disclaimer)
	require.Equal(t, 7, view.Samples)
	require.Equal(t, 5, view.Evaluated)
	require.InDelta(t, 2.0/5.0, *view.WinRate, 1e-9) // n1, w2 positive
	require.False(t, view.IndustryBenchmark.Available)

	n := view.ByState[StateHoldNormal]
	require.Equal(t, 3, n.Samples)
	require.Equal(t, 2, n.Evaluated)
	require.Equal(t, 1, n.Pending)
	require.InDelta(t, 0.5, *n.WinRate, 1e-9)
	require.InDelta(t, 0.035, *n.AvgReturn, 1e-9)

	w := view.ByState[StateHoldWatch]
	require.Equal(t, 3, w.Samples)
	require.Equal(t, 2, w.Evaluated)
	require.Equal(t, 1, w.Insufficient)
	require.InDelta(t, 0.5, *w.RiskCaptureRate, 1e-9) // w1 negative
	require.InDelta(t, -0.01, *w.AvgReturn, 1e-9)

	r := view.ByState[StateHoldReview]
	require.Equal(t, 1, r.Evaluated)
	require.InDelta(t, -0.08, *r.AvgReturn, 1e-9)
	require.InDelta(t, 1.0, *r.RiskCaptureRate, 1e-9)

	// accuracy: n1 correct, n2 wrong, w1 correct, w2 wrong, r1 correct → 3/5
	require.InDelta(t, 0.6, *view.DecisionAccuracy, 1e-9)
}

func TestAggregateMetrics_MissingFutureExcludedFromWinRate(t *testing.T) {
	records := []Record{rec("a", StateHoldNormal, 10), rec("b", StateHoldNormal, 10)}
	outcomes := []Outcome{
		evalOut("a", 0.10, 0.02),
		{ObservationID: "b", Horizon: HorizonT5, Status: OutcomeInsufficient},
	}
	view := AggregateMetrics(records, outcomes, HorizonT5, BenchmarkNone)
	require.Equal(t, 1, view.Evaluated)
	require.InDelta(t, 1.0, *view.WinRate, 1e-9)
	require.InDelta(t, 0.10, *view.AvgReturn, 1e-9)
}
