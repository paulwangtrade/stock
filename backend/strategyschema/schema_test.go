package strategyschema_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-stock/backend/strategyschema"

	"github.com/stretchr/testify/require"
)

func emptyStore(t *testing.T) {
	t.Helper()
	strategyschema.SetStoreForTest(strategyschema.NewMemoryStoreForTest(nil, nil))
	t.Cleanup(strategyschema.ResetStoreForTest)
}

func sampleDraftReq(name string) strategyschema.CreateDraftRequest {
	return strategyschema.CreateDraftRequest{
		Name:        name,
		Description: "B3-B test",
		Universe:    strategyschema.UniverseSpec{Source: "scan", MaxSize: 30},
		Signals:     strategyschema.SignalsSpec{Kind: "deterministic"},
		Filters:     strategyschema.FiltersSpec{},
		Ranking:     strategyschema.RankingSpec{TopN: 10},
		RiskProfileRef: strategyschema.RiskProfileRef{Mode: "inherit"},
		Knobs: map[string]any{
			"lookback_days": 10,
		},
	}
}

func TestLifecycle_CreateUpdateSubmitActivate(t *testing.T) {
	emptyStore(t)

	created, err := strategyschema.CreateDraft(sampleDraftReq("Lifecycle Demo"))
	require.NoError(t, err)
	require.Equal(t, strategyschema.RevisionStatusDraft, created.Revision.Status)
	require.Equal(t, strategyschema.DefinitionStatusDraftOnly, created.Definition.Status)
	require.NotEmpty(t, created.Revision.Parameters.ParamsHash)
	require.NotEmpty(t, created.Revision.RevisionHash)
	strategyID := created.Definition.StrategyID
	version := created.Revision.Revision

	note := "updated note"
	updated, err := strategyschema.UpdateDraft(strategyID, version, strategyschema.UpdateDraftRequest{
		RevisionNote: &note,
		Knobs:        map[string]any{"lookback_days": 20},
	})
	require.NoError(t, err)
	require.Equal(t, "updated note", updated.Revision.RevisionNote)
	require.Equal(t, 20, updated.Revision.Parameters.Knobs["lookback_days"])
	require.NotEqual(t, created.Revision.Parameters.ParamsHash, updated.Revision.Parameters.ParamsHash)
	require.NotEqual(t, created.Revision.RevisionHash, updated.Revision.RevisionHash)

	submitted, err := strategyschema.SubmitReview(strategyID, version)
	require.NoError(t, err)
	require.Equal(t, strategyschema.RevisionStatusReviewing, submitted.Revision.Status)

	// reviewing content frozen
	_, err = strategyschema.UpdateDraft(strategyID, version, strategyschema.UpdateDraftRequest{Knobs: map[string]any{"x": 1}})
	require.Error(t, err)
	var we strategyschema.WriteError
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyschema.CodeRevisionImmutable, we.Code)

	activated, err := strategyschema.Activate(strategyID, version)
	require.NoError(t, err)
	require.Equal(t, strategyschema.RevisionStatusActive, activated.Revision.Status)
	require.Equal(t, strategyschema.DefinitionStatusActive, activated.Definition.Status)
	require.Equal(t, version, activated.Definition.CurrentRevision)
}

func TestLifecycle_ActiveUpdateRejected(t *testing.T) {
	emptyStore(t)
	created, err := strategyschema.CreateDraft(sampleDraftReq("Freeze Demo"))
	require.NoError(t, err)
	id, ver := created.Definition.StrategyID, created.Revision.Revision
	_, err = strategyschema.SubmitReview(id, ver)
	require.NoError(t, err)
	_, err = strategyschema.Activate(id, ver)
	require.NoError(t, err)

	_, err = strategyschema.UpdateDraft(id, ver, strategyschema.UpdateDraftRequest{
		Knobs: map[string]any{"lookback_days": 99},
	})
	require.Error(t, err)
	var we strategyschema.WriteError
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyschema.CodeRevisionImmutable, we.Code)
}

func TestLifecycle_SkipLevelActivateRejected(t *testing.T) {
	emptyStore(t)
	created, err := strategyschema.CreateDraft(sampleDraftReq("Skip Activate"))
	require.NoError(t, err)
	_, err = strategyschema.Activate(created.Definition.StrategyID, created.Revision.Revision)
	require.Error(t, err)
	var we strategyschema.WriteError
	require.ErrorAs(t, err, &we)
	require.Equal(t, strategyschema.CodeInvalidTransition, we.Code)
}

func TestLifecycle_HashStability(t *testing.T) {
	emptyStore(t)
	req := sampleDraftReq("Hash Demo")
	a, err := strategyschema.CreateDraft(req)
	require.NoError(t, err)
	h1 := a.Revision.Parameters.ParamsHash
	h2 := a.Revision.RevisionHash

	// same knobs → same hashes
	require.Equal(t, strategyschema.ComputeParamsHash(req.Knobs), h1)
	require.Equal(t, strategyschema.ComputeRevisionHash(a.Revision), h2)

	// recompute after get
	got, err := strategyschema.GetRevision(a.Definition.StrategyID, a.Revision.Revision)
	require.NoError(t, err)
	require.Equal(t, h1, strategyschema.ComputeParamsHash(got.Parameters.Knobs))
	require.Equal(t, h2, strategyschema.ComputeRevisionHash(got))
	require.NoError(t, strategyschema.VerifyContentHashes(got))
}

