package draft

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go-stock/backend/decision/registry"
	"go-stock/backend/models"
)

func sampleSnap(t *testing.T) (registry.Snapshot, *registry.Registry) {
	t.Helper()
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := &models.QuantDecision{
		AsOf:       asOf,
		TradeDate:  "2026-07-21",
		Purpose:    models.QuantPurposeWatchlist,
		Instrument: models.QuantInstrument{StockCode: "sz000001", StockName: "PingAn"},
		Regime:     models.QuantRegime{Level: 3, Key: "level3", ExposureCap: 0.2},
		Signal:     models.QuantSignal{Tag: "强", TagKind: models.QuantTagKindEntry},
		EntryZone: &models.QuantEntryZone{
			Low: 10, High: 10.2, InstantPrice: 10.1,
			Mode: models.QuantZoneModeNear, DeferMode: models.QuantDeferSame, Text: "10.00~10.20",
		},
		Gate: models.QuantGate{Score: 0.9, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk: models.QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size: models.QuantSize{
			OK: true, TargetShares: 1000, AddShares: 1000, TargetAmount: 101000,
			PositionPct: 12.5, StopPrice: 9.85, EntryPrice: 10.1,
		},
		Action: models.QuantAction{Code: models.QuantActionEnter, Label: "可买", AllowDraft: true, Side: "buy"},
		Meta: models.QuantDecisionMeta{
			SchemaVersion: models.QuantDecisionSchemaVersion,
			Producer:      models.QuantProducerJSLegacy,
		},
	}
	reg := registry.New()
	id, err := reg.Put(d)
	if err != nil {
		t.Fatal(err)
	}
	snap, ok := reg.GetSnapshot(id)
	if !ok {
		t.Fatal("snapshot missing")
	}
	return snap, reg
}

func TestDecisionToTradePlanDraft_ProjectsSourceAndHash(t *testing.T) {
	snap, _ := sampleSnap(t)
	draft, err := DecisionToTradePlanDraft(snap)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != DraftCreated {
		t.Fatalf("status=%s", draft.Status)
	}
	if draft.SourceDecisionID != snap.DecisionID || draft.SourceSnapshotHash != snap.Hash {
		t.Fatalf("source bind: %+v", draft)
	}
	if draft.EnableExecute {
		t.Fatal("EnableExecute must be false")
	}
	if draft.Payload.ActionCode != models.QuantActionEnter || draft.Payload.TargetShares != 1000 {
		t.Fatalf("payload: %+v", draft.Payload)
	}
	res := ValidateTradePlanDraft(draft, snap)
	if !res.OK || draft.Status != DraftValidated {
		t.Fatalf("validate: %+v draft=%+v", res, draft)
	}
	if err := VerifyDraftImmutable(draft); err != nil {
		t.Fatal(err)
	}
}

func TestDraftImmutable_DetectsMutation(t *testing.T) {
	snap, _ := sampleSnap(t)
	draft, err := DecisionToTradePlanDraft(snap)
	if err != nil {
		t.Fatal(err)
	}
	draft.Payload.TargetShares = 1
	draft.Payload.ActionCode = models.QuantActionExitPartial
	res := VerifyDraftIntegrity(draft)
	if res.OK || len(res.Codes) == 0 || res.Codes[0] != CodeDraftMutationDetected {
		t.Fatalf("expected draft_mutation_detected: %+v", res)
	}
}

func TestDraftProjection_DoesNotMutateDecisionOrCandidateRank(t *testing.T) {
	snap, reg := sampleSnap(t)
	beforeHash := snap.Hash
	beforeCode := snap.Decision.Action.Code
	item := models.CandidatePoolItem{
		StockCode: "sz000001", Rank: 1, Score: 0.91, DecisionID: snap.DecisionID,
	}
	draft, err := DecisionToTradePlanDraft(snap)
	if err != nil {
		t.Fatal(err)
	}
	draft.Payload.TargetShares = 999
	_ = ValidateTradePlanDraft(draft, snap)

	again, ok := reg.GetSnapshot(snap.DecisionID)
	if !ok || again.Hash != beforeHash || again.Decision.Action.Code != beforeCode {
		t.Fatalf("decision mutated: %+v", again)
	}
	if item.Rank != 1 || item.Score != 0.91 {
		t.Fatalf("candidate Rank/Score mutated: %+v", item)
	}
}

func TestValidateDraftSource_RejectsHashMismatch(t *testing.T) {
	snap, _ := sampleSnap(t)
	draft, _ := DecisionToTradePlanDraft(snap)
	draft.SourceSnapshotHash = "deadbeef"
	draft.SnapshotHash = "deadbeef"
	draft.DraftHash = ComputeDraftHash(draft)
	res := ValidateTradePlanDraft(draft, snap)
	if res.OK {
		t.Fatal("expected failure")
	}
	found := false
	for _, c := range res.Codes {
		if c == CodeSnapshotChanged {
			found = true
		}
	}
	if !found {
		t.Fatalf("codes=%v", res.Codes)
	}
}

func TestValidateTradePlanDraft_ExecuteForbidden(t *testing.T) {
	snap, _ := sampleSnap(t)
	draft, _ := DecisionToTradePlanDraft(snap)
	draft.EnableExecute = true
	res := ValidateTradePlanDraft(draft, snap)
	if res.OK {
		t.Fatal("expected reject")
	}
	found := false
	for _, c := range res.Codes {
		if c == CodeExecuteForbidden {
			found = true
		}
	}
	if !found {
		t.Fatalf("codes=%v", res.Codes)
	}
	if draft.EnableExecute {
		t.Fatal("EnableExecute must be forced false")
	}
}

func TestVerifyDraftAuthority(t *testing.T) {
	auth := VerifyDraftAuthority()
	if !auth.OK || !auth.CanReadDecision || auth.CanModifyDecision || auth.CanAuthorizeExecute {
		t.Fatalf("%+v", auth)
	}
}

func TestDecisionDraftGolden(t *testing.T) {
	snap, _ := sampleSnap(t)
	draft, err := DecisionToTradePlanDraft(snap)
	if err != nil {
		t.Fatal(err)
	}
	draft.CreatedAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	draft.DraftHash = ComputeDraftHash(draft)
	res := ValidateTradePlanDraft(draft, snap)
	if !res.OK {
		t.Fatal(res.Message)
	}

	golden := map[string]any{
		"phase":   "Phase4-A",
		"harness": "quant-decision-to-tradeplan-draft",
		"source": map[string]any{
			"decisionId":   snap.DecisionID,
			"snapshotHash": snap.Hash,
			"actionCode":   snap.Decision.Action.Code,
		},
		"draft": map[string]any{
			"sourceDecisionId": draft.SourceDecisionID,
			"snapshotHash":     draft.SnapshotHash,
			"draftHash":        draft.DraftHash,
			"tradeDate":        draft.Payload.TradeDate,
			"stockCode":        draft.Payload.StockCode,
			"side":             draft.Payload.Side,
			"actionCode":       draft.Payload.ActionCode,
			"allowDraft":       draft.Payload.AllowDraft,
			"targetShares":     draft.Payload.TargetShares,
			"targetAmount":     draft.Payload.TargetAmount,
			"enableExecute":    draft.EnableExecute,
			"status":           string(draft.Status),
			"consumerRole":     draft.ConsumerRole,
			"gateReady":        draft.Payload.GateReady,
			"marketLevel":      draft.Payload.MarketLevel,
		},
		"validateSourceOK": true,
		"immutableOK":      VerifyDraftImmutable(draft) == nil,
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
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "decision_to_draft_golden.json")
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
		t.Fatalf("golden mismatch\nWANT:\n%s\nGOT:\n%s\n(set UPDATE_GOLDEN=1)", want, got)
	}
}

