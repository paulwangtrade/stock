package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/opportunity/projection"
)

// handleProjections serves GET /api/opportunities/projections (Phase16-C1 read-only).
func (h *OpportunitiesHandler) handleProjections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	svc := h.projections
	if svc == nil {
		svc = projection.NewService()
	}

	q := r.URL.Query()
	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}

	result, err := svc.Query(projection.QueryInput{
		TradeDate:  strings.TrimSpace(q.Get("trade_date")),
		StockCode:  strings.TrimSpace(q.Get("stock_code")),
		Limit:      limit,
		Status:     strings.TrimSpace(q.Get("status")),
		StrategyID: strings.TrimSpace(q.Get("strategy_id")),
	})
	if err != nil {
		if errors.Is(err, projection.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"code": 404, "ok": false, "message": "projection not found",
				"items": []any{}, "total": 0, "generated_at": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		if errors.Is(err, projection.ErrInvalidStockCode) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code": 400, "ok": false, "message": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":          0,
		"ok":            true,
		"items":         result.Items,
		"total":         result.Total,
		"generated_at":  result.GeneratedAt.UTC().Format(time.RFC3339),
	})
}
