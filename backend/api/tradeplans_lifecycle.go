package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/papertrading"
)

// TradePlanLifecycleResponse GET /api/tradeplans/{id}/lifecycle envelope.
type TradePlanLifecycleResponse struct {
	Code      int                                  `json:"code"`
	OK        bool                                 `json:"ok"`
	Lifecycle *papertrading.TradePlanLifecycleView `json:"lifecycle"`
	Message   string                               `json:"message,omitempty"`
}

func parseTradePlanLifecyclePath(path string) (planID uint, ok bool) {
	path = strings.TrimSuffix(path, "/")
	const prefix = "/api/tradeplans/"
	const suffix = "/lifecycle"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return 0, false
	}
	mid := strings.TrimPrefix(path, prefix)
	mid = strings.TrimSuffix(mid, suffix)
	mid = strings.Trim(mid, "/")
	if mid == "" || strings.Contains(mid, "/") {
		return 0, false
	}
	id64, err := strconv.ParseUint(mid, 10, 64)
	if err != nil || id64 == 0 {
		return 0, false
	}
	return uint(id64), true
}

func (h *TradePlansHandler) handleLifecycle(w http.ResponseWriter, r *http.Request, planID uint) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, TradePlanLifecycleResponse{
			Code: 405, OK: false, Message: "GET required",
		})
		return
	}
	view, err := papertrading.BuildTradePlanLifecycle(planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, TradePlanLifecycleResponse{
			Code: TradePlanCodeNoUpcoming, OK: false, Message: "plan not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, TradePlanLifecycleResponse{
		Code: TradePlanCodeOK, OK: true, Lifecycle: view,
	})
}
