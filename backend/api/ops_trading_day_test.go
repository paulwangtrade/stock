package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestOpsTradingDayAPI_FrozenReady(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	data.ResetTradingCronRegistryForTest()
	data.ReportTradingCronRegistered(data.TradingCronKeyAfterClose)
	data.ReportTradingCronRegistered(data.TradingCronKeyMorning)
	data.SetTradingDaySchemaStatusProvider(func() data.TradingDaySchemaView {
		return data.TradingDaySchemaView{RegistryVersion: 3, ValidationStatus: db.SchemaValidationReady}
	})
	t.Cleanup(func() {
		data.ResetTradingCronRegistryForTest()
		data.SetTradingDaySchemaStatusProvider(nil)
	})

	repo := data.NewTradePlanRepo()
	now := time.Now()
	freezeAt := now.Add(-time.Hour)
	tradeDate := "2026-07-27"
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 1,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		ApprovedAt: &freezeAt, ApprovedBy: "approver",
		FreezeAt: &freezeAt, FreezeBy: "ops",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, nil))

	mux := http.NewServeMux()
	api.RegisterOpsTradingDayRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/ops/trading_day_status?trade_date="+tradeDate, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradingDayStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.OpsTradingDayCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.NotNil(t, resp.Data)
	require.True(t, resp.Data.ExecutionReady.Ready)
	require.Equal(t, data.ExecutionGuardWouldPass, resp.Data.ExecutionReady.GuardStatus)
	require.True(t, resp.Data.Freeze.IsFrozen)
	require.Equal(t, data.MorningModeAdoptFrozen, resp.Data.Morning.Mode)
}

func TestOpsTradingDayAPI_BadTradeDate(t *testing.T) {
	setupAPITestDB(t)
	mux := http.NewServeMux()
	api.RegisterOpsTradingDayRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/ops/trading_day_status?trade_date=not-a-date", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp api.TradingDayStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.OpsTradingDayCodeBadTradeDate, resp.Code)
	require.False(t, resp.OK)
}