func TestLifecycle_SidecarReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strategy_schemas.json")
	store := strategyschema.NewFileStoreForTest(path)
	strategyschema.SetStoreForTest(store)
	t.Cleanup(strategyschema.ResetStoreForTest)

	created, err := strategyschema.CreateDraft(sampleDraftReq("Sidecar Demo"))
	require.NoError(t, err)
	require.FileExists(t, path)

	// second write should create rolling .bak
	note := "persist"
	_, err = strategyschema.UpdateDraft(created.Definition.StrategyID, created.Revision.Revision, strategyschema.UpdateDraftRequest{
		RevisionNote: &note,
	})
	require.NoError(t, err)
	require.FileExists(t, path+".bak")

	// new store instance reloads from disk
	strategyschema.SetStoreForTest(strategyschema.NewFileStoreForTest(path))
	got, err := strategyschema.GetRevision(created.Definition.StrategyID, created.Revision.Revision)
	require.NoError(t, err)
	require.Equal(t, "persist", got.RevisionNote)
	require.Equal(t, strategyschema.RevisionStatusDraft, got.Status)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	f, err := strategyschema.LoadStoreFileJSON(raw)
	require.NoError(t, err)
	require.Len(t, f.Definitions, 1)
	require.Len(t, f.Revisions, 1)
}

func TestLifecycle_ActivateRetiresPrevious(t *testing.T) {
	emptyStore(t)
	c1, err := strategyschema.CreateDraft(sampleDraftReq("Retire Prev"))
	require.NoError(t, err)
	id := c1.Definition.StrategyID
	v1 := c1.Revision.Revision
	_, err = strategyschema.SubmitReview(id, v1)
	require.NoError(t, err)
	_, err = strategyschema.Activate(id, v1)
	require.NoError(t, err)

	c2, err := strategyschema.CreateDraft(strategyschema.CreateDraftRequest{
		StrategyID:     id,
		Name:           "Retire Prev",
		ParentRevision: v1,
		Universe:       strategyschema.UniverseSpec{Source: "scan"},
		Signals:        strategyschema.SignalsSpec{Kind: "deterministic"},
		RiskProfileRef: strategyschema.RiskProfileRef{Mode: "inherit"},
		Knobs:          map[string]any{"lookback_days": 5},
	})
	require.NoError(t, err)
	v2 := c2.Revision.Revision
	require.NotEqual(t, v1, v2)
	_, err = strategyschema.SubmitReview(id, v2)
	require.NoError(t, err)
	act, err := strategyschema.Activate(id, v2)
	require.NoError(t, err)
	require.Equal(t, v2, act.Definition.CurrentRevision)

	old, err := strategyschema.GetRevision(id, v1)
	require.NoError(t, err)
	require.Equal(t, strategyschema.RevisionStatusRetired, old.Status)

	// retired immutable
	_, err = strategyschema.UpdateDraft(id, v1, strategyschema.UpdateDraftRequest{Knobs: map[string]any{"x": 1}})
	require.Error(t, err)
}

func TestCanTransitionRevision(t *testing.T) {
	require.True(t, strategyschema.CanTransitionRevision(strategyschema.RevisionStatusDraft, strategyschema.RevisionStatusReviewing))
	require.True(t, strategyschema.CanTransitionRevision(strategyschema.RevisionStatusReviewing, strategyschema.RevisionStatusActive))
	require.True(t, strategyschema.CanTransitionRevision(strategyschema.RevisionStatusActive, strategyschema.RevisionStatusRetired))
	require.False(t, strategyschema.CanTransitionRevision(strategyschema.RevisionStatusRetired, strategyschema.RevisionStatusActive))
	require.False(t, strategyschema.CanTransitionRevision(strategyschema.RevisionStatusActive, strategyschema.RevisionStatusDraft))
}

func TestMemoryStore_ListAndGet(t *testing.T) {
	now := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	defs := map[string]strategyschema.Definition{
		"sdef:demo": {
			StrategyID:      "sdef:demo",
			SchemaVersion:   strategyschema.SchemaVersion,
			Name:            "Demo",
			Status:          strategyschema.DefinitionStatusActive,
			CurrentRevision: "v1",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	revs := map[string]strategyschema.Revision{
		"srev:demo:v1": {
			RevisionID:    "srev:demo:v1",
			StrategyID:    "sdef:demo",
			Revision:      "v1",
			SchemaVersion: strategyschema.SchemaVersion,
			Status:        strategyschema.RevisionStatusActive,
			Parameters:    strategyschema.ParametersSpec{ParamsHash: "abc"},
			Source:        "manual",
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
	strategyschema.SetStoreForTest(strategyschema.NewMemoryStoreForTest(defs, revs))
	t.Cleanup(strategyschema.ResetStoreForTest)

	list, err := strategyschema.ListSchemas()
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "sdef:demo", list.Items[0].StrategyID)

	detail, err := strategyschema.GetSchema("sdef:demo")
	require.NoError(t, err)
	require.Equal(t, "Demo", detail.Definition.Name)
	require.Len(t, detail.Revisions, 1)
	require.NotNil(t, detail.Current)
	require.Equal(t, "v1", detail.Current.Revision)

	rev, err := strategyschema.GetRevision("sdef:demo", "v1")
	require.NoError(t, err)
	require.Equal(t, "abc", rev.Parameters.ParamsHash)

	_, err = strategyschema.GetSchema("missing")
	require.Error(t, err)
}

func TestSeedFile_LoadsIfPresent(t *testing.T) {
	root, err := os.Getwd()
	require.NoError(t, err)
	seed := filepath.Clean(filepath.Join(root, "..", "..", "data", "strategy_schemas.json"))
	raw, err := os.ReadFile(seed)
	if err != nil {
		t.Skip("seed file not found from package cwd:", seed)
	}
	f, err := strategyschema.LoadStoreFileJSON(raw)
	require.NoError(t, err)
	require.NotEmpty(t, f.Definitions)
	require.NotEmpty(t, f.Revisions)
}
