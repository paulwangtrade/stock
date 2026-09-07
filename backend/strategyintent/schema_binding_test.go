package strategyintent

import (
	"testing"

	"go-stock/backend/strategyschema"

	"github.com/stretchr/testify/require"
)

type fakeSchemaLookup map[string]string

func (f fakeSchemaLookup) LookupRevision(strategyID, revision string) (status string, found bool, err error) {
	st, ok := f[strategyID+"/"+revision]
	return st, ok, nil
}

func TestValidateSchemaBinding_ExistsAndRetired(t *testing.T) {
	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:ok/v1":      strategyschema.RevisionStatusActive,
		"sdef:old/v1":     strategyschema.RevisionStatusRetired,
		"sdef:trash/v1":   strategyschema.RevisionStatusDiscarded,
	})
	t.Cleanup(func() { SetSchemaRevisionLookupForTest(nil) })

	require.NoError(t, validateSchemaBinding(SchemaRef{
		Unbound: false, StrategyID: "sdef:ok", Revision: "v1",
	}, "v1", schemaBindingContext{}))

	err := validateSchemaBinding(SchemaRef{
		Unbound: false, StrategyID: "sdef:missing", Revision: "v1",
	}, "v1", schemaBindingContext{})
	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, CodeSchemaRevisionNotFound, ve.Code)

	err = validateSchemaBinding(SchemaRef{
		Unbound: false, StrategyID: "sdef:old", Revision: "v1",
	}, "v1", schemaBindingContext{})
	require.Error(t, err)
	require.ErrorAs(t, err, &ve)
	require.Equal(t, CodeSchemaRevisionNotBindable, ve.Code)

	require.NoError(t, validateSchemaBinding(SchemaRef{Unbound: true}, "", schemaBindingContext{}))
}

func TestValidateSchemaBinding_GrandfatherUnchangedPin(t *testing.T) {
	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:old/v1": strategyschema.RevisionStatusRetired,
	})
	t.Cleanup(func() { SetSchemaRevisionLookupForTest(nil) })

	prior := SchemaRef{Unbound: false, StrategyID: "sdef:old", Revision: "v1"}
	err := validateSchemaBinding(prior, "v1", schemaBindingContext{
		priorRef:      &prior,
		priorRevision: "v1",
	})
	require.NoError(t, err)
}

func TestWrite_SchemaBindingValidation(t *testing.T) {
	SetStoreForTest(NewMemoryStoreForTest(nil))
	t.Cleanup(ResetStoreForTest)
	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:trend_breakout/v1": strategyschema.RevisionStatusActive,
		"sdef:retired/v1":        strategyschema.RevisionStatusRetired,
	})
	t.Cleanup(func() { SetSchemaRevisionLookupForTest(nil) })

	_, err := CreateDraft(CreateDraftRequest{
		CandidateID: "rc:signal:2026-08-17:bind1",
		StrategySchemaRef: SchemaRef{
			Unbound: false, StrategyID: "sdef:missing", Revision: "v1",
		},
		SchemaRevision: "v1",
		Action:         Action{Verb: "watch"},
	})
	require.Error(t, err)
	var ve ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, CodeSchemaRevisionNotFound, ve.Code)

	_, err = CreateDraft(CreateDraftRequest{
		CandidateID: "rc:signal:2026-08-17:bind2",
		StrategySchemaRef: SchemaRef{
			Unbound: false, StrategyID: "sdef:retired", Revision: "v1",
		},
		SchemaRevision: "v1",
		Action:         Action{Verb: "watch"},
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &ve)
	require.Equal(t, CodeSchemaRevisionNotBindable, ve.Code)

	created, err := CreateDraft(CreateDraftRequest{
		CandidateID: "rc:signal:2026-08-17:bind3",
		StrategySchemaRef: SchemaRef{
			Unbound: false, StrategyID: "sdef:trend_breakout", Revision: "v1",
		},
		SchemaRevision: "v1",
		Action:         Action{Verb: "watch"},
	})
	require.NoError(t, err)

	// Grandfather: pin already v1; lookup now says retired — summary-only update OK.
	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:trend_breakout/v1": strategyschema.RevisionStatusRetired,
	})
	summary := "still ok"
	_, err = UpdateDraft(created.Intent.ID, UpdateDraftRequest{Summary: &summary})
	require.NoError(t, err)

	// New binding to retired revision rejected.
	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:trend_breakout/v1": strategyschema.RevisionStatusRetired,
		"sdef:retired/v1":        strategyschema.RevisionStatusRetired,
	})
	retired := SchemaRef{Unbound: false, StrategyID: "sdef:retired", Revision: "v1"}
	rev := "v1"
	_, err = UpdateDraft(created.Intent.ID, UpdateDraftRequest{
		StrategySchemaRef: &retired,
		SchemaRevision:    &rev,
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &ve)
	require.Equal(t, CodeSchemaRevisionNotBindable, ve.Code)
}

func TestWrite_SubmitApprove_GrandfatherRetiredPin(t *testing.T) {
	SetStoreForTest(NewMemoryStoreForTest(nil))
	t.Cleanup(ResetStoreForTest)

	prior := SchemaRef{Unbound: false, StrategyID: "sdef:legacy", Revision: "v1"}
	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:legacy/v1": strategyschema.RevisionStatusActive,
	})
	t.Cleanup(func() { SetSchemaRevisionLookupForTest(nil) })

	created, err := CreateDraft(CreateDraftRequest{
		CandidateID:       "rc:signal:2026-08-17:legacy",
		StrategySchemaRef: prior,
		SchemaRevision:    "v1",
		Action:            Action{Verb: "watch"},
	})
	require.NoError(t, err)

	SetSchemaRevisionLookupForTest(fakeSchemaLookup{
		"sdef:legacy/v1": strategyschema.RevisionStatusRetired,
	})

	submitted, err := SubmitReview(created.Intent.ID)
	require.NoError(t, err)
	require.Equal(t, StatusReviewing, submitted.Intent.Status)

	approved, err := Approve(created.Intent.ID)
	require.NoError(t, err)
	require.Equal(t, StatusApproved, approved.Intent.Status)
}
