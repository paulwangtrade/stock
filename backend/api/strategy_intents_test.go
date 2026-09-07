package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/strategyintent"

	"github.com/stretchr/testify/require"
)

func seedIntentStore(t *testing.T) {
	t.Helper()
	now := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	in := strategyintent.Intent{
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
	strategyintent.SetStoreForTest(strategyintent.NewMemoryStoreForTest(map[string]strategyintent.Intent{
		in.ID: in,
	}))
	t.Cleanup(strategyintent.ResetStoreForTest)
}

func emptyIntentStore(t *testing.T) {
	t.Helper()
	strategyintent.SetStoreForTest(strategyintent.NewMemoryStoreForTest(nil))
	strategyintent.SetSchemaRevisionLookupForTest(strategyintent.DefaultTestSchemaLookup())
	t.Cleanup(func() {
		strategyintent.ResetStoreForTest()
		strategyintent.SetSchemaRevisionLookupForTest(nil)
	})
}

func TestStrategyIntentsAPI_ListDetail(t *testing.T) {
	seedIntentStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategyIntentsRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategy/intents", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var list strategyintent.ListResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)
	require.Equal(t, "v1", list.Items[0].SchemaRevision)

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/strategy/intents/si:demo", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var detail strategyintent.Intent
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &detail))
	require.Equal(t, "si:demo", detail.ID)
	require.Equal(t, "v1", detail.SchemaRevision)
	require.Nil(t, detail.PromotedPoolID)
}

func TestStrategyIntentsAPI_WriteLifecycle(t *testing.T) {
	emptyIntentStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategyIntentsRoutes(mux)

	body := map[string]any{
		"candidate_id": "rc:signal:2026-08-17:api",
		"strategy_schema_ref": map[string]any{
			"unbound":     false,
			"strategy_id": "sdef:trend_breakout",
			"revision":    "v1",
		},
		"schema_revision": "v1",
		"intent_type":     "manual",
		"action":          map[string]any{"verb": "watch"},
		"conditions":      map[string]any{"session": "open"},
	}
	raw, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/strategy/intents", bytes.NewReader(raw)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var created strategyintent.WriteResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id := created.Intent.ID

	patch, _ := json.Marshal(map[string]any{"summary": "n1"})
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodPatch, "/api/strategy/intents/"+id, bytes.NewReader(patch)))
	require.Equal(t, http.StatusOK, rec2.Code)

	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, httptest.NewRequest(http.MethodPost, "/api/strategy/intents/"+id+"/submit", nil))
	require.Equal(t, http.StatusOK, rec3.Code)

	rec4 := httptest.NewRecorder()
	mux.ServeHTTP(rec4, httptest.NewRequest(http.MethodPost, "/api/strategy/intents/"+id+"/approve", nil))
	require.Equal(t, http.StatusOK, rec4.Code)

	rec5 := httptest.NewRecorder()
	mux.ServeHTTP(rec5, httptest.NewRequest(http.MethodPatch, "/api/strategy/intents/"+id, bytes.NewReader(patch)))
	require.Equal(t, http.StatusBadRequest, rec5.Code)
	var errBody map[string]any
	require.NoError(t, json.Unmarshal(rec5.Body.Bytes(), &errBody))
	require.Equal(t, strategyintent.CodeIntentImmutable, errBody["error"])
}

func TestStrategyIntentsAPI_SkipApproveRejected(t *testing.T) {
	emptyIntentStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategyIntentsRoutes(mux)

	raw, _ := json.Marshal(map[string]any{
		"candidate_id": "rc:signal:2026-08-17:skip",
		"strategy_schema_ref": map[string]any{
			"unbound": true,
			"note":    "unbound",
		},
		"action": map[string]any{"verb": "watch"},
	})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/strategy/intents", bytes.NewReader(raw)))
	require.Equal(t, http.StatusCreated, rec.Code)
	var created strategyintent.WriteResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/strategy/intents/"+created.Intent.ID+"/approve", nil))
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	var errBody map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &errBody))
	require.Equal(t, strategyintent.CodeInvalidTransition, errBody["error"])
}

func TestStrategyIntentsAPI_NotFound(t *testing.T) {
	seedIntentStore(t)
	mux := http.NewServeMux()
	api.RegisterStrategyIntentsRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/strategy/intents/si:missing", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}
