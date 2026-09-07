package api

import (
	"net/http"
	"strings"

	"go-stock/backend/papertrading"
)

func (h *PaperObservationHandler) handleExitEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	stockCode := strings.TrimSpace(r.URL.Query().Get("stock_code"))
	view, err := papertrading.BuildExitEvaluation(papertrading.ExitEvaluationBuildOptions{
		StockCode: stockCode,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	if mergeErr := papertrading.AttachLatestOutcomes(view); mergeErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": mergeErr.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":             0,
		"ok":               true,
		"exit_evaluation":  view,
	})
}
