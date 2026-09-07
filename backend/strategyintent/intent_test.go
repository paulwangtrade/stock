package strategyintent_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/strategyintent"

	"github.com/stretchr/testify/require"
)

func emptyIntentStore(t *testing.T) {
	t.Helper()
	strategyintent.SetStoreForTest(strategyintent.NewMemoryStoreForTest(nil))
	strategyintent.SetSchemaRevisionLookupForTest(strategyintent.DefaultTestSchemaLookup())
	t.Cleanup(func() {
		strategyintent.ResetStoreForTest()
		strategyintent.SetSchemaRevisionLookupForTest(nil)
	})
}

func sampleCreateReq(candidateID string) strategyintent.CreateDraftRequest {
	return strategyintent.CreateDraftRequest{
		CandidateID: candidateID,
		StrategySchemaRef: strategyintent.SchemaRef{
			Unbound:    false,
			StrategyID: "sdef:trend_breakout",
			Revision:   "v1",
		},
		SchemaRevision: "v1",
		IntentType:     strategyintent.IntentTypeManual,
		Action:         strategyintent.Action{Verb: "consider_buy", Text: "观察买入"},
		Conditions:     strategyintent.Conditions{Session: "open"},
	}
}

func TestLifecycleEnum(t *testing.T) {
	for _, st := range []string{
		strategyintent.StatusDraft,
		strategyintent.StatusReviewing,
		strategyintent.StatusApproved,
		strategyintent.StatusExpired,
		strategyintent.StatusDiscarded,
	} {
		require.True(t, strategyintent.ValidStatuses[st], st)
	}
	require.False(t, strategyintent.ValidStatuses["ready"])
	require.False(t, strategyintent.ValidStatuses["active"])
	require.False(t, strategyintent.ValidStatuses["frozen"])

	require.True(t, strategyintent.CanTransition(strategyintent.StatusDraft, strategyintent.StatusReviewing))
	require.True(t, strategyintent.CanTransition(strategyintent.StatusReviewing, strategyintent.StatusApproved))
	require.True(t, strategyintent.CanTransition(strategyintent.StatusApproved, strategyintent.StatusExpired))
	require.False(t, strategyintent.CanTransition(strategyintent.StatusApproved, strategyintent.StatusDraft))
	require.False(t, strategyintent.CanTransition(strategyintent.StatusExpired, strategyintent.StatusApproved))
}

func TestSchemaRevisionReference(t *testing.T) {
	require.NoError(t, strategyintent.ValidateSchemaRef(strategyintent.SchemaRef{
		Unbound:    false,
		StrategyID: "sdef:trend_breakout",
		Revision:   "v1",
	}, "v1"))

	err := strategyintent.ValidateSchemaRef(strategyintent.SchemaRef{
		Unbound:    false,
		StrategyID: "sdef:trend_breakout",
	}, "")
	require.Error(t, err)
	var ve strategyintent.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, strategyintent.CodeInvalidSchemaRef, ve.Code)

	err = strategyintent.ValidateSchemaRef(strategyintent.SchemaRef{
		Unbound:    false,
		StrategyID: "sdef:trend_breakout",
		Revision:   "current",
	}, "current")
	require.Error(t, err)
	require.ErrorAs(t, err, &ve)
	require.Equal(t, strategyintent.CodeForbiddenRevision, ve.Code)

	err = strategyintent.ValidateSchemaRef(strategyintent.SchemaRef{
		Unbound:    false,
		StrategyID: "sdef:x",
		Revision:   "latest",
	}, "")
	require.Error(t, err)

	err = strategyintent.ValidateSchemaRef(strategyintent.SchemaRef{
		Unbound:    false,
		StrategyID: "sdef:x",
		Revision:   "v1",
	}, "v2")
	require.Error(t, err)

	require.NoError(t, strategyintent.ValidateSchemaRef(strategyintent.SchemaRef{
		Unbound: true,
		Note:    "no schema yet",
	}, ""))
}

func TestDomainValidation(t *testing.T) {
	now := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	ok := strategyintent.Intent{
		ID:            "si:demo",
		SchemaVersion: strategyintent.SchemaVersion,
		CandidateID:   "rc:signal:2026-08-17:600519",
		StrategySchemaRef: strategyintent.SchemaRef{
			Unbound:    false,
			StrategyID: "sdef:trend_breakout",
			Revision:   "v1",
		},
		SchemaRevision: "v1",
		IntentType:     strategyintent.IntentTypeManual,
		Action:         strategyintent.Action{Verb: "consider_buy"},
		Status:         strategyintent.StatusApproved,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, strategyintent.ValidateIntent(ok))

	bad := ok
	bad.Status = "ready"
	require.Error(t, strategyintent.ValidateIntent(bad))

	bad2 := ok
	bad2.CandidateID = ""
	require.Error(t, strategyintent.ValidateIntent(bad2))

	bad3 := ok
	bad3.StrategySchemaRef.Revision = "current"
	bad3.SchemaRevision = "current"
	require.Error(t, strategyintent.ValidateIntent(bad3))
}

