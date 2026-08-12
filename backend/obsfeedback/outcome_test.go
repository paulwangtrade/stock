package obsfeedback

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateOutcome_FutureReturnAndAlpha(t *testing.T) {
	px := 10.0
	rec := Record{
		ObservationID:   "obs-2026-08-03-sz000001",
		Symbol:          "sz000001",
		ObservationDate: "2026-08-03",
		MarketPrice:     &px,
		DecisionState:   StateHoldNormal,
	}
	stock := []Bar{
		{Date: "2026-08-03", Close: 10, High: 10.2, Low: 9.9},
		{Date: "2026-08-04", Close: 10.2, High: 10.4, Low: 10.0},
		{Date: "2026-08-05", Close: 10.5, High: 10.6, Low: 10.1},
		{Date: "2026-08-06", Close: 10.4, High: 10.7, Low: 10.2},
		{Date: "2026-08-07", Close: 10.6, High: 10.8, Low: 10.3},
		{Date: "2026-08-10", Close: 10.8, High: 11.0, Low: 10.4}, // T+5
	}
	bench := []Bar{
		{Date: "2026-08-03", Close: 100},
		{Date: "2026-08-04", Close: 101},
		{Date: "2026-08-05", Close: 102},
		{Date: "2026-08-06", Close: 102},
		{Date: "2026-08-07", Close: 103},
		{Date: "2026-08-10", Close: 103}, // +3%
	}
	o := EvaluateOutcome(rec, HorizonT5, stock, bench, BenchmarkCSI300)
	require.Equal(t, OutcomeEvaluated, o.Status)
	require.Equal(t, "2026-08-10", o.FutureDate)
	require.InDelta(t, 10.8, *o.FuturePrice, 1e-9)
	require.InDelta(t, 0.08, *o.FutureReturn, 1e-9) // +8%
	require.InDelta(t, 0.03, *o.BenchmarkReturn, 1e-9)
	require.InDelta(t, 0.05, *o.Alpha, 1e-9) // +5%
	require.NotNil(t, o.MaxDrawdown)
	require.NotNil(t, o.MaxProfit)
}

func TestEvaluateOutcome_MissingFuturePrice(t *testing.T) {
	px := 10.0
	rec := Record{
		ObservationID:   "obs-2026-08-03-sz000001",
		Symbol:          "sz000001",
		ObservationDate: "2026-08-03",
		MarketPrice:     &px,
	}
	o := EvaluateOutcome(rec, HorizonT5, []Bar{{Date: "2026-08-03", Close: 10}}, nil, BenchmarkCSI300)
	require.Equal(t, OutcomePending, o.Status)
	require.Nil(t, o.FutureReturn)

	o2 := EvaluateOutcome(rec, HorizonT5, []Bar{
		{Date: "2026-08-04", Close: 10.1},
		{Date: "2026-08-05", Close: 10.2},
	}, nil, BenchmarkCSI300)
	require.Equal(t, OutcomeInsufficient, o2.Status)
	require.Nil(t, o2.FutureReturn)
}

func TestEvaluateOutcome_MissingBenchmarkDoesNotFail(t *testing.T) {
	px := 10.0
	rec := Record{
		ObservationID:   "obs-2026-08-03-sz000001",
		ObservationDate: "2026-08-03",
		MarketPrice:     &px,
	}
	stock := []Bar{
		{Date: "2026-08-04", Close: 11},
		{Date: "2026-08-05", Close: 11},
		{Date: "2026-08-06", Close: 11},
		{Date: "2026-08-07", Close: 11},
		{Date: "2026-08-10", Close: 11},
	}
	o := EvaluateOutcome(rec, HorizonT5, stock, nil, BenchmarkCSI300)
	require.Equal(t, OutcomeEvaluated, o.Status)
	require.InDelta(t, 0.10, *o.FutureReturn, 1e-9)
	require.Nil(t, o.BenchmarkReturn)
	require.Nil(t, o.Alpha)
}

func TestEvaluateOutcome_MissingMarketPrice(t *testing.T) {
	rec := Record{ObservationID: "x", ObservationDate: "2026-08-03"}
	o := EvaluateOutcome(rec, HorizonT5, []Bar{{Date: "2026-08-10", Close: 11}}, nil, BenchmarkNone)
	require.Equal(t, OutcomeInsufficient, o.Status)
	require.Nil(t, o.FutureReturn)
}
