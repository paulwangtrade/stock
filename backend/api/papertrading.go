package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/papertrading"
)

// PaperTradingRunRequest POST /api/papertrading/run
type PaperTradingRunRequest struct {
	Actor     string `json:"actor"`
	Trigger   string `json:"trigger"` // must be "manual"
	TradeDate string `json:"trade_date,omitempty"`
	PlanID    uint   `json:"plan_id,omitempty"`
}

// PaperTradingRunResponse envelope.
type PaperTradingRunResponse struct {
	Code    int                            `json:"code"`
	OK      bool                           `json:"ok"`
	Message string                         `json:"message,omitempty"`
	Result  *papertrading.ExecutionResult  `json:"result,omitempty"`
}

// PaperTradingHandler serves /api/papertrading/* (isolated from Real Execution).
type PaperTradingHandler struct{}

func NewPaperTradingHandler() *PaperTradingHandler { return &PaperTradingHandler{} }

func (h *PaperTradingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Dashboard / status are served by PaperObservationHandler (Phase7-A3.1).
	switch r.URL.Path {
	case "/api/papertrading/run":
		h.handleRun(w, r)
	case "/api/papertrading/reports/daily":
		h.handleReportsDaily(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *PaperTradingHandler) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, PaperTradingRunResponse{
			Code: 405, OK: false, Message: "POST required",
		})
		return
	}
	var req PaperTradingRunRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, PaperTradingRunResponse{
			Code: 400, OK: false, Message: "invalid body: " + err.Error(),
		})
		return
	}
	req.Actor = strings.TrimSpace(req.Actor)
	req.Trigger = strings.TrimSpace(req.Trigger)
	if req.Actor == "" {
		writeJSON(w, http.StatusBadRequest, PaperTradingRunResponse{
			Code: 400, OK: false, Message: "actor is required",
		})
		return
	}
	if req.Trigger != papertrading.TriggerManual {
		writeJSON(w, http.StatusBadRequest, PaperTradingRunResponse{
			Code: 400, OK: false, Message: "trigger must be \"manual\"",
		})
		return
	}

	// Phase10-C.2-A: unified Execution Gateway (track B → paper_sim_*).
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        strings.TrimSpace(req.TradeDate),
		PlanID:           req.PlanID,
		Trigger:          papertrading.TriggerManual,
		Actor:            req.Actor,
		SkipWeekdayCheck: true, // calendar-day only; session policy is C.2-B
	})
	if err != nil {
		writeJSON(w, http.StatusConflict, PaperTradingRunResponse{
			Code: 409, OK: false, Message: err.Error(), Result: res,
		})
		return
	}
	status := ""
	if res != nil {
		status = res.Status
	}
	writeJSON(w, http.StatusOK, PaperTradingRunResponse{
		Code: 0, OK: true, Message: status, Result: res,
	})
}

func (h *PaperTradingHandler) handleReportsDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	q := r.URL.Query()
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	rows, total, err := papertrading.ListDailyReports(from, to, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0, "ok": true,
		"enabled": papertrading.IsEnabled(),
		"reports": rows,
		"total":   total,
		"from":    from,
		"to":      to,
	})
}

// RegisterPaperTradingRoutes mounts routes on mux (httptest).
// Read-only dashboard/status are registered via RegisterPaperObservationRoutes.
func RegisterPaperTradingRoutes(mux *http.ServeMux) {
	RegisterPaperObservationRoutes(mux)
	h := NewPaperTradingHandler()
	mux.Handle("/api/papertrading/run", h)
}

// PaperTradingAssetMiddleware mounts /api/papertrading/* on Wails AssetServer.
// Observation paths are dispatched to PaperObservationHandler; /run stays on PaperTradingHandler.
func PaperTradingAssetMiddleware(next http.Handler) http.Handler {
	obs := NewPaperObservationHandler()
	h := NewPaperTradingHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPaperObservationPath(r.URL.Path) {
			obs.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/papertrading/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
