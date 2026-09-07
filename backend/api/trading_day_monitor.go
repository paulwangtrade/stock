package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/tradingdaymonitor"
)

// TradingDayMonitorHandler serves GET /api/trading/day-monitor (read-only).
type TradingDayMonitorHandler struct{}

func NewTradingDayMonitorHandler() *TradingDayMonitorHandler {
	return &TradingDayMonitorHandler{}
}

func (h *TradingDayMonitorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/trading/day-monitor" {
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
	view := tradingdaymonitor.Build(tradingdaymonitor.Options{
		TradeDate: tradeDate,
		AsOf:      time.Now(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"ok":      true,
		"monitor": view,
	})
}

// RegisterTradingDayMonitorRoutes mounts the Day Monitor route.
func RegisterTradingDayMonitorRoutes(mux *http.ServeMux) {
	mux.Handle("/api/trading/day-monitor", NewTradingDayMonitorHandler())
}

// TradingDayMonitorAssetMiddleware mounts /api/trading/* Day Monitor on Wails.
func TradingDayMonitorAssetMiddleware(next http.Handler) http.Handler {
	h := NewTradingDayMonitorHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/trading/day-monitor" || strings.HasPrefix(r.URL.Path, "/api/trading/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
