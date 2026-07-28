// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A3.1：Paper Observation 只读 HTTP（status + dashboard/*），不含 /run

package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/papertrading"
)

// PaperObservationHandler serves read-only Paper Observation dashboard APIs.
// Does not mount POST /run or any PaperTradingJob / Broker / Settlement paths.
type PaperObservationHandler struct{}

func NewPaperObservationHandler() *PaperObservationHandler {
	return &PaperObservationHandler{}
}

func isPaperObservationPath(path string) bool {
	switch path {
	case "/api/papertrading/status",
		"/api/papertrading/dashboard/today",
		"/api/papertrading/dashboard/positions",
		"/api/papertrading/dashboard/runs":
		return true
	default:
		return false
	}
}

func (h *PaperObservationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/papertrading/status":
		h.handleStatus(w, r)
	case "/api/papertrading/dashboard/today":
		h.handleDashboardToday(w, r)
	case "/api/papertrading/dashboard/positions":
		h.handleDashboardPositions(w, r)
	case "/api/papertrading/dashboard/runs":
		h.handleDashboardRuns(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *PaperObservationHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	tradeDate := strings.TrimSpace(r.URL.Query().Get("trade_date"))
	st, err := papertrading.GetTodayStatus(tradeDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0, "ok": true, "status": st,
	})
}

func (h *PaperObservationHandler) handleDashboardToday(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	tradeDate := strings.TrimSpace(r.URL.Query().Get("trade_date"))
	view, err := papertrading.GetDashboardToday(tradeDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0, "ok": true, "today": view,
	})
}

func (h *PaperObservationHandler) handleDashboardPositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	view, err := papertrading.GetDashboardPositions()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	enrichObservationPositionNames(view, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0, "ok": true, "positions": view,
	})
}

func (h *PaperObservationHandler) handleDashboardRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	q := r.URL.Query()
	tradeDate := strings.TrimSpace(q.Get("trade_date"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	view, err := papertrading.ListRuns(tradeDate, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0, "ok": true, "runs": view,
	})
}

// RegisterPaperObservationRoutes mounts read-only observation routes on mux.
func RegisterPaperObservationRoutes(mux *http.ServeMux) {
	h := NewPaperObservationHandler()
	mux.Handle("/api/papertrading/status", h)
	mux.Handle("/api/papertrading/dashboard/today", h)
	mux.Handle("/api/papertrading/dashboard/positions", h)
	mux.Handle("/api/papertrading/dashboard/runs", h)
}

// PaperObservationAssetMiddleware mounts only read-only /api/papertrading observation paths.
func PaperObservationAssetMiddleware(next http.Handler) http.Handler {
	h := NewPaperObservationHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPaperObservationPath(r.URL.Path) {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
