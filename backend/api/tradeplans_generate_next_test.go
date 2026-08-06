package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/strategy"

	"github.com/stretchr/testify/require"
)

func generateNextRequest(
	t *testing.T,
	body string,
	runner func(string) (*strategy.AfterCloseWorkflowResult, error),
) (*httptest.ResponseRecorder, api.TradePlanGenerateNextResponse) {
	t.Helper()
	h := api.NewTradePlansHandler().WithGenerateNextRunner(runner)
	mux := http.NewServeMux()
	api.RegisterTradePlansHandler(mux, h)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tradeplans/generate-next", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	var response api.TradePlanGenerateNextResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	return rec, response
}

func TestTradePlansGenerateNext_Success(t *testing.T) {
	var sourceDate string
	rec, response := generateNextRequest(t, `{"actor":"desktop-ui","source_date":"2026-07-26"}`,
		func(source string) (*strategy.AfterCloseWorkflowResult, error) {
			sourceDate = source
			return &strategy.AfterCloseWorkflowResult{
				SourceDate: "2026-07-26", TradeDate: "2026-07-27",
				CandidatePoolID: 11, TradePlanID: 22, PlanVersion: 3,
				RiskPassed: true, OK: true, Message: "after-close workflow completed",
			}, nil
		})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "2026-07-26", sourceDate)
	require.Equal(t, api.TradePlanCodeOK, response.Code)
	require.True(t, response.OK)
	require.Equal(t, "manual", response.Trigger)
	require.Equal(t, "desktop-ui", response.Actor)
	require.Equal(t, uint(22), response.PlanID)
	require.True(t, response.RiskPassed)
}

func TestTradePlansGenerateNext_RiskSoftFail(t *testing.T) {
	rec, response := generateNextRequest(t, `{"actor":"desktop-ui"}`,
		func(string) (*strategy.AfterCloseWorkflowResult, error) {
			return &strategy.AfterCloseWorkflowResult{
				SourceDate: "2026-07-26", TradeDate: "2026-07-27",
				CandidatePoolID: 11, TradePlanID: 22, PlanVersion: 3,
				OK: false, FailedStep: "risk", Message: "risk proposal did not pass",
			}, nil
		})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, api.TradePlanCodeOK, response.Code)
	require.False(t, response.OK)
	require.False(t, response.RiskPassed)
	require.Equal(t, "risk", response.FailedStep)
	require.Equal(t, uint(22), response.PlanID)
}

func TestTradePlansGenerateNext_ValidatesRequest(t *testing.T) {
	t.Run("actor required", func(t *testing.T) {
		rec, response := generateNextRequest(t, `{}`, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, api.TradePlanCodeBadActor, response.Code)
	})
	t.Run("source date format", func(t *testing.T) {
		rec, response := generateNextRequest(t, `{"actor":"desktop-ui","source_date":"07/26/2026"}`, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, api.TradePlanCodeBadTradeDate, response.Code)
	})
}

func TestTradePlansGenerateNext_EmptyDateKeepsDefault(t *testing.T) {
	var sourceDate string
	rec, response := generateNextRequest(t, `{"actor":"test"}`,
		func(source string) (*strategy.AfterCloseWorkflowResult, error) {
			sourceDate = source
			return &strategy.AfterCloseWorkflowResult{
				SourceDate: "2026-08-05", TradeDate: "2026-08-06",
				CandidatePoolID: 1, TradePlanID: 2, PlanVersion: 1,
				RiskPassed: true, OK: true, Message: "ok",
			}, nil
		})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "", sourceDate) // empty → workflow uses today
	require.Equal(t, api.TradePlanCodeOK, response.Code)
	require.True(t, response.OK)
}

func TestTradePlansGenerateNext_SourceDate(t *testing.T) {
	var sourceDate string
	rec, response := generateNextRequest(t, `{"actor":"test","source_date":"2026-08-05"}`,
		func(source string) (*strategy.AfterCloseWorkflowResult, error) {
			sourceDate = source
			return &strategy.AfterCloseWorkflowResult{
				SourceDate: "2026-08-05", TradeDate: "2026-08-06",
				CandidatePoolID: 1, TradePlanID: 2, PlanVersion: 1,
				RiskPassed: true, OK: true, Message: "ok",
			}, nil
		})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "2026-08-05", sourceDate)
	require.Equal(t, "2026-08-06", response.TradeDate)
	require.True(t, response.OK)
}

func TestTradePlansGenerateNext_TradeDateConvertsToSource(t *testing.T) {
	var sourceDate string
	rec, response := generateNextRequest(t, `{"actor":"test","trade_date":"2026-08-06"}`,
		func(source string) (*strategy.AfterCloseWorkflowResult, error) {
			sourceDate = source
			return &strategy.AfterCloseWorkflowResult{
				SourceDate: source, TradeDate: "2026-08-06",
				CandidatePoolID: 1, TradePlanID: 2, PlanVersion: 1,
				RiskPassed: true, OK: true, Message: "ok",
			}, nil
		})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "2026-08-05", sourceDate)
	require.Equal(t, "2026-08-06", response.TradeDate)
	require.True(t, response.OK)
}

func TestTradePlansGenerateNext_BothDatesRejected(t *testing.T) {
	rec, response := generateNextRequest(t,
		`{"actor":"test","source_date":"2026-08-05","trade_date":"2026-08-06"}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, api.TradePlanCodeBadTradeDate, response.Code)
	require.Contains(t, response.Message, "source_date and trade_date cannot both be specified")
}

func TestTradePlansGenerateNext_NonTradingTradeDateRejected(t *testing.T) {
	// 2026-08-08 is Saturday
	rec, response := generateNextRequest(t, `{"actor":"test","trade_date":"2026-08-08"}`, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, api.TradePlanCodeBadTradeDate, response.Code)
	require.Contains(t, response.Message, "trade_date must be a trading day")
}

func TestTradePlansGenerateNext_HardFailUsesBusinessEnvelope(t *testing.T) {
	rec, response := generateNextRequest(t, `{"actor":"desktop-ui"}`,
		func(string) (*strategy.AfterCloseWorkflowResult, error) {
			return &strategy.AfterCloseWorkflowResult{
				SourceDate: "2026-07-26", FailedStep: "candidate", Message: "candidate failed",
			}, errors.New("candidate failed")
		})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, api.TradePlanCodeInternalError, response.Code)
	require.False(t, response.OK)
	require.Equal(t, "candidate", response.FailedStep)
}

func TestTradePlansGenerateNext_MethodNotAllowed(t *testing.T) {
	h := api.NewTradePlansHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tradeplans/generate-next", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
