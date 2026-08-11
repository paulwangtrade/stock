package api

import (
	"net/http"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
)

func (h *PaperObservationHandler) handleHoldingsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	eval, err := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": 500, "ok": false, "message": err.Error(),
		})
		return
	}
	decision := holdingdecision.EvaluateObservation(eval, holdingdecision.DefaultPolicy())
	summary := holdingdecision.BuildPortfolioObservation(snap, eval, decision)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"ok":      true,
		"summary": summary,
	})
}
