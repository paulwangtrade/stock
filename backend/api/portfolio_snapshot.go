package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/readmodel"
)

// PortfolioSnapshotHandler serves GET /api/portfolio/snapshot (read-only).
// Additive: does not replace dashboard / position-state.
type PortfolioSnapshotHandler struct {
	svc *readmodel.Service
}

// NewPortfolioSnapshotHandler returns handler. nil svc → readmodel.NewService(nil).
func NewPortfolioSnapshotHandler(svc *readmodel.Service) *PortfolioSnapshotHandler {
	if svc == nil {
		svc = readmodel.NewService(nil)
	}
	return &PortfolioSnapshotHandler{svc: svc}
}

func (h *PortfolioSnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/portfolio/snapshot" {
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
	view, err := h.svc.Evaluate(readmodel.Query{
		TradeDate:      tradeDate,
		AsOf:           time.Now(),
		IncludeDisplay: parseIncludeDisplay(r.URL.Query().Get("include_display")),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":     0,
		"ok":       true,
		"snapshot": view,
	})
}

func parseIncludeDisplay(raw string) bool {
	s := strings.TrimSpace(strings.ToLower(raw))
	return s == "1" || s == "true" || s == "yes"
}

// RegisterPortfolioSnapshotRoutes mounts the snapshot route (explicit path, no dashboard fallback).
func RegisterPortfolioSnapshotRoutes(mux *http.ServeMux) {
	mux.Handle("/api/portfolio/snapshot", NewPortfolioSnapshotHandler(nil))
}
