package api

import (
	"net/http"
	"strings"

	"go-stock/backend/papertrading"
)

func (h *PaperObservationHandler) handleHoldingsEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	stockCode := strings.TrimSpace(r.URL.Query().Get("stock_code"))
	// GET-time QuoteService overlay (C.5-B.1); does not write mark_price.
	view, err := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{
		StockCode: stockCode,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":       0,
		"ok":         true,
		"evaluation": view,
	})
}
