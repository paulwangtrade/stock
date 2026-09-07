package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedOpportunitySnapshot(t *testing.T) uint {
	t.Helper()
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}))
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE:           "600363.SH",
				SECURITY_NAME_ABBR: "联创光电",
				Tag:                "强",
				SignalTime:         "2026-08-18",
				SignalPrice:        28.5,
				SignalPriceStatus:  models.SignalPriceStatusFrozen,
			},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{
		TradeDate: "2026-08-18", Session: "close", StrategyID: "default",
		Status: "done", HitTotal: 1, ResultJSON: string(raw),
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	return snap.ID
}

func TestOpportunitiesAPI_POST_WATCH_and_LIST(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	acc := seedExitEvalAccount(t)
	snapID := seedOpportunitySnapshot(t)
	batchKey := opportunity.BatchKeyFromSnapshot(snapID, "2026-08-18", "close", "default")

	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	postBody, err := json.Marshal(map[string]any{
		"account_id":     acc.ID,
		"scan_batch_key": batchKey,
		"secucode":       "600363.SH",
		"signal_time":    "2026-08-18",
		"signal_tag":     "强",
		"action":         opportunity.ActionWatch,
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/opportunities/action", bytes.NewReader(postBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var postResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &postResp))
	require.True(t, postResp["ok"].(bool))

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet,
		"/api/opportunities/list?trade_date=2026-08-18&session=close&strategy_id=default&include_user_action=true&account_id="+
			jsonNumber(acc.ID), nil)
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	var listResp map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &listResp))
	require.True(t, listResp["ok"].(bool))
	pool := listResp["pool"].(map[string]any)
	entries := pool["entries"].([]any)
	require.Len(t, entries, 1)
	entry := entries[0].(map[string]any)
	require.Equal(t, "sh600363", entry["stock_code"])
	latest := entry["latest_user_action"].(map[string]any)
	require.Equal(t, opportunity.ActionWatch, latest["action"])
}

func jsonNumber(n uint) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestOpportunitiesAPI_REJECT_CREATE_PLAN(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)

	postBody, _ := json.Marshal(map[string]any{
		"scan_batch_key": "snap:1",
		"stock_code":     "sh600363",
		"action":         "CREATE_PLAN",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/opportunities/action", bytes.NewReader(postBody))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
