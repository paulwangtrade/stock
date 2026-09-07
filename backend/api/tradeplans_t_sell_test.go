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
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedAPITSellHolding(t *testing.T, code string, total, avail, locked int64) {
	t.Helper()
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000, Equity: 1_000_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	pos := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: code, StockName: "测试",
		TotalVolume: total, AvailableVolume: avail, LockedVolume: locked,
		AvgCost: 10, MarkPrice: 10, UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(&pos).Error)
}

func postTSellDraft(t *testing.T, mux *http.ServeMux, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tradeplans/t-sell/draft", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

func TestTradePlansTSellDraftAPI_AvailableSufficient(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	seedAPITSellHolding(t, "sz000001", 1000, 1000, 0)
	mux := registerTradePlansTestMux(t)

	rec := postTSellDraft(t, mux, map[string]any{
		"trade_date": "2026-08-18",
		"stock_code": "sz000001",
		"quantity":   100,
		"actor":      "desktop-ui",
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp api.TSellDraftResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.NotZero(t, resp.PlanID)
	require.Equal(t, models.TradePlanStatusDraft, resp.Status)
	require.Equal(t, "sell", resp.Side)
	require.Equal(t, "2026-08-18", resp.TradeDate)
	require.Equal(t, 1, resp.ItemCount)

	got, err := data.NewTradePlanRepo().GetByID(resp.PlanID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.Nil(t, got.ApprovedAt)
	require.Nil(t, got.FreezeAt)
	require.Equal(t, int64(100), got.Items[0].TargetVolume)
}

func TestTradePlansTSellDraftAPI_ExitReviewSourceSession(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	seedAPITSellHolding(t, "sz000001", 1000, 1000, 0)
	mux := registerTradePlansTestMux(t)

	rec := postTSellDraft(t, mux, map[string]any{
		"trade_date":     "2026-08-18",
		"stock_code":     "sz000001",
		"quantity":       100,
		"actor":          "ui:exit-review-sell",
		"reason":         "exit_review:REVIEW_REQUIRED;LOSS_REVIEW",
		"source_session": models.TradePlanSourceExitReview,
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp api.TSellDraftResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.OK)
	require.Equal(t, models.TradePlanSourceExitReview, resp.SourceSession)

	got, err := data.NewTradePlanRepo().GetByID(resp.PlanID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanSourceExitReview, got.SourceSession)
}

func TestTradePlansTSellDraftAPI_InsufficientAvailable(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	seedAPITSellHolding(t, "sz000001", 1000, 200, 800)
	mux := registerTradePlansTestMux(t)

	rec := postTSellDraft(t, mux, map[string]any{
		"trade_date": "2026-08-18",
		"stock_code": "sz000001",
		"quantity":   500,
	})
	require.Equal(t, http.StatusConflict, rec.Code)

	var resp api.TSellDraftErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.OK)
	require.Equal(t, api.TSellDraftCodeInsufficientAvailable, resp.Code)
}

func TestTradePlansTSellDraftAPI_CannotSell(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	seedAPITSellHolding(t, "sz000001", 1000, 0, 1000)
	mux := registerTradePlansTestMux(t)

	rec := postTSellDraft(t, mux, map[string]any{
		"trade_date": "2026-08-18",
		"stock_code": "sz000001",
		"quantity":   100,
	})
	require.Equal(t, http.StatusConflict, rec.Code)

	var resp api.TSellDraftErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.OK)
	require.Equal(t, api.TSellDraftCodeCannotSell, resp.Code)
}

func TestTradePlansTSellDraftAPI_InvalidStock(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	seedAPITSellHolding(t, "sz000001", 1000, 1000, 0)
	mux := registerTradePlansTestMux(t)

	rec := postTSellDraft(t, mux, map[string]any{
		"trade_date": "2026-08-18",
		"stock_code": "sh999999",
		"quantity":   100,
	})
	require.Equal(t, http.StatusNotFound, rec.Code)

	var resp api.TSellDraftErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.OK)
	require.Equal(t, api.TSellDraftCodeInvalidStock, resp.Code)
}
