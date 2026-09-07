package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCandidatePoolItem_DecisionIDJSONRoundTrip(t *testing.T) {
	in := CandidatePoolItem{
		PoolID:     1,
		TradeDate:  "2026-07-21",
		StockCode:  "sz000001",
		StockName:  "PingAn",
		Rank:       1,
		Score:      0.91,
		SignalTag:  "强",
		DecisionID: "qd1:js_legacy:sz000001:2026-07-21T00:00:00.000Z:ENTER:可买",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"decisionId"`) {
		t.Fatalf("json missing decisionId: %s", b)
	}
	var out CandidatePoolItem
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.DecisionID != in.DecisionID {
		t.Fatalf("DecisionID=%q want %q", out.DecisionID, in.DecisionID)
	}
	if out.Rank != 1 || out.Score != 0.91 {
		t.Fatalf("Score/Rank mutated: rank=%d score=%v", out.Rank, out.Score)
	}
}

func TestEnrichCandidatePoolDecisionIDs_DoesNotChangeScoreOrRank(t *testing.T) {
	items := []CandidatePoolItem{
		{StockCode: "sz000001", Rank: 1, Score: 0.9},
		{StockCode: "sh600519", Rank: 2, Score: 0.8},
		{StockCode: "sz000002", Rank: 3, Score: 0.7},
	}
	ranks := []int{1, 2, 3}
	scores := []float64{0.9, 0.8, 0.7}

	EnrichCandidatePoolDecisionIDs(items, map[string]string{
		"sz000001": "qd1:js_legacy:sz000001:_:ENTER:_",
		"SH600519": "qd1:js_legacy:sh600519:_:WATCH:_", // case-insensitive
	})

	if items[0].DecisionID == "" || items[1].DecisionID == "" {
		t.Fatalf("expected bindings: %+v", items)
	}
	if items[2].DecisionID != "" {
		t.Fatalf("unbound item should stay empty, got %q", items[2].DecisionID)
	}
	for i := range items {
		if items[i].Rank != ranks[i] || items[i].Score != scores[i] {
			t.Fatalf("item[%d] Score/Rank changed: %+v", i, items[i])
		}
	}
	// order preserved
	if items[0].StockCode != "sz000001" || items[1].StockCode != "sh600519" {
		t.Fatalf("order changed: %+v", items)
	}
}

func TestEnrichCandidatePoolDecisionIDs_EmptyLookupNoop(t *testing.T) {
	items := []CandidatePoolItem{{StockCode: "sz000001", Rank: 1, Score: 1}}
	EnrichCandidatePoolDecisionIDs(items, nil)
	if items[0].DecisionID != "" {
		t.Fatalf("noop should not bind, got %q", items[0].DecisionID)
	}
}
