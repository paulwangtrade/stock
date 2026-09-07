package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/portfolio/pretrade"

	"github.com/stretchr/testify/require"
)

func TestTradePlanPreTradeRiskAPI_OK(t *testing.T) {
	planID := uint(42)
	mock := func(id uint) (*pretrade.PreTradeRiskResult, error) {
		require.Equal(t, planID, id)
		return pretrade.Build(pretrade.Inputs{
			PlanID:    id,
			TradeDate: "2026-08-18",
			AsOf:      time.Date(2026, 8, 18, 9, 28, 0, 0, time.Local),
			Account:   pretrade.AccountInput{Cash: 500_000, Equity: 2_000_000},
			Lines: []pretrade.PlanLine{
				{StockCode: "sz000001", Side: "buy", TargetAmount: 100_000},
			},
			RiskSnapshot: pretrade.RiskSnapshot{PlanRiskStatus: "passed", MarketLevel: 0},
		}), nil
	}
	h := api.NewTradePlansHandler().WithPreTradeRiskEval(mock)
	mux := http.NewServeMux()
	mux.Handle("/api/tradeplans/", h)

	rec := httptest.NewRecorder()
	url := "/api/tradeplans/" + strconv.FormatUint(uint64(planID), 10) + "/pretrade-risk"
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	report := body["report"].(map[string]any)
	require.Equal(t, "PASS", report["level"])
	require.Contains(t, report, "cash_check")
	require.Contains(t, report, "existing_position_check")
	require.Contains(t, report, "concentration_check")
	require.Contains(t, report, "risk_snapshot")
	require.Contains(t, report, "position_action")
}

func TestTradePlanPreTradeRiskAPI_InsufficientCashVisible(t *testing.T) {
	mock := func(id uint) (*pretrade.PreTradeRiskResult, error) {
		return pretrade.Build(pretrade.Inputs{
			PlanID:  id,
			Account: pretrade.AccountInput{Cash: 50_000, Equity: 1_000_000},
			Lines: []pretrade.PlanLine{
				{StockCode: "sz000001", Side: "buy", TargetAmount: 200_000},
			},
		}), nil
	}
	h := api.NewTradePlansHandler().WithPreTradeRiskEval(mock)
	mux := http.NewServeMux()
	mux.Handle("/api/tradeplans/", h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/7/pretrade-risk", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	report := body["report"].(map[string]any)
	require.Equal(t, "BLOCKED", report["level"])
	cash := report["cash_check"].(map[string]any)
	require.Equal(t, "BLOCKED", cash["status"])
}

func TestTradePlanPreTradeRiskAPI_NotFound(t *testing.T) {
	mock := func(id uint) (*pretrade.PreTradeRiskResult, error) {
		return nil, fmt.Errorf("plan not found")
	}
	h := api.NewTradePlansHandler().WithPreTradeRiskEval(mock)
	mux := http.NewServeMux()
	mux.Handle("/api/tradeplans/", h)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/999/pretrade-risk", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTradePlanPreTradeRiskAPI_GETOnly(t *testing.T) {
	h := api.NewTradePlansHandler().WithPreTradeRiskEval(func(uint) (*pretrade.PreTradeRiskResult, error) {
		return &pretrade.PreTradeRiskResult{Level: pretrade.LevelPass}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/api/tradeplans/", h)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tradeplans/1/pretrade-risk", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
