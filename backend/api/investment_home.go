package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/home"
)

// InvestmentHomeHandler serves GET /api/investment/home (read-only aggregate).
type InvestmentHomeHandler struct {
	svc *home.Service
}

// NewInvestmentHomeHandler returns handler. nil svc → home.NewService(nil).
func NewInvestmentHomeHandler(svc *home.Service) *InvestmentHomeHandler {
	if svc == nil {
		svc = home.NewService(nil)
	}
	return &InvestmentHomeHandler{svc: svc}
}

func (h *InvestmentHomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/investment/home" {
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
	view := h.svc.Build(home.Query{TradeDate: tradeDate, AsOf: time.Now()})
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"ok":   true,
		"home": view,
	})
}

// RegisterInvestmentHomeRoutes mounts the investment home route.
func RegisterInvestmentHomeRoutes(mux *http.ServeMux) {
	mux.Handle("/api/investment/home", NewInvestmentHomeHandler(nil))
}
