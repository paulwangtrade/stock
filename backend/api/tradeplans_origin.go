package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/tradeplanorigin"
)

// TradePlanOriginResponse GET /api/tradeplans/{id}/origin envelope.
type TradePlanOriginResponse struct {
	Code    int                       `json:"code"`
	OK      bool                      `json:"ok"`
	PlanID  uint                      `json:"plan_id,omitempty"`
	Items   []tradePlanOriginItemWire `json:"items"`
	Message string                    `json:"message,omitempty"`
}

func parseTradePlanOriginPath(path string) (planID uint, ok bool) {
	path = strings.TrimSuffix(path, "/")
	const prefix = "/api/tradeplans/"
	const suffix = "/origin"
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

func (h *TradePlansHandler) handleOrigin(w http.ResponseWriter, r *http.Request, planID uint) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, TradePlanOriginResponse{
			Code: 405, OK: false, Message: "GET required",
		})
		return
	}
	items, err := tradeplanorigin.ProjectPlanOrigin(planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, TradePlanOriginResponse{
			Code: TradePlanCodeNoUpcoming, OK: false, Message: "plan not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, TradePlanOriginResponse{
		Code: TradePlanCodeOK, OK: true, PlanID: planID, Items: wireOriginItems(items),
	})
}
