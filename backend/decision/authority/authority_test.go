package authority

import (
	"testing"
	"time"

	"go-stock/backend/decision/registry"
	"go-stock/backend/models"
)

func TestAuthorityMatrix_ConsumerPermissions(t *testing.T) {
	cases := []struct {
		role    DecisionConsumerRole
		allowed bool
		read    bool
		create  bool
	}{
		{RoleUICard, true, true, false},
		{RoleAlert, true, true, false},
		{RoleKline, true, true, false},
		{RoleObservability, true, true, false},
		{RoleShadowCompare, true, true, false},
		{RoleTradePlanDraftFuture, true, true, false},
		{RoleTradePlanCandidateShadow, true, true, false},
		{RoleExecutionForbidden, false, false, false},
	}
	for _, tc := range cases {
		ad := Authorize(tc.role)
		if ad.Allowed != tc.allowed {
			t.Fatalf("%s allowed=%v want %v (%s)", tc.role, ad.Allowed, tc.allowed, ad.Reason)
		}
		if ad.Permission.ReadDecision != tc.read {
			t.Fatalf("%s read=%v want %v", tc.role, ad.Permission.ReadDecision, tc.read)
		}
		if ad.Permission.WriteDecision || ad.Permission.ModifyAction || ad.Permission.CreateTrade {
			t.Fatalf("%s must not write/modify/createTrade: %+v", tc.role, ad.Permission)
		}
		ct := AuthorizeCreateTrade(tc.role)
		if ct.Allowed {
			t.Fatalf("%s CreateTrade must be denied", tc.role)
		}
	}
	shadow := PermissionFor(RoleTradePlanCandidateShadow)
	if !shadow.CreateCandidate {
		t.Fatal("TRADEPLAN_CANDIDATE_SHADOW must CreateCandidate")
	}
	if !AuthorizeCreateCandidate(RoleTradePlanCandidateShadow).Allowed {
		t.Fatal("AuthorizeCreateCandidate shadow")
	}
	if AuthorizeCreateCandidate(RoleUICard).Allowed {
		t.Fatal("UI must not CreateCandidate")
	}
}

func TestVerifyDecisionReadOnlyUsage_OKAndMutation(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := &models.QuantDecision{
		AsOf:       asOf,
		TradeDate:  "2026-07-21",
		Purpose:    models.QuantPurposeWatchlist,
		Instrument: models.QuantInstrument{StockCode: "sz000001"},
		Signal:     models.QuantSignal{Tag: "强", TagKind: models.QuantTagKindEntry},
		Gate:       models.QuantGate{Score: 0.9, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk:       models.QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size:       models.QuantSize{OK: true, TargetShares: 1000},
		Action:     models.QuantAction{Code: models.QuantActionEnter, Label: "可买", AllowDraft: true, Side: "buy"},
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

	readOK, ok := reg.Get(id)
	if !ok {
		t.Fatal("missing")
	}
	mr := VerifyDecisionReadOnlyUsage(reg, id, readOK)
	if mr.MutationDetected {
		t.Fatalf("read should OK: %+v", mr)
	}

	mut := *readOK
	mut.Action.Code = models.QuantActionExitPartial
	mr2 := VerifyDecisionReadOnlyUsage(reg, id, &mut)
	if !mr2.MutationDetected {
		t.Fatalf("expected mutation_detected: %+v", mr2)
	}
	if mr2.SealedActionCode != models.QuantActionEnter || mr2.ObservedActionCode != models.QuantActionExitPartial {
		t.Fatalf("action codes: %+v", mr2)
	}

	item := models.CandidatePoolItem{StockCode: "sz000001", Rank: 2, Score: 0.77, DecisionID: id}
	_ = VerifyDecisionReadOnlyUsage(reg, id, &mut)
	if item.Rank != 2 || item.Score != 0.77 {
		t.Fatalf("Rank/Score mutated: %+v", item)
	}
}
