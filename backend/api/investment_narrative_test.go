package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/investmentnarrative"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedNarrativeSnapshot(t *testing.T) {
	t.Helper()
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}))
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE: "600363.SH", Tag: "强", SignalTime: "2026-08-18", SignalPrice: 28.5,
				SignalPriceStatus: models.SignalPriceStatusFrozen,
			},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		TradeDate: "2026-08-18", Session: "close", Status: "done", ResultJSON: string(raw),
	}).Error)
}

func TestInvestmentNarrativeAPI_GET(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	seedNarrativeSnapshot(t)

	mux := http.NewServeMux()
	api.RegisterInvestmentNarrativeRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/investment/narrative/sh600363", nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	narr := body["narrative"].(map[string]any)
	disc := narr["discovery"].(map[string]any)
	require.True(t, disc["exists"].(bool))
	require.Equal(t, investmentnarrative.SchemaVersion, narr["schema_version"])
}

func TestInvestmentNarrativeAPI_MissingStock(t *testing.T) {
	setupAPITestDB(t)

	mux := http.NewServeMux()
	api.RegisterInvestmentNarrativeRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/investment/narrative/sh999999", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	narr := body["narrative"].(map[string]any)
	disc := narr["discovery"].(map[string]any)
	require.False(t, disc["exists"].(bool))
}
