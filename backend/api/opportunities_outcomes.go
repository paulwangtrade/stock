package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/opportunity/outcome"
)

// handleOutcomes serves GET /api/opportunities/outcomes (Phase16-D3 read-only).
func (h *OpportunitiesHandler) handleOutcomes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	svc := h.outcomes
	if svc == nil {
		svc = outcome.NewService()
	}

	q := r.URL.Query()
	limit := 0
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}

	opts := outcome.ProjectOptions{
		TradeDate:      strings.TrimSpace(q.Get("trade_date")),
		Status:         strings.TrimSpace(q.Get("status")),
		Limit:          limit,
		IncludeNoTrade: true,
	}

	stockCode := strings.TrimSpace(q.Get("stock_code"))
	var (
		items []outcome.OutcomeProjection
		err   error
	)
	if stockCode != "" {
		items, err = svc.ProjectStock(stockCode, opts)
	} else {
		items, err = svc.ProjectList(opts)
	}
	if err != nil {
		if errors.Is(err, outcome.ErrInvalidStockCode) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code": 400, "ok": false, "message": err.Error(),
			})
			return
		}
		if errors.Is(err, outcome.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"code": 404, "ok": false, "message": "outcome not found",
				"items": []any{}, "generated_at": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	if items == nil {
		items = []outcome.OutcomeProjection{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":          0,
		"ok":            true,
		"items":         items,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
	})
}
