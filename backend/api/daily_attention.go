package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/attention"
)

// DailyAttentionHandler serves GET /api/investment/daily-attention (read-only).
type DailyAttentionHandler struct {
	svc *attention.Service
}

// NewDailyAttentionHandler returns handler. nil svc → attention.NewService(nil).
func NewDailyAttentionHandler(svc *attention.Service) *DailyAttentionHandler {
	if svc == nil {
		svc = attention.NewService(nil)
	}
	return &DailyAttentionHandler{svc: svc}
}

func (h *DailyAttentionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/investment/daily-attention" {
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
	view := h.svc.BuildDailyAttention(tradeDate)
	if view == nil {
		view = attention.Build(attention.Inputs{TradeDate: tradeDate, AsOf: time.Now()})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":      0,
		"ok":        true,
		"attention": view,
	})
}

// RegisterDailyAttentionRoutes mounts the daily-attention route.
func RegisterDailyAttentionRoutes(mux *http.ServeMux) {
	mux.Handle("/api/investment/daily-attention", NewDailyAttentionHandler(nil))
}
