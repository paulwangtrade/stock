package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/strategyschema"

	"github.com/stretchr/testify/require"
)

func seedSchemaStore(t *testing.T) {
	t.Helper()
	now := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	defs := map[string]strategyschema.Definition{
		"sdef:trend_breakout": {
			StrategyID:      "sdef:trend_breakout",
			SchemaVersion:   strategyschema.SchemaVersion,
			Name:            "趋势突破模板",
			Status:          strategyschema.DefinitionStatusActive,
			CurrentRevision: "v1",
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	revs := map[string]strategyschema.Revision{
		"srev:trend_breakout:v1": {
			RevisionID:     "srev:trend_breakout:v1",
			StrategyID:     "sdef:trend_breakout",
			Revision:       "v1",
			SchemaVersion:  strategyschema.SchemaVersion,
			Status:         strategyschema.RevisionStatusActive,
			Universe:       strategyschema.UniverseSpec{Source: "scan", MaxSize: 50},
			Signals:        strategyschema.SignalsSpec{Kind: "deterministic"},
			Ranking:        strategyschema.RankingSpec{TopN: 20},
			RiskProfileRef: strategyschema.RiskProfileRef{Mode: "inherit"},
			Parameters:     strategyschema.ParametersSpec{ParamsHash: "seed_hash"},
			Source:         "manual",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	strategyschema.SetStoreForTest(strategyschema.NewMemoryStoreForTest(defs, revs))
	t.Cleanup(strategyschema.ResetStoreForTest)
}

func emptySchemaStore(t *testing.T) {
	t.Helper()
	strategyschema.SetStoreForTest(strategyschema.NewMemoryStoreForTest(nil, nil))
	t.Cleanup(strategyschema.ResetStoreForTest)
}

func TestStrategySchemasAPI_ListDetailRevisions(t *testing.T) {
	seedSchemaStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategySchemasRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategy/schemas", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var list strategyschema.ListResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)

	id := "sdef:trend_breakout"
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/strategy/schemas/"+id, nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var detail strategyschema.DetailResult
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &detail))
	require.Equal(t, id, detail.Definition.StrategyID)
	require.NotNil(t, detail.Current)

	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/api/strategy/schemas/"+id+"/revisions", nil))
	require.Equal(t, http.StatusOK, rec3.Code)
	var revList strategyschema.RevisionListResult
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &revList))
	require.Len(t, revList.Items, 1)

	rec4 := httptest.NewRecorder()
	mux.ServeHTTP(rec4, httptest.NewRequest(http.MethodGet, "/api/strategy/schemas/"+id+"/revisions/v1", nil))
	require.Equal(t, http.StatusOK, rec4.Code)
	var rev strategyschema.Revision
	require.NoError(t, json.Unmarshal(rec4.Body.Bytes(), &rev))
	require.Equal(t, "v1", rev.Revision)
	require.Equal(t, "seed_hash", rev.Parameters.ParamsHash)
}

func TestStrategySchemasAPI_WriteLifecycle(t *testing.T) {
	emptySchemaStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategySchemasRoutes(mux)

	body := map[string]any{
		"name":        "API Lifecycle",
		"description": "write mvp",
		"universe":    map[string]any{"source": "scan"},
		"signals":     map[string]any{"kind": "deterministic"},
		"ranking":     map[string]any{"top_n": 5},
		"risk_profile_ref": map[string]any{"mode": "inherit"},
		"knobs":       map[string]any{"lookback_days": 7},
	}
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/strategy/schemas", bytes.NewReader(raw)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var created strategyschema.WriteResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created.Definition.StrategyID
	ver := created.Revision.Revision

	patch, _ := json.Marshal(map[string]any{"revision_note": "n1", "knobs": map[string]any{"lookback_days": 8}})
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPatch, "/api/strategy/schemas/"+id+"/revisions/"+ver, bytes.NewReader(patch))
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, httptest.NewRequest(http.MethodPost, "/api/strategy/schemas/"+id+"/revisions/"+ver+"/submit", nil))
	require.Equal(t, http.StatusOK, rec3.Code)

	rec4 := httptest.NewRecorder()
	mux.ServeHTTP(rec4, httptest.NewRequest(http.MethodPost, "/api/strategy/schemas/"+id+"/revisions/"+ver+"/activate", nil))
	require.Equal(t, http.StatusOK, rec4.Code)
	var act strategyschema.WriteResult
	require.NoError(t, json.Unmarshal(rec4.Body.Bytes(), &act))
	require.Equal(t, strategyschema.RevisionStatusActive, act.Revision.Status)

	// active update reject
	rec5 := httptest.NewRecorder()
	req5 := httptest.NewRequest(http.MethodPatch, "/api/strategy/schemas/"+id+"/revisions/"+ver, bytes.NewReader(patch))
	mux.ServeHTTP(rec5, req5)
	require.Equal(t, http.StatusBadRequest, rec5.Code)
	var errBody map[string]any
	require.NoError(t, json.Unmarshal(rec5.Body.Bytes(), &errBody))
	require.Equal(t, strategyschema.CodeRevisionImmutable, errBody["error"])
}

func TestStrategySchemasAPI_SkipActivateRejected(t *testing.T) {
	emptySchemaStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategySchemasRoutes(mux)

	raw, _ := json.Marshal(map[string]any{
		"name":             "Skip",
		"universe":         map[string]any{"source": "scan"},
		"signals":          map[string]any{"kind": "deterministic"},
		"risk_profile_ref": map[string]any{"mode": "inherit"},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/strategy/schemas", bytes.NewReader(raw)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var created strategyschema.WriteResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost,
		"/api/strategy/schemas/"+created.Definition.StrategyID+"/revisions/"+created.Revision.Revision+"/activate", nil))
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	var errBody map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errBody))
	require.Equal(t, strategyschema.CodeInvalidTransition, errBody["error"])
}

func TestStrategySchemasAPI_NotFound(t *testing.T) {
	seedSchemaStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategySchemasRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategy/schemas/sdef:missing", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStrategySchemasAPI_DisallowedMethod(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterStrategySchemasRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/strategy/schemas", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
