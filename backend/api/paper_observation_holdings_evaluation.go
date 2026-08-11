package api

import (
	"net/http"
	"strings"

	"go-stock/backend/holdingdecision"
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
	decision := holdingdecision.EvaluateObservation(view, holdingdecision.DefaultPolicy())
	writeJSON(w, http.StatusOK, map[string]any{
		"code":              0,
		"ok":                true,
		"evaluation":        view,
		"holding_decision":  decision,
		"decision_state":    summaryDecisionState(decision),
		"decision_reason":   summaryDecisionReason(decision),
	})
}

func summaryDecisionState(v *holdingdecision.View) string {
	if v == nil || len(v.Holdings) == 0 {
		return holdingdecision.StateHoldNormal
	}
	worst := holdingdecision.StateHoldNormal
	rank := map[string]int{
		holdingdecision.StateHoldNormal:    0,
		holdingdecision.StateHoldWatch:     1,
		holdingdecision.StateHoldReview:    2,
		holdingdecision.StateExitCandidate: 3,
	}
	for _, h := range v.Holdings {
		if rank[h.State] > rank[worst] {
			worst = h.State
		}
	}
	return worst
}

func summaryDecisionReason(v *holdingdecision.View) string {
	if v == nil || len(v.Holdings) == 0 {
		return holdingdecision.ReasonNone
	}
	want := summaryDecisionState(v)
	for _, h := range v.Holdings {
		if h.State == want {
			return h.Reason
		}
	}
	return v.Holdings[0].Reason
}
