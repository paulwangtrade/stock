package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/tradeplanorigin"

	"github.com/stretchr/testify/require"
)

func TestTradePlanOriginAPI_WithSignal(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}))

	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE: "000001.SZ", Tag: "强", SignalTime: "2026-08-28", SignalPrice: 10.5,
				SignalPriceStatus: models.SignalPriceStatusFrozen,
			},
		},
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{TradeDate: "2026-08-28", Session: "close", Status: "done", ResultJSON: string(raw)}
	require.NoError(t, db.Dao.Create(snap).Error)

	now := time.Now()
	pool := &models.CandidatePool{TradeDate: "2026-08-28", GeneratedAt: now, Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady}
	poolItems := []models.CandidatePoolItem{
		{
			StockCode: "sz000001", Rank: 1, Score: 0.9, StrategyName: "测试策略",
			SignalTag: "强", SignalSnapshotID: snap.ID, Reason: models.CandidatePoolSourceStrategyRun,
		},
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, poolItems))

	plan := &models.TradePlan{
		TradeDate: "2026-08-29", GeneratedAt: now, PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
	}
	planItems := []models.TradePlanItem{
		{
			TradeDate: "2026-08-29", StockCode: "sz000001", Side: "buy", Score: 0.9,
			StrategyName: "测试策略", Status: models.TradePlanItemPending,
		},
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tradeplans/%d/origin", plan.ID), nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body api.TradePlanOriginResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.OK)
	require.Equal(t, plan.ID, body.PlanID)
	require.Len(t, body.Items, 1)
	require.Equal(t, "强", body.Items[0].SignalTag)
	require.NotEqual(t, tradeplanorigin.Missing, body.Items[0].SourceReason)
}

func TestTradePlanOriginAPI_PlanNotFound(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/404/origin", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTradePlanOriginAPI_AllMissingFields(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())

	now := time.Now()
	plan := &models.TradePlan{TradeDate: "2026-08-29", GeneratedAt: now, Status: models.TradePlanStatusDraft, PlanVersion: 1}
	planItems := []models.TradePlanItem{
		{TradeDate: "2026-08-29", StockCode: "sh600000", Side: "buy", Status: models.TradePlanItemPending},
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))

	mux := registerTradePlansTestMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tradeplans/%d/origin", plan.ID), nil)
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body api.TradePlanOriginResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	item := body.Items[0]
	require.False(t, item.Signal.Present)
	require.False(t, item.Reason.Present)
	require.False(t, item.Strategy.Present)
	require.Empty(t, item.SignalTime)
	require.Empty(t, item.SignalPrice)
	require.Empty(t, item.SignalTag)
	require.Empty(t, item.SourceReason)
	require.Empty(t, item.StrategyName)
	require.Empty(t, item.Score)
	require.Empty(t, item.SelectionReason)
}
