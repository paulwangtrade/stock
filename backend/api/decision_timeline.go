package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/research/decisiontimeline"
)

// DecisionTimelineHandler serves GET /api/research/decision-timeline.
// Read-only. It does not create TradePlan rows or orders.
type DecisionTimelineHandler struct{}

func NewDecisionTimelineHandler() *DecisionTimelineHandler {
	return &DecisionTimelineHandler{}
}

func (h *DecisionTimelineHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != "/api/research/decision-timeline" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	q := r.URL.Query()
	limit := 0
	if raw := strings.TrimSpace(q.Get("snapshot_limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	tl, err := decisiontimeline.Build(q.Get("stock_code"), decisiontimeline.Options{SnapshotLimit: limit})
	if err != nil {
		if errors.Is(err, decisiontimeline.ErrInvalidStockCode) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code": 400, "ok": false, "message": "请提供股票代码",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":     0,
		"ok":       true,
		"timeline": tl,
	})
}

// DecisionTimelineAssetMiddleware mounts the read-only decision timeline route.
func DecisionTimelineAssetMiddleware(next http.Handler) http.Handler {
	h := NewDecisionTimelineHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		if path == "/api/research/decision-timeline" {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
