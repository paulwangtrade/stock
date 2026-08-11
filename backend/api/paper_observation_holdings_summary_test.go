package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func getHoldingsSummary(t *testing.T) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/holdings/summary", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	sum, ok := body["summary"].(map[string]any)
	require.True(t, ok)
	return sum
}

func TestHoldingsSummaryAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/holdings/summary", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHoldingsSummaryAPI_EmptyBook(t *testing.T) {
	setupAPITestDB(t)
	_ = seedEvalObsAccount(t)
	sum := getHoldingsSummary(t)
	require.Equal(t, "none", sum["action"])
	require.Equal(t, "HOLD_NORMAL", sum["portfolio_decision_state"])
	counts := sum["decision_counts"].(map[string]any)
	require.Equal(t, float64(0), counts["hold_normal_count"])
	require.Equal(t, float64(0), counts["hold_watch_count"])
	require.Equal(t, float64(0), counts["hold_review_count"])
	require.Equal(t, float64(0), counts["exit_candidate_count"])
}

func TestHoldingsSummaryAPI_NormalAndDoesNotMutateLedger(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)
	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 11.00, FetchedAt: time.Now(),
	}}})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	sum := getHoldingsSummary(t)
	require.Equal(t, "HOLD_NORMAL", sum["portfolio_decision_state"])
	counts := sum["decision_counts"].(map[string]any)
	require.Equal(t, float64(1), counts["hold_normal_count"])
	acct := sum["account"].(map[string]any)
	require.InDelta(t, 1_000_000.0, acct["cash"].(float64), 1e-6)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.00, pos.MarkPrice, 1e-9)
	var acc2 papertrading.PaperSimAccount
	require.NoError(t, db.Dao.First(&acc2, acc.ID).Error)
	require.InDelta(t, 1_000_000.0, acc2.Cash, 1e-6)
	require.NotContains(t, strings.ToUpper(mustJSON(t, sum)), "EXIT_NOW")
}

func TestHoldingsSummaryAPI_WatchAndReviewCounts(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)

	p1, i1 := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, p1, i1, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	p2, i2 := seedEvalObsPlanItem(t, "2026-08-11", "sz000002", "万科A")
	seedEvalObsFill(t, acc.ID, p2, i2, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000002", StockName: "万科A",
		TotalVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	p3, i3 := seedEvalObsPlanItem(t, "2026-08-11", "sz000003", "金证股份")
	seedEvalObsFill(t, acc.ID, p3, i3, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000003", StockName: "金证股份",
		TotalVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: []marketdata.Quote{
		{Code: "sz000001", Price: 11.00, FetchedAt: time.Now()}, // +10% NORMAL
		{Code: "sz000002", Price: 9.40, FetchedAt: time.Now()},  // -6% WATCH
		{Code: "sz000003", Price: 8.80, FetchedAt: time.Now()},  // -12% DANGER → REVIEW
	}})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	sum := getHoldingsSummary(t)
	counts := sum["decision_counts"].(map[string]any)
	require.Equal(t, float64(1), counts["hold_normal_count"])
	require.Equal(t, float64(1), counts["hold_watch_count"])
	require.Equal(t, float64(1), counts["hold_review_count"])
	require.Equal(t, float64(0), counts["exit_candidate_count"])
	require.Equal(t, "HOLD_REVIEW", sum["portfolio_decision_state"])
	require.Equal(t, "RISK_MATERIAL", sum["portfolio_decision_reason"])
	risk := sum["risk_distribution"].(map[string]any)
	require.Equal(t, float64(1), risk["normal_count"])
	require.Equal(t, float64(1), risk["watch_count"])
	require.Equal(t, float64(1), risk["danger_count"])
	dmv := sum["decision_market_value"].(map[string]any)
	require.Greater(t, dmv["hold_review_weight"].(float64), 0.0)
	require.Equal(t, false, sum["exit_candidate_enabled"])
	require.Equal(t, "none", sum["action"])
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
