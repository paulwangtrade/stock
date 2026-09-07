package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestWatchlistAPI_GET_WATCHING_and_IGNORE(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	acc := seedExitEvalAccount(t)
	batch := "2026-09-05|close|default"
	oid := opportunity.BuildOpportunityID(batch, "600363.SH", "2026-09-05", "强")

	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     acc.ID,
		ScanBatchKey:  batch,
		OpportunityID: oid,
		StockCode:     "sh600363",
		Action:        opportunity.ActionWatch,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	api.RegisterWatchlistRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/watchlist?account_id="+jsonNumber(acc.ID), nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp["ok"].(bool))
	wl := resp["watchlist"].(map[string]any)
	require.EqualValues(t, 1, wl["watching_count"])
	items := wl["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Equal(t, "sh600363", item["stock_code"])
	require.Equal(t, opportunity.ActionWatch, item["latest_action"])

	_, err = opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     acc.ID,
		ScanBatchKey:  batch,
		OpportunityID: oid,
		StockCode:     "sh600363",
		Action:        opportunity.ActionIgnore,
	})
	require.NoError(t, err)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/watchlist?account_id="+jsonNumber(acc.ID), nil)
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var resp2 map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	wl2 := resp2["watchlist"].(map[string]any)
	require.EqualValues(t, 0, wl2["watching_count"])
	items2, _ := wl2["items"].([]any)
	require.Empty(t, items2)
}
