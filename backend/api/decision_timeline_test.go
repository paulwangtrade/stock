package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecisionTimelineHandler_GetOnlyAndInvalidCode(t *testing.T) {
	h := NewDecisionTimelineHandler()

	post := httptest.NewRequest(http.MethodPost, "/api/research/decision-timeline?stock_code=sz000001", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, post)
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	get := httptest.NewRequest(http.MethodGet, "/api/research/decision-timeline", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, false, body["ok"])
}
