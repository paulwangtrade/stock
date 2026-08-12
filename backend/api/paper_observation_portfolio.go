package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolioobs"
	"go-stock/backend/rebalance"
)

func (h *PaperObservationHandler) handlePortfolioObservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": 405, "ok": false, "message": "GET required",
		})
		return
	}
	var warnings []string

	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	if err != nil {
		warnings = append(warnings, "snapshot: "+err.Error())
	}

	eval, evalErr := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{})
	if evalErr != nil {
		warnings = append(warnings, "evaluation: "+evalErr.Error())
		eval = nil
	}

	decision := holdingdecision.EvaluateObservation(eval, holdingdecision.DefaultPolicy())
	current := rebalance.CurrentFromSnapshot(snap)
	if decision != nil {
		by := map[string]string{}
		for _, row := range decision.Holdings {
			by[row.Symbol] = row.State
		}
		rebalance.AttachDecisions(current, by)
	}

	q := r.URL.Query()
	opts := rebalance.ObservationTargetOptions{
		Identity: strings.EqualFold(strings.TrimSpace(q.Get("target")), "identity"),
	}
	if enter := strings.TrimSpace(q.Get("enter")); enter != "" {
		opts.EnterSymbols = splitCSV(enter)
	}
	if drop := strings.TrimSpace(q.Get("drop")); drop != "" {
		opts.DropSymbols = splitCSV(drop)
	}
	target := rebalance.BuildObservationTarget(current, opts)
	diff := rebalance.Diff(current, target, rebalance.DefaultPolicy())

	obs := portfolioobs.Assemble(snap, eval, decision, diff, target, warnings, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{
		"code":       0,
		"ok":         true,
		"disclaimer": portfolioobs.Disclaimer,
		"portfolio":  obs,
	})
}
