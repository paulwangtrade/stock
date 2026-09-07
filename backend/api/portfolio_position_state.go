package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/positionstate"
)

// PositionStateHandler serves GET /api/portfolio/position-state (read-only).
type PositionStateHandler struct {
	svc *positionstate.Service
}

func NewPositionStateHandler(svc *positionstate.Service) *PositionStateHandler {
	if svc == nil {
		svc = positionstate.NewService(nil)
	}
	return &PositionStateHandler{svc: svc}
}

func (h *PositionStateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/portfolio/position-state" {
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
	view := h.svc.Evaluate(positionstate.Query{TradeDate: tradeDate, AsOf: time.Now()})
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"ok":   true,
		"position_states": view,
	})
}

// RegisterPositionStateRoutes mounts position-state.
func RegisterPositionStateRoutes(mux *http.ServeMux) {
	mux.Handle("/api/portfolio/position-state", NewPositionStateHandler(nil))
}
