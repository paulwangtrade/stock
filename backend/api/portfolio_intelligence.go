package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/intelligence"
)

// PortfolioIntelligenceHandler serves GET /api/portfolio/intelligence (read-only).
type PortfolioIntelligenceHandler struct {
	svc *intelligence.Service
}

// NewPortfolioIntelligenceHandler returns handler. nil svc → intelligence.NewService(nil).
func NewPortfolioIntelligenceHandler(svc *intelligence.Service) *PortfolioIntelligenceHandler {
	if svc == nil {
		svc = intelligence.NewService(nil)
	}
	return &PortfolioIntelligenceHandler{svc: svc}
}

func (h *PortfolioIntelligenceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/portfolio/intelligence" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	extra := splitCSV(strings.TrimSpace(r.URL.Query().Get("stock_codes")))
	bundle, err := h.svc.Evaluate(intelligence.Query{
		AsOf:       time.Now(),
		ExtraCodes: extra,
	})
	if err != nil && bundle == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	if bundle == nil {
		bundle = &intelligence.Bundle{Positions: []intelligence.PositionIntelligenceView{}}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":      0,
		"ok":        true,
		"found":     bundle.Found,
		"positions": bundle.Positions,
		"as_of":     bundle.AsOf,
		"disclaimer": bundle.Disclaimer,
		"data_source_note": bundle.DataSourceNote,
	})
}

// RegisterPortfolioIntelligenceRoutes mounts the intelligence route.
func RegisterPortfolioIntelligenceRoutes(mux *http.ServeMux) {
	mux.Handle("/api/portfolio/intelligence", NewPortfolioIntelligenceHandler(nil))
}
