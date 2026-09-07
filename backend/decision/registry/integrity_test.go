package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"go-stock/backend/models"
)

func TestPutImmutable_IdempotentSameContent(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	reg := New()

	r1, err := reg.PutImmutable(d)
	if err != nil || !r1.Created || r1.Hash == "" {
		t.Fatalf("first put: %+v err=%v", r1, err)
	}
	r2, err := reg.PutImmutable(d)
	if err != nil || !r2.Idempotent || r2.Conflict != nil {
		t.Fatalf("second put should be idempotent: %+v err=%v", r2, err)
	}
	if reg.ConflictCount() != 0 {
		t.Fatalf("conflicts=%d", reg.ConflictCount())
	}
	v := reg.Verify(r1.DecisionID)
	if !v.Intact {
		t.Fatalf("verify: %+v", v)
	}
}

func TestPutImmutable_RefuseOverwriteDifferentContent(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d1 := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	reg := New()
	r1, _ := reg.PutImmutable(d1)

	// Force same DecisionID but different semantic content (mutate gate after id fixed)
	d2 := cloneDecision(d1)
	d2.ID = r1.DecisionID
	d2.Gate.Ready = false
	d2.Gate.Score = 0.1
	d2.Action.Code = models.QuantActionWatch

	r2, err := reg.PutImmutable(d2)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Conflict == nil || r2.Conflict.Kind != ConflictKindIDContent {
		t.Fatalf("expected id_content_conflict: %+v", r2)
	}
	// original preserved
	got, ok := reg.Get(r1.DecisionID)
	if !ok || got.Action.Code != models.QuantActionEnter || !got.Gate.Ready {
		t.Fatalf("original mutated: %+v", got)
	}
	if !reg.Verify(r1.DecisionID).Intact {
		t.Fatal("sealed snapshot not intact")
	}
}

func TestPutImmutable_ActionCodeConflictSameCodeAsOf(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 1, 0, 0, 0, time.UTC)
	d1 := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	d2 := sampleDecision("sz000001", "2026-07-21", models.QuantActionWaitPullback, "等回踩", asOf)
	// different labels → different DecisionIDs; same code+asOf
	reg := New()
	if _, err := reg.PutImmutable(d1); err != nil {
		t.Fatal(err)
	}
	r2, err := reg.PutImmutable(d2)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Conflict == nil || r2.Conflict.Kind != ConflictKindActionCode {
		t.Fatalf("expected action_code_conflict: %+v", r2)
	}
	if reg.Count() != 2 {
		t.Fatalf("both snapshots should be archived, count=%d", reg.Count())
	}
}

func TestSnapshotIntegrity_DoesNotChangeCandidateRankScore(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	reg := New()
	id, _ := reg.Put(d)

	items := []models.CandidatePoolItem{
		{StockCode: "sz000001", TradeDate: "2026-07-21", Rank: 1, Score: 0.91},
	}
	models.EnrichCandidatePoolDecisionIDs(items, map[string]string{"sz000001": id})
	rank, score := items[0].Rank, items[0].Score

	snap, ok := reg.GetSnapshot(id)
	if !ok || snap.Hash == "" {
		t.Fatal("snapshot missing")
	}
	_ = reg.Verify(id)
	_ = reg.Conflicts()
	tr := reg.TraceItem(items[0])
	if !tr.Found {
		t.Fatalf("trace: %+v", tr)
	}

	if items[0].Rank != rank || items[0].Score != score {
		t.Fatalf("Rank/Score mutated: %+v", items[0])
	}
}

func TestIntegrityGolden(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d1 := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	reg := New()
	r1, _ := reg.PutImmutable(d1)

	dBad := cloneDecision(d1)
	dBad.ID = r1.DecisionID
	dBad.Gate.Ready = false
	rBad, _ := reg.PutImmutable(dBad)

	d2 := sampleDecision("sz000001", "2026-07-21", models.QuantActionWatch, "观察", asOf)
	r2, _ := reg.PutImmutable(d2)

	v := reg.Verify(r1.DecisionID)
	conflicts := reg.Conflicts()
	kinds := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		kinds = append(kinds, c.Kind)
	}
	sort.Strings(kinds)

	golden := map[string]any{
		"phase":   "Phase3-C",
		"harness": "quant-decision-snapshot-integrity",
		"sealed": map[string]any{
			"decisionId": r1.DecisionID,
			"hash":       r1.Hash,
			"intact":     v.Intact,
		},
		"overwriteAttempt": map[string]any{
			"conflictKind": rBad.Conflict.Kind,
			"keptHash":     rBad.Hash,
		},
		"actionConflict": map[string]any{
			"created":      r2.Created,
			"conflictKind": r2.Conflict.Kind,
			"left":         r2.Conflict.LeftActionCode,
			"right":        r2.Conflict.RightActionCode,
		},
		"conflictKinds": kinds,
		"count":         reg.Count(),
		"conflictCount": reg.ConflictCount(),
	}

	got, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "snapshot_integrity_golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, got, 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, got, 0o644)
		t.Logf("wrote initial golden %s", path)
		return
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nWANT:\n%s\nGOT:\n%s", want, got)
	}
}
