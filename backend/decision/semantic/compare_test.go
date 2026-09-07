package semantic

import (
	"testing"

	"go-stock/backend/models"
)

func sampleDecision(code, label string) *models.QuantDecision {
	return &models.QuantDecision{
		Instrument: models.QuantInstrument{StockCode: "sz000001"},
		Signal: models.QuantSignal{
			Tag: "\u5f3a", TagKind: models.QuantTagKindEntry, DaysAgo: 0,
		},
		EntryZone: &models.QuantEntryZone{
			Low: 10, High: 10.2, InstantPrice: 10.1,
			Mode: models.QuantZoneModeNear, DeferMode: models.QuantDeferSame,
			Text: "10.00~10.20",
		},
		Gate: models.QuantGate{Score: 0.875, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk: models.QuantRiskSlice{Passed: true, Code: "APPROVED", Message: "placeholder-a"},
		Size: models.QuantSize{OK: true, TargetShares: 1000, AddShares: 1000, PositionPct: 12.5, Reason: "r1"},
		Action: models.QuantAction{
			Code: code, Label: label, AllowDraft: true, Side: "buy",
		},
		Meta: models.QuantDecisionMeta{
			SchemaVersion: 1,
			Producer:      models.QuantProducerJSLegacy,
		},
	}
}

func TestNormalize_DropsActionLabel(t *testing.T) {
	d := sampleDecision(models.QuantActionEnter, "\u53ef\u4e70")
	n := NormalizeDecision(d)
	if n.Action.Code != models.QuantActionEnter {
		t.Fatalf("code=%s", n.Action.Code)
	}
	// Label must not live on semantic action
	if n.Action.Code == "" {
		t.Fatal("code required")
	}
}

func TestSemanticEqual_SameCodeDifferentLabel(t *testing.T) {
	left := sampleDecision(models.QuantActionEnter, "\u53ef\u4e70")
	right := sampleDecision(models.QuantActionEnter, "\u5efa\u8bae\u5173\u6ce8\u4e70\u5165") // different display label
	right.Meta.Producer = models.QuantProducerGoEngine
	right.Risk.Message = "placeholder-b"
	right.Size.Reason = "r2"

	r := CompareSemantic(left, right)
	if !r.SemanticEqual {
		t.Fatalf("want semantic equal, got %s diffs=%v", r.Summary, r.SemanticDiffs)
	}
	if !r.ActionCodeEqual {
		t.Fatal("action code should match")
	}
	if len(r.DisplayDiffs) != 1 || r.DisplayDiffs[0].Path != "action.label" {
		t.Fatalf("want display label diff, got %+v", r.DisplayDiffs)
	}
}

func TestSemanticFail_ActionCodeHighestPriority(t *testing.T) {
	left := sampleDecision(models.QuantActionEnter, "\u53ef\u4e70")
	right := sampleDecision(models.QuantActionWaitPullback, "\u7b49\u56de\u8e29")
	right.Meta.Producer = models.QuantProducerGoEngine

	r := CompareSemantic(left, right)
	if r.SemanticEqual {
		t.Fatal("want semantic fail")
	}
	if r.ActionCodeEqual {
		t.Fatal("codes differ")
	}
	if !HasHighestPriorityCodeDiff(r) {
		t.Fatalf("want highest priority action.code diff: %+v", r.SemanticDiffs)
	}
}

func TestSemanticEqual_IgnoresIDProducerAsOf(t *testing.T) {
	left := sampleDecision(models.QuantActionEnter, "\u53ef\u4e70")
	right := *left
	right.ID = "other"
	right.Meta.Producer = models.QuantProducerGoEngine
	r := CompareSemantic(left, &right)
	if !r.SemanticEqual {
		t.Fatalf("%s %v", r.Summary, r.SemanticDiffs)
	}
}
