package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/decision"
)

// DecisionSummaryHandler serves GET /api/portfolio/decision-summary.
type DecisionSummaryHandler struct {
	svc *decision.Service
}

// NewDecisionSummaryHandler returns handler. nil svc → decision.NewService(nil).
func NewDecisionSummaryHandler(svc *decision.Service) *DecisionSummaryHandler {
	if svc == nil {
		svc = decision.NewService(nil)
	}
	return &DecisionSummaryHandler{svc: svc}
}

func (h *DecisionSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/portfolio/decision-summary" {
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
	view := h.svc.Build(decision.Query{TradeDate: tradeDate, AsOf: time.Now()})
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"ok":      true,
		"summary": view,
	})
}

// RegisterDecisionSummaryRoutes mounts the decision-summary route.
func RegisterDecisionSummaryRoutes(mux *http.ServeMux) {
	mux.Handle("/api/portfolio/decision-summary", NewDecisionSummaryHandler(nil))
}
