package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio"
)

// PortfolioDashboardHandler serves GET /api/portfolio/dashboard (read-only).
// Delegates entirely to portfolio.Service.Dashboard — no paper_sim reads in this layer.
type PortfolioDashboardHandler struct {
	svc portfolio.Service
}

// NewPortfolioDashboardHandler returns a handler. nil svc → portfolio.NewService().
func NewPortfolioDashboardHandler(svc portfolio.Service) *PortfolioDashboardHandler {
	if svc == nil {
		svc = portfolio.NewService()
	}
	return &PortfolioDashboardHandler{svc: svc}
}

func (h *PortfolioDashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/portfolio/dashboard" {
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
	view, err := h.svc.Dashboard(portfolio.DashboardOptions{
		TradeDate: tradeDate,
		AsOf:      time.Now(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":      0,
		"ok":        true,
		"dashboard": view,
	})
}

// RegisterPortfolioDashboardRoutes mounts the read-only portfolio dashboard route.
func RegisterPortfolioDashboardRoutes(mux *http.ServeMux) {
	RegisterPortfolioDashboardHandler(mux, nil)
	RegisterPortfolioSnapshotRoutes(mux)
	RegisterPortfolioIntelligenceRoutes(mux)
	RegisterInvestmentScoreRoutes(mux)
	RegisterDecisionSummaryRoutes(mux)
	RegisterPortfolioProvenanceRoutes(mux)
}

// RegisterPortfolioDashboardHandler mounts an injected handler (tests).
func RegisterPortfolioDashboardHandler(mux *http.ServeMux, h *PortfolioDashboardHandler) {
	if h == nil {
		h = NewPortfolioDashboardHandler(nil)
	}
	mux.Handle("/api/portfolio/dashboard", h)
}

// PortfolioDashboardAssetMiddleware mounts /api/portfolio/* on Wails AssetServer.
func PortfolioDashboardAssetMiddleware(next http.Handler) http.Handler {
	dash := NewPortfolioDashboardHandler(nil)
	snapH := NewPortfolioSnapshotHandler(nil)
	intel := NewPortfolioIntelligenceHandler(nil)
	scoreH := NewInvestmentScoreHandler(nil)
	decisionH := NewDecisionSummaryHandler(nil)
	posStateH := NewPositionStateHandler(nil)
	provenanceH := NewPortfolioProvenanceHandler(nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/portfolio/snapshot" {
			snapH.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/portfolio/intelligence" {
			intel.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/portfolio/decision-summary" {
			decisionH.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/portfolio/position-state" {
			posStateH.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/portfolio/investment-score/") {
			scoreH.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/portfolio/positions/") {
			provenanceH.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/portfolio/") {
			dash.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
