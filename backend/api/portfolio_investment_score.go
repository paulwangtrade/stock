package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/score"
)

// InvestmentScoreHandler serves GET /api/portfolio/investment-score/{stock_code}.
type InvestmentScoreHandler struct {
	svc *score.Service
}

// NewInvestmentScoreHandler returns handler. nil svc → score.NewService(nil).
func NewInvestmentScoreHandler(svc *score.Service) *InvestmentScoreHandler {
	if svc == nil {
		svc = score.NewService(nil)
	}
	return &InvestmentScoreHandler{svc: svc}
}

func parseInvestmentScorePath(path string) (stockCode string, ok bool) {
	path = strings.TrimSuffix(path, "/")
	const prefix = "/api/portfolio/investment-score/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	code := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if code == "" || strings.Contains(code, "/") {
		return "", false
	}
	return code, true
}

func (h *InvestmentScoreHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code, ok := parseInvestmentScorePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	tradeDate := strings.TrimSpace(r.URL.Query().Get("trade_date"))
	view := h.svc.EvaluateByCode(score.Query{
		StockCode: code,
		TradeDate: tradeDate,
		AsOf:      time.Now(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"code":  0,
		"ok":    true,
		"score": view,
	})
}

// RegisterInvestmentScoreRoutes mounts the investment-score route pattern.
func RegisterInvestmentScoreRoutes(mux *http.ServeMux) {
	mux.Handle("/api/portfolio/investment-score/", NewInvestmentScoreHandler(nil))
}
