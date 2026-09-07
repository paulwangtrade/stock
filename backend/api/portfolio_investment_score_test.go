package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/portfolio/score"

	"github.com/stretchr/testify/require"
)

// stubScoreService wraps Evaluate via a custom Service is hard; use handler with injected
// service by building through real NewInvestmentScoreHandler and testing Build via path
// that may return NOT_FOUND without DB — plus a dedicated inject test using middleware shape.

func TestInvestmentScoreAPI_PathAndShape(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterInvestmentScoreRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/investment-score/sz000001?trade_date=2099-01-02", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	sc := body["score"].(map[string]any)
	require.Equal(t, "sz000001", sc["stock_code"])
	require.Contains(t, sc, "subject_type")
	require.Contains(t, sc, "total_score")
	require.Contains(t, sc, "strategy_score")
	require.Contains(t, sc, "risk_score")
	require.Contains(t, sc, "momentum_score")
	require.Contains(t, sc, "portfolio_fit_score")
	require.Contains(t, sc, "quality")
	// Without DB pool/holding → NOT_FOUND, null components (no fake data)
	require.Equal(t, "NOT_FOUND", sc["quality"])
	require.Nil(t, sc["total_score"])
	require.Nil(t, sc["strategy_score"])
}

func TestInvestmentScoreAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterInvestmentScoreRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/portfolio/investment-score/sz000001", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestInvestmentScoreAPI_BuildEnvelopeMatchesView(t *testing.T) {
	// Ensure JSON nulls round-trip for missing components (candidate-only shape).
	view := score.Build(score.Inputs{
		StockCode: "sz000001",
		AsOf:      time.Date(2026, 8, 18, 10, 0, 0, 0, time.Local),
		Candidate: score.CandidateInput{Present: true, Score: 0.9, HasSignal: false},
	})
	require.Equal(t, score.SubjectCandidate, view.SubjectType)
	require.Nil(t, view.RiskScore)
	require.Nil(t, view.MomentumScore)
	require.Nil(t, view.PortfolioFitScore)
}

func TestPortfolioMiddleware_RoutesInvestmentScore(t *testing.T) {
	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		w.WriteHeader(http.StatusTeapot)
	})
	h := api.PortfolioDashboardAssetMiddleware(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/portfolio/investment-score/sz000001", nil))
	require.False(t, nextHit)
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
}
