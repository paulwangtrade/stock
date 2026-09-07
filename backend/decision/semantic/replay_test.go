package semantic

import (
	"testing"

	"go-stock/backend/models"
)

func TestBuildActionTransitionMatrix(t *testing.T) {
	m := BuildActionTransitionMatrix([]string{
		models.QuantActionEnter,
		models.QuantActionEnter,
		models.QuantActionWaitPullback,
		models.QuantActionWatch,
	})
	if m.TransitionCount != 3 {
		t.Fatalf("transitionCount=%d", m.TransitionCount)
	}
	if m.UniqueEdges != 3 {
		t.Fatalf("uniqueEdges=%d %+v", m.UniqueEdges, m.Transitions)
	}
	// ENTER→ENTER, ENTER→WAIT_PULLBACK, WAIT_PULLBACK→WATCH
	found := false
	for _, e := range m.Transitions {
		if e.FromCode == models.QuantActionEnter && e.ToCode == models.QuantActionEnter && e.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing ENTER→ENTER: %+v", m.Transitions)
	}
}

func TestBuildHistoricalReplayReport_DailyAndTransitions(t *testing.T) {
	mk := func(code, action string) *models.QuantDecision {
		d := basePairDecision(action, "L")
		d.Instrument.StockCode = code
		d.Action.Code = action
		return d
	}

	d1b := mk("sz000001", models.QuantActionEnter)
	d1c := *d1b
	d1c.Meta.Producer = models.QuantProducerGoEngine

	d2b := mk("sz000001", models.QuantActionWaitPullback)
	d2c := *d2b
	d2c.Meta.Producer = models.QuantProducerGoEngine
	d2c.Action.Code = models.QuantActionWatch // conflict

	d3b := mk("sz000001", models.QuantActionWatch)
	d3c := *d3b
	d3c.Meta.Producer = models.QuantProducerGoEngine

	report := BuildHistoricalReplayReport("hist-demo", "sz000001", []ReplayDayPair{
		{TradeDate: "2026-07-14", Pair: ShadowPair{Baseline: d1b, Candidate: &d1c}},
		{TradeDate: "2026-07-15", Pair: ShadowPair{Baseline: d2b, Candidate: &d2c}},
		{TradeDate: "2026-07-16", Pair: ShadowPair{Baseline: d3b, Candidate: &d3c}},
	})

	if report.Phase != "Phase2-D" {
		t.Fatalf("phase=%s", report.Phase)
	}
	if report.TotalDays != 3 || len(report.Daily) != 3 {
		t.Fatalf("days=%d daily=%d", report.TotalDays, len(report.Daily))
	}
	if report.Daily[0].Stability.Phase != "Phase2-C" {
		t.Fatalf("daily stability phase=%s", report.Daily[0].Stability.Phase)
	}
	if report.ActionTransitions.Baseline.TransitionCount != 2 {
		t.Fatalf("baseline transitions=%d", report.ActionTransitions.Baseline.TransitionCount)
	}
	// ENTER→WAIT_PULLBACK, WAIT_PULLBACK→WATCH
	if report.ActionTransitions.Baseline.UniqueEdges != 2 {
		t.Fatalf("baseline edges=%+v", report.ActionTransitions.Baseline.Transitions)
	}
	if report.Rollup.ActionCode.ConflictCount != 1 {
		t.Fatalf("rollup conflicts=%d", report.Rollup.ActionCode.ConflictCount)
	}
	if report.ActionTransitions.CrossProducerSameDay.DayCount != 3 {
		t.Fatalf("cross dayCount=%d", report.ActionTransitions.CrossProducerSameDay.DayCount)
	}
}
