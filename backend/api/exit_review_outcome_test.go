package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func postExitReviewOutcome(t *testing.T, mux http.Handler, body map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/exit-review/outcome", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp["ok"].(bool))
	return resp
}

func TestExitReviewOutcomeAPI_HOLD_WATCH_CREATE_SELL_PLAN(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)

	mux := http.NewServeMux()
	api.RegisterExitReviewRoutes(mux)
	api.RegisterPaperObservationRoutes(mux)

	postExitReviewOutcome(t, mux, map[string]any{
		"stock_code":            "sh600363",
		"decision":              papertrading.ExitReviewDecisionHold,
		"reason":                "逻辑仍成立",
		"exit_state_snapshot":   papertrading.ExitEvalStateReviewRequired,
		"reason_codes_snapshot": []string{papertrading.ExitReasonLossReview},
		"account_id":            acc.ID,
	})

	postExitReviewOutcome(t, mux, map[string]any{
		"stock_code":          "sz000001",
		"decision":            papertrading.ExitReviewDecisionWatch,
		"exit_state_snapshot": papertrading.ExitEvalStateWatch,
		"account_id":          acc.ID,
	})

	postExitReviewOutcome(t, mux, map[string]any{
		"stock_code":             "sh600105",
		"decision":               papertrading.ExitReviewDecisionCreateSellPlan,
		"exit_state_snapshot":    papertrading.ExitEvalStateReviewRequired,
		"related_trade_plan_id":  99,
		"account_id":             acc.ID,
	})
}

func TestExitEvaluationAPI_LatestOutcome(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	today := time.Now().Format("2006-01-02")
	_, _ = seedExitEvalFill(t, acc.ID, today, "sz000001", "平安银行", 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 8.5,
	}).Error)

	mux := http.NewServeMux()
	api.RegisterExitReviewRoutes(mux)
	api.RegisterPaperObservationRoutes(mux)

	reviewTime := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	postExitReviewOutcome(t, mux, map[string]any{
		"stock_code":            "sz000001",
		"decision":              papertrading.ExitReviewDecisionHold,
		"reason":                "等待反弹",
		"review_time":           reviewTime,
		"exit_state_snapshot":   papertrading.ExitEvalStateReviewRequired,
		"reason_codes_snapshot": []string{papertrading.ExitReasonLossReview},
		"account_id":            acc.ID,
	})

	ev := getExitEvaluation(t)
	holdings := ev["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	latest, ok := row["latest_outcome"].(map[string]any)
	require.True(t, ok, "latest_outcome missing")
	require.Equal(t, papertrading.ExitReviewDecisionHold, latest["decision"])
	require.Equal(t, "等待反弹", latest["reason"])
	require.NotEmpty(t, latest["review_time"])
}

func TestExitReviewOutcomeAPI_InvalidDecision(t *testing.T) {
	setupAPITestDB(t)
	_ = seedExitEvalAccount(t)

	mux := http.NewServeMux()
	api.RegisterExitReviewRoutes(mux)

	rec := httptest.NewRecorder()
	body := []byte(`{"stock_code":"sh600363","decision":"SELL","exit_state_snapshot":"WATCH"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/exit-review/outcome", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestExitReviewOutcomeAPI_DuplicateSubmit(t *testing.T) {
	setupAPITestDB(t)
	acc := seedExitEvalAccount(t)
	today := time.Now().Format("2006-01-02")
	_, _ = seedExitEvalFill(t, acc.ID, today, "sh600363", "联创光电", 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 8.0,
	}).Error)

	mux := http.NewServeMux()
	api.RegisterExitReviewRoutes(mux)
	api.RegisterPaperObservationRoutes(mux)

	postExitReviewOutcome(t, mux, map[string]any{
		"stock_code":          "sh600363",
		"decision":            papertrading.ExitReviewDecisionHold,
		"reason":              "先 HOLD",
		"exit_state_snapshot": papertrading.ExitEvalStateReviewRequired,
		"account_id":          acc.ID,
	})

	postExitReviewOutcome(t, mux, map[string]any{
		"stock_code":          "sh600363",
		"decision":            papertrading.ExitReviewDecisionCreateSellPlan,
		"reason":              "后改卖计划意图",
		"exit_state_snapshot": papertrading.ExitEvalStateReviewRequired,
		"account_id":          acc.ID,
	})

	var count int64
	require.NoError(t, db.Dao.Model(&papertrading.ExitReviewOutcome{}).
		Where("account_id = ? AND stock_code = ?", acc.ID, "sh600363").
		Count(&count).Error)
	require.Equal(t, int64(2), count)

	ev := getExitEvaluation(t)
	row := ev["holdings"].([]any)[0].(map[string]any)
	latest := row["latest_outcome"].(map[string]any)
	require.Equal(t, papertrading.ExitReviewDecisionCreateSellPlan, latest["decision"])
	require.Equal(t, "后改卖计划意图", latest["reason"])
}
