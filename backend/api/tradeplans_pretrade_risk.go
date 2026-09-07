package api

import (
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/portfolio/pretrade"
)

func parseTradePlanPreTradeRiskPath(path string) (planID uint, ok bool) {
	path = strings.TrimSuffix(path, "/")
	const prefix = "/api/tradeplans/"
	const suffix = "/pretrade-risk"
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

type preTradeRiskEvaluator func(planID uint) (*pretrade.PreTradeRiskResult, error)

// WithPreTradeRiskEval injects PreTrade evaluator for tests.
func (h *TradePlansHandler) WithPreTradeRiskEval(eval preTradeRiskEvaluator) *TradePlansHandler {
	if h == nil {
		h = NewTradePlansHandler()
	}
	h.preTradeRiskEval = eval
	return h
}

func (h *TradePlansHandler) handlePreTradeRisk(w http.ResponseWriter, r *http.Request, planID uint) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	evaluate := h.preTradeRiskEval
	if evaluate == nil {
		svc := pretrade.NewService(nil)
		evaluate = svc.EvaluateByPlanID
	}
	result, err := evaluate(planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"code": TradePlanCodeNoUpcoming, "ok": false, "message": "plan not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": TradePlanCodeOK,
		"ok":   true,
		"report": result,
	})
}