func TestDraftLifecycleGolden(t *testing.T) {
	snap, reg := sampleSnap(t)

	// Case1: CREATED → VALIDATED
	d1, _ := DecisionToTradePlanDraft(snap)
	d1.CreatedAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d1.DraftHash = ComputeDraftHash(d1)
	createdStatus := string(d1.Status)
	v1 := ValidateTradePlanDraft(d1, snap)

	// Case2: snapshot_changed
	d2, _ := DecisionToTradePlanDraft(snap)
	d2.CreatedAt = d1.CreatedAt
	d2.SourceSnapshotHash = "changed-hash"
	d2.SnapshotHash = "changed-hash"
	d2.DraftHash = ComputeDraftHash(d2)
	v2 := ValidateTradePlanDraft(d2, snap)

	// Case3: draft_mutation_detected
	d3, _ := DecisionToTradePlanDraft(snap)
	d3.CreatedAt = d1.CreatedAt
	d3.DraftHash = ComputeDraftHash(d3)
	d3.Payload.TargetShares = 42
	v3 := VerifyDraftIntegrity(d3)

	// Case4: execute_forbidden
	d4, _ := DecisionToTradePlanDraft(snap)
	d4.CreatedAt = d1.CreatedAt
	d4.EnableExecute = true
	d4.DraftHash = ComputeDraftHash(d4)
	v4 := ValidateTradePlanDraft(d4, snap)

	// Decision / Rank untouched
	item := models.CandidatePoolItem{Rank: 3, Score: 0.5}
	again, _ := reg.GetSnapshot(snap.DecisionID)
	rankOK := item.Rank == 3 && item.Score == 0.5 && again.Hash == snap.Hash

	auth := VerifyDraftAuthority()

	golden := map[string]any{
		"phase":   "Phase4-B",
		"harness": "quant-tradeplan-draft-lifecycle",
		"case1_lifecycle": map[string]any{
			"from":               createdStatus,
			"to":                 string(v1.Status),
			"ok":                 v1.OK,
			"draftId":            d1.DraftID,
			"sourceDecisionId":   d1.SourceDecisionID,
			"sourceSnapshotHash": d1.SourceSnapshotHash,
		},
		"case2_snapshot_changed": map[string]any{
			"ok":    v2.OK,
			"codes": v2.Codes,
		},
		"case3_draft_mutation": map[string]any{
			"ok":    v3.OK,
			"codes": v3.Codes,
		},
		"case4_execute_forbidden": map[string]any{
			"ok":            v4.OK,
			"codes":         v4.Codes,
			"enableExecute": d4.EnableExecute,
		},
		"authority": map[string]any{
			"ok":                  auth.OK,
			"canReadDecision":     auth.CanReadDecision,
			"canModifyDecision":   auth.CanModifyDecision,
			"canAuthorizeExecute": auth.CanAuthorizeExecute,
		},
		"decisionAndRankUntouched": rankOK,
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
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "draft_lifecycle_golden.json")
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
		t.Fatalf("lifecycle golden mismatch\nWANT:\n%s\nGOT:\n%s", want, got)
	}
}
