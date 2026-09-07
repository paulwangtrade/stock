package semantic

import (
	"testing"

	"go-stock/backend/models"
)

func basePairDecision(actionCode, label string) *models.QuantDecision {
	return &models.QuantDecision{
		Instrument: models.QuantInstrument{StockCode: "sz000001"},
		Signal:     models.QuantSignal{Tag: "\u5f3a", TagKind: models.QuantTagKindEntry},
		EntryZone: &models.QuantEntryZone{
			Low: 10, High: 10.2, InstantPrice: 10.1,
			Mode: models.QuantZoneModeNear, DeferMode: models.QuantDeferSame, Text: "10.00~10.20",
		},
		Gate: models.QuantGate{Score: 0.9, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk: models.QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size: models.QuantSize{OK: true, TargetShares: 1000, AddShares: 1000, PositionPct: 10},
		Action: models.QuantAction{Code: actionCode, Label: label, AllowDraft: true, Side: "buy"},
		Meta:   models.QuantDecisionMeta{SchemaVersion: 1, Producer: models.QuantProducerJSLegacy},
	}
}

func TestBuildShadowStabilityReport_StableBatch(t *testing.T) {
	a := basePairDecision(models.QuantActionEnter, "\u53ef\u4e70")
	b := *a
	b.Meta.Producer = models.QuantProducerGoEngine
	b.Action.Label = "\u5efa\u8bae\u5173\u6ce8\u4e70\u5165" // display-only

	c := basePairDecision(models.QuantActionEnter, "\u53ef\u4e70")
	c.Instrument.StockCode = "sz000002"
	d := *c
	d.Meta.Producer = models.QuantProducerGoEngine

	report := BuildShadowStabilityReport([]ShadowPair{
		{ID: "p1", Code: "sz000001", Baseline: a, Candidate: &b},
		{ID: "p2", Code: "sz000002", Baseline: c, Candidate: &d},
	})

	if report.TotalPairs != 2 {
		t.Fatalf("total=%d", report.TotalPairs)
	}
	if report.SemanticEqualCount != 2 || report.SemanticMismatchCount != 0 {
		t.Fatalf("semantic counts %+v", report)
	}
	if report.ActionCode.ConflictCount != 0 {
		t.Fatalf("action conflicts=%d", report.ActionCode.ConflictCount)
	}
	if report.DisplayOnlyDiffCount != 1 {
		t.Fatalf("displayOnly=%d", report.DisplayOnlyDiffCount)
	}
	if report.Phase != "Phase2-C" {
		t.Fatalf("phase=%s", report.Phase)
	}
}

func TestBuildShadowStabilityReport_ActionCodeConflictsAndSliceStats(t *testing.T) {
	left := basePairDecision(models.QuantActionEnter, "\u53ef\u4e70")
	right := *left
	right.Meta.Producer = models.QuantProducerGoEngine
	right.Action = models.QuantAction{
		Code: models.QuantActionWaitPullback, Label: "\u7b49\u56de\u8e29", Side: "none",
	}
	z := *right.EntryZone
	z.Mode = models.QuantZoneModeAbove
	right.EntryZone = &z

	left2 := basePairDecision(models.QuantActionEnter, "\u53ef\u4e70")
	left2.Instrument.StockCode = "sz000003"
	right2 := *left2
	right2.Meta.Producer = models.QuantProducerGoEngine
	right2.Action = models.QuantAction{
		Code: models.QuantActionWaitPullback, Label: "\u7b49\u56de\u8e29", Side: "none",
	}

	report := BuildShadowStabilityReport([]ShadowPair{
		{ID: "c1", Baseline: left, Candidate: &right},
		{ID: "c2", Baseline: left2, Candidate: &right2},
	})

	if report.ActionCode.ConflictCount != 2 {
		t.Fatalf("conflictCount=%d", report.ActionCode.ConflictCount)
	}
	if len(report.ActionCode.ConflictStats) == 0 {
		t.Fatal("expected conflictStats")
	}
	stat := report.ActionCode.ConflictStats[0]
	if stat.LeftCode != models.QuantActionEnter || stat.RightCode != models.QuantActionWaitPullback || stat.Count != 2 {
		t.Fatalf("stat=%+v", stat)
	}
	if report.MismatchBySlice["action"].MismatchPairCount != 2 {
		t.Fatalf("action slice mismatch=%+v", report.MismatchBySlice["action"])
	}
	if report.MismatchBySlice["entryZone"].MismatchPairCount != 1 {
		t.Fatalf("entryZone mismatch=%+v", report.MismatchBySlice["entryZone"])
	}
	if report.SemanticMismatchCount != 2 {
		t.Fatalf("semanticMismatch=%d", report.SemanticMismatchCount)
	}
}
