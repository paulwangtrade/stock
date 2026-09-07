package shadow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/tradeplan/candidate"
)

func baseCandidate() candidate.TradePlanCandidate {
	return candidate.TradePlanCandidate{
		CandidateID:        "tpc:test1",
		SourceDecisionID:   "qd1:js_legacy:sz000001:2026-07-21T00:00:00Z:ENTER:可买",
		SourceSnapshotHash: "hash-abc",
		Side:               "buy",
		TargetShares:       1000,
		EntryPriceHint:     10.1,
		StopPriceHint:      9.85,
		IntentKind:         candidate.IntentEnterHint,
		Executable:         false,
		StockCode:          "sz000001",
		TradeDate:          "2026-07-21",
		AdapterVersion:     "1",
	}
}

func basePlan(item models.TradePlanItem) models.TradePlan {
	return models.TradePlan{
		TradeDate:  "2026-07-21",
		Status:     models.TradePlanStatusReady,
		Side:       "buy",
		RiskStatus: "ok",
		Items:      []models.TradePlanItem{item},
	}
}

func TestCompare_Case1_Equal(t *testing.T) {
	c := baseCandidate()
	item := models.TradePlanItem{
		StockCode:    "sz000001",
		Side:         "buy",
		TargetVolume: 1000,
		LimitPrice:   10.1,
		Reason:       "buy enter",
		Status:       models.TradePlanItemPending,
	}
	diff := CompareTradePlanCandidate(c, basePlan(item))
	if !diff.Equal {
		t.Fatalf("want equal: %+v", diff)
	}
}

func TestCompare_Case2_SharesDiff(t *testing.T) {
	c := baseCandidate()
	item := models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 500, LimitPrice: 10.1, Reason: "buy",
	}
	diff := CompareTradePlanCandidate(c, basePlan(item))
	if diff.Equal {
		t.Fatal("want diff")
	}
	found := false
	for _, d := range diff.DifferentFields {
		if d.Path == "TargetShares" && d.Kind == "different" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing TargetShares diff: %+v", diff.DifferentFields)
	}
}

func TestCompare_Case3_StatusIgnored(t *testing.T) {
	c := baseCandidate()
	item := models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 1000, LimitPrice: 10.1, Reason: "buy",
		Status: models.TradePlanItemFilled, // lifecycle different
	}
	plan := basePlan(item)
	plan.Status = models.TradePlanStatusDone
	diff := CompareTradePlanCandidate(c, plan)
	if !diff.Equal {
		t.Fatalf("status must be ignored for Equal: %+v", diff)
	}
	ignored := false
	for _, d := range diff.DifferentFields {
		if d.Kind == "ignored" && (d.Path == "Status" || d.Path == "Item.Status") {
			ignored = true
		}
	}
	if !ignored {
		t.Fatalf("expected ignored status: %+v", diff.DifferentFields)
	}
}

func TestCompare_Case4_ForbiddenExecutionFields(t *testing.T) {
	c := baseCandidate()
	item := models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 1000, LimitPrice: 10.1, Reason: "buy",
		OrderID: 42, FillID: 7, FilledVolume: 100, FilledPrice: 10.05,
	}
	diff := CompareTradePlanCandidate(c, basePlan(item))
	found := false
	for _, d := range diff.DifferentFields {
		if d.Kind == "forbidden_field" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected forbidden_field: %+v", diff)
	}
	risk := false
	for _, r := range diff.RiskFlags {
		if r == "forbidden_field" {
			risk = true
		}
	}
	if !risk {
		t.Fatalf("riskFlags: %+v", diff.RiskFlags)
	}
}

func TestShadowBatchGolden(t *testing.T) {
	c1 := baseCandidate()
	p1 := basePlan(models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 1000, LimitPrice: 10.1, Reason: "buy enter",
	})

	c2 := baseCandidate()
	c2.CandidateID = "tpc:test2"
	p2 := basePlan(models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 800, LimitPrice: 10.1, Reason: "buy",
	})

	c3 := baseCandidate()
	c3.CandidateID = "tpc:test3"
	p3item := models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 1000, LimitPrice: 10.1, Reason: "buy",
		Status: models.TradePlanItemFilled,
	}
	p3 := basePlan(p3item)
	p3.Status = models.TradePlanStatusExecuting
	now := time.Date(2026, 7, 21, 1, 0, 0, 0, time.UTC)
	p3.ExecutedAt = &now

	c4 := baseCandidate()
	c4.CandidateID = "tpc:test4"
	p4 := basePlan(models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", TargetVolume: 1000, LimitPrice: 10.1, Reason: "buy",
		OrderID: 99, FillID: 1, FilledVolume: 1000,
	})

	pairs := []ShadowPair{
		{ID: "case1_equal", Candidate: c1, Plan: p1},
		{ID: "case2_shares", Candidate: c2, Plan: p2},
		{ID: "case3_status_ignored", Candidate: c3, Plan: p3},
		{ID: "case4_forbidden", Candidate: c4, Plan: p4},
	}
	report := BuildShadowBatchReport(pairs)
	report.GeneratedAt = "2026-07-21T00:00:00Z"

	// assertions for cases
	if !report.Pairs[0].Equal {
		t.Fatal("case1")
	}
	if report.Pairs[1].Equal {
		t.Fatal("case2")
	}
	// case3: executing flags forbidden but shares/side equal — Equal true if no "different"
	if !report.Pairs[2].Equal {
		// executing adds forbidden_field only; Equal should still true
		t.Fatalf("case3 equal=%v diffs=%+v", report.Pairs[2].Equal, report.Pairs[2].DifferentFields)
	}
	hasForbidden := false
	for _, d := range report.Pairs[3].DifferentFields {
		if d.Kind == "forbidden_field" {
			hasForbidden = true
		}
	}
	if !hasForbidden {
		t.Fatal("case4 forbidden")
	}

	got, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "tradeplan_candidate_shadow_report.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, got, 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, got, 0o644)
		t.Logf("wrote %s", path)
		return
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nWANT:\n%s\nGOT:\n%s", want, got)
	}
}
