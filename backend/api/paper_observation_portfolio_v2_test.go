package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPortfolioObservationV2_GETOnly(t *testing.T) {
	h := NewPaperObservationHandler()
	post := httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/portfolio-v2", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, post)
	require.Equal(t, http.StatusMethodNotAllowed, rw.Code)

	get := httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/portfolio-v2", nil)
	rw2 := httptest.NewRecorder()
	h.ServeHTTP(rw2, get)
	require.Equal(t, http.StatusOK, rw2.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rw2.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])
	flags, _ := body["flags"].(map[string]any)
	require.Equal(t, true, flags["not_provider_switch"])
	require.Equal(t, true, flags["not_execution"])
	require.Equal(t, true, flags["not_a_trade_plan"])
	obs, ok := body["observation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, obs["not_provider_switch"])
	require.Equal(t, true, obs["read_only"])
}
