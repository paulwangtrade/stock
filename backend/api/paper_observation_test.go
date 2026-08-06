package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"

	"github.com/stretchr/testify/require"
)

func TestPaperObservation_RegisterRoutes_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)

	paths := []string{
		"/api/papertrading/status",
		"/api/papertrading/dashboard/today",
		"/api/papertrading/dashboard/positions",
		"/api/papertrading/dashboard/runs",
		"/api/papertrading/reports/daily",
		"/api/papertrading/observation/metrics",
	}
	for _, p := range paths {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, p, nil))
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code, "POST must be rejected on %s", p)
	}

	// Without DB the handlers return 500 but must remain wired on the expected URLs.
	// With DB available they return 200; either proves path registration.
	for _, p := range paths {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		require.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rec.Code,
			"GET must hit handler on %s", p)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Contains(t, body, "ok")
		require.Contains(t, body, "code")
	}
}

func TestPaperObservation_AssetMiddleware_DoesNotCaptureRun(t *testing.T) {
	calledNext := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.PaperObservationAssetMiddleware(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/dashboard/positions", nil))
	require.False(t, calledNext, "observation path must be handled by observation middleware")
	require.NotEqual(t, http.StatusTeapot, rec.Code)

	calledNext = false
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/reports/daily", nil))
	require.False(t, calledNext, "daily reports path must be handled by observation middleware")
	require.NotEqual(t, http.StatusTeapot, rec.Code)

	calledNext = false
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/metrics", nil))
	require.False(t, calledNext, "observation metrics path must be handled by observation middleware")
	require.NotEqual(t, http.StatusTeapot, rec.Code)

	calledNext = false
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/run", nil))
	require.True(t, calledNext, "/run must fall through; observation middleware must not capture it")
	require.Equal(t, http.StatusTeapot, rec.Code)
}
