package authority

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

func TestAuthorityGolden(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := &models.QuantDecision{
		AsOf:       asOf,
		TradeDate:  "2026-07-21",
		Instrument: models.QuantInstrument{StockCode: "sz000001"},
		Signal:     models.QuantSignal{Tag: "强", TagKind: models.QuantTagKindEntry},
		Gate:       models.QuantGate{Ready: true, Score: 0.9, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk:       models.QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size:       models.QuantSize{OK: true, TargetShares: 1000},
		Action:     models.QuantAction{Code: models.QuantActionEnter, Label: "可买", Side: "buy"},
		Meta: models.QuantDecisionMeta{
			SchemaVersion: 1,
			Producer:      models.QuantProducerJSLegacy,
		},
	}
	reg := registry.New()
	id, _ := reg.Put(d)
	okRead, _ := reg.Get(id)
	mut := *okRead
	mut.Action.Code = models.QuantActionExitPartial
	mutRes := VerifyDecisionReadOnlyUsage(reg, id, &mut)
	okRes := VerifyDecisionReadOnlyUsage(reg, id, okRead)

	type caseRow struct {
		ID       string               `json:"id"`
		Role     DecisionConsumerRole `json:"role,omitempty"`
		Expect   string               `json:"expect"`
		Allowed  *bool                `json:"allowed,omitempty"`
		Mutation *bool                `json:"mutationDetected,omitempty"`
		Reason   string               `json:"reason,omitempty"`
	}
	trueV, falseV := true, false

	cases := []caseRow{
		{ID: "A_UI_CARD_read", Role: RoleUICard, Expect: "allowed", Allowed: &trueV, Reason: Authorize(RoleUICard).Reason},
		{ID: "B_ALERT_read", Role: RoleAlert, Expect: "allowed", Allowed: &trueV, Reason: Authorize(RoleAlert).Reason},
		{ID: "C_EXECUTION_access", Role: RoleExecutionForbidden, Expect: "denied", Allowed: &falseV, Reason: Authorize(RoleExecutionForbidden).Reason},
		{ID: "D_TradePlan_future_read_only", Role: RoleTradePlanDraftFuture, Expect: "read_only", Allowed: &trueV, Reason: Authorize(RoleTradePlanDraftFuture).Reason},
		{ID: "E_Decision_mutation", Expect: "blocked", Mutation: &trueV, Reason: mutRes.Summary},
		{ID: "E0_Decision_read_ok", Expect: "ok", Mutation: &falseV, Reason: okRes.Summary},
	}

	matrix := map[string]DecisionPermission{}
	for _, r := range AllRoles() {
		matrix[string(r)] = PermissionFor(r)
	}

	golden := map[string]any{
		"phase":   "Phase3-D",
		"harness": "quant-decision-authority-boundary",
		"matrix":  matrix,
		"cases":   cases,
		"rules": map[string]any{
			"decisionIsFactNotAuthorization": true,
			"createTradeAlwaysDenied":        true,
			"forbiddenImporters":             ForbiddenDecisionImporters,
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
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "authority_golden.json")
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

	if !mutRes.MutationDetected || okRes.MutationDetected {
		t.Fatalf("mutation cases: mut=%+v ok=%+v", mutRes, okRes)
	}
}
