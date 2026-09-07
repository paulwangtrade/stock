package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func postWatchlistDraft(t *testing.T, mux *http.ServeMux, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tradeplans/watchlist-draft", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

func TestTradePlansWatchlistDraftAPI_CreatesDraft(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	require.NoError(t, data.EnsureTradePlanTables())
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	batch := "2026-09-06|close|default"
	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID: acc.ID, ScanBatchKey: batch, StockCode: "sz300620", Secucode: "300620.SZ",
		Action: opportunity.ActionWatch,
	})
	require.NoError(t, err)

	mux := registerTradePlansTestMux(t)
	rec := postWatchlistDraft(t, mux, map[string]any{
		"account_id":     acc.ID,
		"trade_date":     "2026-09-07",
		"stock_code":     "sz300620",
		"stock_name":     "测试",
		"opportunity_id": row.OpportunityID,
		"scan_batch_key": batch,
		"actor":          "ui:watched-opportunities",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp api.WatchlistDraftResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.NotZero(t, resp.PlanID)
	require.Equal(t, models.TradePlanStatusDraft, resp.Status)
	require.Equal(t, "buy", resp.Side)
	require.Equal(t, models.TradePlanSourceWatchlist, resp.SourceSession)

	got, err := data.NewTradePlanRepo().GetByID(resp.PlanID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.False(t, got.EnableExecute)
	require.Nil(t, got.ApprovedAt)
	require.Nil(t, got.FreezeAt)
}

func TestTradePlansWatchlistDraftAPI_PlanExistsConflict(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	require.NoError(t, data.EnsureTradePlanTables())
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	existing := &models.TradePlan{
		TradeDate: "2026-09-06", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusReady, Side: "buy",
	}
	fr := time.Now()
	existing.FreezeAt = &fr
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(existing, []models.TradePlanItem{{
		TradeDate: "2026-09-06", StockCode: "sz300620", Side: "buy", Status: models.TradePlanItemPending,
	}}))

	batch := "2026-09-06|close|default"
	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID: acc.ID, ScanBatchKey: batch, StockCode: "sz300620", Secucode: "300620.SZ",
		Action: opportunity.ActionWatch,
	})
	require.NoError(t, err)

	mux := registerTradePlansTestMux(t)
	rec := postWatchlistDraft(t, mux, map[string]any{
		"account_id":     acc.ID,
		"trade_date":     "2026-09-07",
		"stock_code":     "sz300620",
		"opportunity_id": row.OpportunityID,
		"scan_batch_key": batch,
		"actor":          "ui:watched-opportunities",
	})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	var resp api.WatchlistDraftErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.OK)
	require.Equal(t, api.WatchlistDraftCodePlanExists, resp.Code)
	require.Equal(t, existing.ID, resp.PlanID)
}
