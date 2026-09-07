package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/portfolio/summary"
)

// DailyInvestmentSummaryHandler serves GET /api/investment/daily-summary (read-only).
type DailyInvestmentSummaryHandler struct {
	svc *summary.Service
}

// NewDailyInvestmentSummaryHandler returns handler. nil svc → summary.NewService(nil, nil).
func NewDailyInvestmentSummaryHandler(svc *summary.Service) *DailyInvestmentSummaryHandler {
	if svc == nil {
		svc = summary.NewService(nil, nil)
	}
	return &DailyInvestmentSummaryHandler{svc: svc}
}

func (h *DailyInvestmentSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/investment/daily-summary" {
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
	view := h.svc.Evaluate(summary.Query{TradeDate: tradeDate, AsOf: time.Now()})
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"ok":      true,
		"summary": view,
	})
}

// RegisterDailyInvestmentSummaryRoutes mounts the daily summary route.
func RegisterDailyInvestmentSummaryRoutes(mux *http.ServeMux) {
	mux.Handle("/api/investment/daily-summary", NewDailyInvestmentSummaryHandler(nil))
	RegisterInvestmentHomeRoutes(mux)
	RegisterDailyAttentionRoutes(mux)
}

// DailyInvestmentSummaryAssetMiddleware mounts /api/investment/* on Wails.
// Routes: /api/investment/home, /daily-summary, /daily-attention.
func DailyInvestmentSummaryAssetMiddleware(next http.Handler) http.Handler {
	daily := NewDailyInvestmentSummaryHandler(nil)
	homeH := NewInvestmentHomeHandler(nil)
	attn := NewDailyAttentionHandler(nil)
	narrativeH := NewInvestmentNarrativeHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/investment/") {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/investment/narrative/") {
			narrativeH.ServeHTTP(w, r)
			return
		}
		switch r.URL.Path {
		case "/api/investment/home":
			homeH.ServeHTTP(w, r)
		case "/api/investment/daily-summary":
			daily.ServeHTTP(w, r)
		case "/api/investment/daily-attention":
			attn.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}
