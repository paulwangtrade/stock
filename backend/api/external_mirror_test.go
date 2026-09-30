package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/externalmirror"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestExternalMirrorAPI_CRUDAndIsolation(t *testing.T) {
	setupPaperAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	externalmirror.SetNameResolverForTest(func(string) string { return "" })
	t.Cleanup(func() { externalmirror.SetNameResolverForTest(nil) })

	mux := http.NewServeMux()
	api.RegisterExternalMirrorRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/external-mirror/holdings", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	bad := postMirror(t, mux, map[string]any{"stockCode": "600519", "quantity": 0, "costPrice": 10})
	require.Equal(t, false, bad["ok"])
	require.Equal(t, "invalid_quantity", bad["reason"])

	bad = postMirror(t, mux, map[string]any{
		"stockCode": "600519", "quantity": 100, "costPrice": 12, "source": "paper_sim",
	})
	require.Equal(t, "invalid_source", bad["reason"])

	var simN int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Count(&simN).Error)
	require.Equal(t, int64(0), simN)

	created := postMirror(t, mux, map[string]any{
		"stockCode": "600519",
		"quantity":  100,
		"costPrice": 1400.25,
		"note":      "观察",
		"stockName": "茅台",
	})
	require.Equal(t, true, created["ok"])
	require.Equal(t, externalmirror.Disclaimer, created["disclaimer"])
	require.Equal(t, false, created["feedsTradePlan"])
	require.Equal(t, false, created["tradable"])
	require.Equal(t, externalmirror.Source, created["source"])
	holding := created["holding"].(map[string]any)
	require.Equal(t, "sh600519", holding["stockCode"])
	require.Equal(t, "茅台", holding["stockName"])
	require.Equal(t, false, holding["feedsTradePlan"])
	id := uint(holding["id"].(float64))

	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Count(&simN).Error)
	require.Equal(t, int64(0), simN)

	listed := getMirror(t, mux)
	rows := listed["holdings"].([]any)
	require.Len(t, rows, 1)

	updBody, _ := json.Marshal(map[string]any{"quantity": 80, "costPrice": 1411, "entryDate": "2026-09-02"})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/external-mirror/holdings/"+strconv.FormatUint(uint64(id), 10), bytes.NewReader(updBody))
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var updated map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	require.Equal(t, true, updated["ok"])
	got := updated["holding"].(map[string]any)
	require.Equal(t, float64(80), got["quantity"])
	require.Equal(t, "2026-09-02", got["entryDate"])
	require.Equal(t, "茅台", got["stockName"])

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/external-mirror/holdings/"+strconv.FormatUint(uint64(id), 10), nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	listed = getMirror(t, mux)
	require.Empty(t, listed["holdings"].([]any))
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Count(&simN).Error)
	require.Equal(t, int64(0), simN)
}

func postMirror(t *testing.T, mux *http.ServeMux, payload map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/external-mirror/holdings", bytes.NewReader(raw)))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	if payload["quantity"] == 0 || payload["source"] == "paper_sim" {
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	return body
}

func getMirror(t *testing.T, mux *http.ServeMux) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/external-mirror/holdings", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, true, body["ok"])
	require.Equal(t, externalmirror.Disclaimer, body["disclaimer"])
	return body
}