func TestWrite_CreateUpdateSubmitApprove(t *testing.T) {
	emptyIntentStore(t)
	cid := "rc:signal:2026-08-17:000001"

	created, err := strategyintent.CreateDraft(sampleCreateReq(cid))
	require.NoError(t, err)
	require.Equal(t, strategyintent.StatusDraft, created.Intent.Status)
	require.Equal(t, "v1", created.Intent.SchemaRevision)

	summary := "updated summary"
	updated, err := strategyintent.UpdateDraft(created.Intent.ID, strategyintent.UpdateDraftRequest{
		Summary: &summary,
	})
	require.NoError(t, err)
	require.Equal(t, "updated summary", updated.Intent.Summary)

	submitted, err := strategyintent.SubmitReview(created.Intent.ID)
	require.NoError(t, err)
	require.Equal(t, strategyintent.StatusReviewing, submitted.Intent.Status)

	_, err = strategyintent.UpdateDraft(created.Intent.ID, strategyintent.UpdateDraftRequest{
		Summary: &summary,
	})
	require.Error(t, err)
	var we strategyintent.WriteError
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyintent.CodeIntentImmutable, we.Code)

	approved, err := strategyintent.Approve(created.Intent.ID)
	require.NoError(t, err)
	require.Equal(t, strategyintent.StatusApproved, approved.Intent.Status)
}

func TestWrite_FreezeApproved(t *testing.T) {
	emptyIntentStore(t)
	cid := "rc:signal:2026-08-17:000002"
	created, err := strategyintent.CreateDraft(sampleCreateReq(cid))
	require.NoError(t, err)
	_, err = strategyintent.SubmitReview(created.Intent.ID)
	require.NoError(t, err)
	_, err = strategyintent.Approve(created.Intent.ID)
	require.NoError(t, err)

	_, err = strategyintent.UpdateDraft(created.Intent.ID, strategyintent.UpdateDraftRequest{
		Action: &strategyintent.Action{Verb: "avoid"},
	})
	require.Error(t, err)
	var we strategyintent.WriteError
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyintent.CodeIntentImmutable, we.Code)

	_, err = strategyintent.CreateDraft(sampleCreateReq(cid))
	require.Error(t, err)
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyintent.CodeCreateNotAllowed, we.Code)
}

func TestWrite_InvalidTransition(t *testing.T) {
	emptyIntentStore(t)
	cid := "rc:signal:2026-08-17:000003"
	created, err := strategyintent.CreateDraft(sampleCreateReq(cid))
	require.NoError(t, err)

	_, err = strategyintent.Approve(created.Intent.ID)
	require.Error(t, err)
	var we strategyintent.WriteError
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyintent.CodeInvalidTransition, we.Code)
}

func TestWrite_SchemaBindingOnCreate(t *testing.T) {
	emptyIntentStore(t)
	_, err := strategyintent.CreateDraft(strategyintent.CreateDraftRequest{
		CandidateID: "rc:x",
		StrategySchemaRef: strategyintent.SchemaRef{
			Unbound:    false,
			StrategyID: "sdef:trend_breakout",
			Revision:   "current",
		},
		SchemaRevision: "current",
		Action:         strategyintent.Action{Verb: "watch"},
	})
	require.Error(t, err)
}

func TestWrite_SidecarReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategy_intents.json")
	strategyintent.SetStoreForTest(strategyintent.NewFileStoreForTest(path))
	strategyintent.SetSchemaRevisionLookupForTest(strategyintent.DefaultTestSchemaLookup())
	t.Cleanup(func() {
		strategyintent.ResetStoreForTest()
		strategyintent.SetSchemaRevisionLookupForTest(nil)
	})

	cid := "rc:signal:2026-08-17:sidecar"
	created, err := strategyintent.CreateDraft(sampleCreateReq(cid))
	require.NoError(t, err)
	require.FileExists(t, path)

	summary := "persist"
	_, err = strategyintent.UpdateDraft(created.Intent.ID, strategyintent.UpdateDraftRequest{Summary: &summary})
	require.NoError(t, err)
	require.FileExists(t, path + ".bak")

	strategyintent.SetStoreForTest(strategyintent.NewFileStoreForTest(path))
	got, err := strategyintent.GetIntent(created.Intent.ID)
	require.NoError(t, err)
	require.Equal(t, "persist", got.Summary)
}

func TestListAndGet(t *testing.T) {
	now := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	in := strategyintent.Intent{
		ID:            "si:demo",
		SchemaVersion: strategyintent.SchemaVersion,
		CandidateID:   "rc:x",
		StrategySchemaRef: strategyintent.SchemaRef{
			Unbound:    false,
			StrategyID: "sdef:trend_breakout",
			Revision:   "v1",
		},
		SchemaRevision: "v1",
		IntentType:     strategyintent.IntentTypeManual,
		Action:         strategyintent.Action{Verb: "watch"},
		Status:         strategyintent.StatusDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	strategyintent.SetStoreForTest(strategyintent.NewMemoryStoreForTest(map[string]strategyintent.Intent{
		in.ID: in,
	}))
	t.Cleanup(strategyintent.ResetStoreForTest)

	list, err := strategyintent.ListIntents()
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "v1", list.Items[0].SchemaRevision)

	got, err := strategyintent.GetIntent("si:demo")
	require.NoError(t, err)
	require.Nil(t, got.PromotedPoolID)
	require.Nil(t, got.TradePlanID)
	require.NoError(t, strategyintent.ValidateIntent(got))

	_, err = strategyintent.GetIntent("missing")
	require.Error(t, err)
}

func TestSeedFile_LoadsAndValidates(t *testing.T) {
	root, err := os.Getwd()
	require.NoError(t, err)
	seed := filepath.Clean(filepath.Join(root, "..", "..", "data", "strategy_intents.json"))
	raw, err := os.ReadFile(seed)
	if err != nil {
		t.Skip("seed not found:", seed)
	}
	f, err := strategyintent.LoadStoreFileJSON(raw)
	require.NoError(t, err)
	require.NotEmpty(t, f.Intents)
	for _, in := range f.Intents {
		require.NoError(t, strategyintent.ValidateIntent(in), in.ID)
	}
}
