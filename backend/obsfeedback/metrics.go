package obsfeedback

import "strings"

// AggregateMetrics builds performance stats from records + outcomes.
// Only EVALUATED outcomes enter win_rate / avg_return / accuracy.
func AggregateMetrics(records []Record, outcomes []Outcome, horizon int, benchmark string) PerformanceView {
	horizon = NormalizeHorizon(horizon)
	benchmark = NormalizeBenchmark(benchmark)
	view := PerformanceView{
		Horizon:   horizon,
		Benchmark: benchmark,
		Disclaimer: Disclaimer,
		Action:     ActionNone,
		ByState: map[string]StateMetrics{
			StateHoldNormal:    {DecisionState: StateHoldNormal},
			StateHoldWatch:     {DecisionState: StateHoldWatch},
			StateHoldReview:    {DecisionState: StateHoldReview},
			StateExitCandidate: {DecisionState: StateExitCandidate},
		},
		IndustryBenchmark: IndustryBenchmarkView{Available: false, Note: industryBenchmarkNote},
	}

	outByID := map[string]Outcome{}
	for _, o := range outcomes {
		if o.Horizon != horizon {
			continue
		}
		outByID[o.ObservationID] = o
	}

	type acc struct {
		samples, evaluated, pending, insufficient int
		wins, riskHits, correct                   int
		sumRet, sumAlpha, sumDD                   float64
		alphaN, ddN                               int
	}
	by := map[string]*acc{
		StateHoldNormal:    {},
		StateHoldWatch:     {},
		StateHoldReview:    {},
		StateExitCandidate: {},
	}
	var all acc

	for _, rec := range records {
		state := strings.ToUpper(strings.TrimSpace(rec.DecisionState))
		a, ok := by[state]
		if !ok {
			a = &acc{}
			by[state] = a
		}
		a.samples++
		all.samples++
		o, has := outByID[rec.ObservationID]
		if !has {
			a.insufficient++
			all.insufficient++
			continue
		}
		switch o.Status {
		case OutcomePending:
			a.pending++
			all.pending++
			continue
		case OutcomeEvaluated:
			if o.FutureReturn == nil {
				a.insufficient++
				all.insufficient++
				continue
			}
		default:
			a.insufficient++
			all.insufficient++
			continue
		}
		a.evaluated++
		all.evaluated++
		ret := *o.FutureReturn
		a.sumRet += ret
		all.sumRet += ret
		if ret > 0 {
			a.wins++
			all.wins++
		}
		if ret < 0 {
			a.riskHits++
			all.riskHits++
		}
		if o.Alpha != nil {
			a.sumAlpha += *o.Alpha
			a.alphaN++
			all.sumAlpha += *o.Alpha
			all.alphaN++
		}
		if o.MaxDrawdown != nil {
			a.sumDD += *o.MaxDrawdown
			a.ddN++
			all.sumDD += *o.MaxDrawdown
			all.ddN++
		}
		if isCorrect(state, ret) {
			a.correct++
			all.correct++
		}
	}

	view.Samples = all.samples
	view.Evaluated = all.evaluated
	view.WinRate = safeRatio(all.wins, all.evaluated)
	view.AvgReturn = safeMean(all.sumRet, all.evaluated)
	view.AvgAlpha = safeMean(all.sumAlpha, all.alphaN)
	view.DecisionAccuracy = safeRatio(all.correct, all.evaluated)

	for state, a := range by {
		m := StateMetrics{
			DecisionState:  state,
			Samples:        a.samples,
			Evaluated:      a.evaluated,
			Pending:        a.pending,
			Insufficient:   a.insufficient,
			WinRate:        safeRatio(a.wins, a.evaluated),
			AvgReturn:      safeMean(a.sumRet, a.evaluated),
			AvgAlpha:       safeMean(a.sumAlpha, a.alphaN),
			AvgMaxDrawdown: safeMean(a.sumDD, a.ddN),
		}
		if state == StateHoldWatch || state == StateHoldReview || state == StateExitCandidate {
			m.RiskCaptureRate = safeRatio(a.riskHits, a.evaluated)
		}
		view.ByState[state] = m
	}
	return view
}

func isCorrect(state string, ret float64) bool {
	switch state {
	case StateHoldWatch, StateHoldReview, StateExitCandidate:
		return ret < 0
	default:
		return ret >= 0
	}
}

// EvaluateAll maps each record through EvaluateOutcome.
func EvaluateAll(records []Record, horizon int, stock map[string][]Bar, benchmark []Bar, benchmarkID string) []Outcome {
	horizon = NormalizeHorizon(horizon)
	out := make([]Outcome, 0, len(records))
	for _, rec := range records {
		out = append(out, EvaluateOutcome(rec, horizon, stock[rec.Symbol], benchmark, benchmarkID))
	}
	return out
}
