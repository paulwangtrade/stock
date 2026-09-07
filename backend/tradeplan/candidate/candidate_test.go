package candidate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go-stock/backend/decision/registry"
	"go-stock/backend/models"
	"go-stock/backend/tradeplan/draft"
)

func validatedDraft(t *testing.T) (draft.TradePlanDraft, registry.Snapshot) {
	t.Helper()
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := &models.QuantDecision{
		AsOf:       asOf,
		TradeDate:  "2026-07-21",
		Instrument: models.QuantInstrument{StockCode: "sz000001", StockName: "PingAn"},
		Regime:     models.QuantRegime{Level: 3, ExposureCap: 0.2},
		Signal:     models.QuantSignal{Tag: "强", TagKind: models.QuantTagKindEntry},
		EntryZone: &models.QuantEntryZone{
			Low: 10, High: 10.2, InstantPrice: 10.1,
			Mode: models.QuantZoneModeNear, DeferMode: models.QuantDeferSame,
		},
		Gate:   models.QuantGate{Ready: true, Score: 0.9, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk:   models.QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size:   models.QuantSize{OK: true, TargetShares: 1000, TargetAmount: 101000, StopPrice: 9.85, EntryPrice: 10.1},
		Action: models.QuantAction{Code: models.QuantActionEnter, Label: "可买", AllowDraft: true, Side: "buy"},
		Meta:   models.QuantDecisionMeta{SchemaVersion: 1, Producer: models.QuantProducerJSLegacy},
	}
	reg := registry.New()
	id, err := reg.Put(d)
	if err != nil {
		t.Fatal(err)
	}
	snap, _ := reg.GetSnapshot(id)
	dr, err := draft.DecisionToTradePlanDraft(snap)
	if err != nil {
		t.Fatal(err)
	}
	dr.CreatedAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	dr.DraftHash = draft.ComputeDraftHash(dr)
	res := draft.ValidateTradePlanDraft(dr, snap)
	if !res.OK {
		t.Fatalf("draft validate: %+v", res)
	}
	return *dr, snap
}

func TestDraftToTradePlanCandidate_OK(t *testing.T) {
	dr, _ := validatedDraft(t)
	c, err := DraftToTradePlanCandidate(dr)
	if err != nil {
		t.Fatal(err)
	}
	if c.Executable {
		t.Fatal("Executable must be false")
	}
	if c.SourceDecisionID != dr.SourceDecisionID || c.SourceSnapshotHash != dr.SourceSnapshotHash {
		t.Fatalf("provenance: %+v", c)
	}
	if c.IntentKind != IntentEnterHint || c.TargetShares != 1000 {
		t.Fatalf("mapping: %+v", c)
	}
	if c.EntryPriceHint != 10.1 || c.StopPriceHint != 9.85 {
		t.Fatalf("prices: %+v", c)
	}
	v := ValidateTradePlanCandidate(c, dr)
	if !v.OK {
		t.Fatal(v.Message)
	}
	if !VerifyCandidateIntegrity(c).OK {
		t.Fatal("integrity")
	}
}

func TestDraftToTradePlanCandidate_NotValidated(t *testing.T) {
	dr, _ := validatedDraft(t)
	dr.Status = draft.DraftCreated
	dr.DraftHash = draft.ComputeDraftHash(&dr)
	_, err := DraftToTradePlanCandidate(dr)
	if err == nil || !containsCode(err.Error(), CodeDraftNotValidated) {
		t.Fatalf("err=%v", err)
	}
}

func TestCandidate_ExecuteForbidden(t *testing.T) {
	dr, _ := validatedDraft(t)
	c, err := DraftToTradePlanCandidate(dr)
	if err != nil {
		t.Fatal(err)
	}
	c.Executable = true
	if err := RejectExecutableInjection(&c); err == nil {
		t.Fatal("expected execute forbidden")
	}
	if c.Executable {
		t.Fatal("must force false")
	}
	v := VerifyCandidateIntegrity(c)
	// hash still ok but Executable check in integrity
	c.Executable = true
	v = VerifyCandidateIntegrity(c)
	if v.OK || v.Codes[0] != CodeCandidateExecuteForbidden {
		t.Fatalf("%+v", v)
	}
}

func TestCandidate_MutationDetected(t *testing.T) {
	dr, _ := validatedDraft(t)
	c, _ := DraftToTradePlanCandidate(dr)
	c.TargetShares = 1
	v := VerifyCandidateIntegrity(c)
	if v.OK || v.Codes[0] != CodeCandidateMutationDetected {
		t.Fatalf("%+v", v)
	}
}

func TestVerifyCandidateAuthority(t *testing.T) {
	a := VerifyCandidateAuthority()
	if !a.OK || !a.CanReadDecision || !a.CanCreateCandidate || a.CanModifyDecision || a.CanAuthorizeExecute {
		t.Fatalf("%+v", a)
	}
}

func TestMapActionCodeToIntentKind(t *testing.T) {
	if MapActionCodeToIntentKind(models.QuantActionWaitPullback) != IntentWaitHint {
		t.Fatal("wait")
	}
	if MapActionCodeToIntentKind(models.QuantActionScaleIn) != IntentScaleInHint {
		t.Fatal("scale")
	}
}

func TestTradePlanCandidateGolden(t *testing.T) {
	dr, _ := validatedDraft(t)

	// Case1 OK
	c1, err := DraftToTradePlanCandidate(dr)
	if err != nil {
		t.Fatal(err)
	}
	c1.CreatedAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	c1.CandidateHash = ComputeCandidateHash(c1)

	// Case2 not validated
	dr2 := dr
	dr2.Status = draft.DraftCreated
	_, err2 := DraftToTradePlanCandidate(dr2)

	// Case3 execute injection
	c3 := c1
	c3.Executable = true
	err3 := RejectExecutableInjection(&c3)

	// Case4 mutation
	c4 := c1
	c4.TargetShares = 99
	v4 := VerifyCandidateIntegrity(c4)

	auth := VerifyCandidateAuthority()

	golden := map[string]any{
		"phase":   "Phase5-B",
		"harness": "quant-tradeplan-candidate-shadow",
		"case1_ok": map[string]any{
			"ok":                 err == nil,
			"executable":         c1.Executable,
			"intentKind":         c1.IntentKind,
			"targetShares":       c1.TargetShares,
			"sourceDecisionId":   c1.SourceDecisionID,
			"sourceSnapshotHash": c1.SourceSnapshotHash,
			"side":               c1.Side,
			"entryPriceHint":     c1.EntryPriceHint,
			"stopPriceHint":      c1.StopPriceHint,
			"candidateId":        c1.CandidateID,
		},
		"case2_draft_not_validated": map[string]any{
			"ok":   err2 == nil,
			"code": CodeDraftNotValidated,
			"err":  errString(err2),
		},
		"case3_execute_forbidden": map[string]any{
			"ok":         err3 == nil,
			"code":       CodeCandidateExecuteForbidden,
			"executable": c3.Executable,
			"err":        errString(err3),
		},
		"case4_mutation": map[string]any{
			"ok":    v4.OK,
			"codes": v4.Codes,
		},
		"authority": map[string]any{
			"ok":                  auth.OK,
			"canReadDecision":     auth.CanReadDecision,
			"canCreateCandidate":  auth.CanCreateCandidate,
			"canModifyDecision":   auth.CanModifyDecision,
			"canAuthorizeExecute": auth.CanAuthorizeExecute,
		},
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
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "tradeplan_candidate_golden.json")
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

func containsCode(s, code string) bool {
	return strings.Contains(s, code)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
