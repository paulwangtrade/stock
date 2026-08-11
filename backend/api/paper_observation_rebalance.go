package api

import (
	"net/http"
	"strings"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/rebalance"
)

func (h *PaperObservationHandler) handleRebalanceObservation(w http.ResponseWriter, r *http.Request) {
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
	current := rebalance.CurrentFromSnapshot(snap)

	// Optional decision states for switch annotations (read-only).
	eval, evalErr := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{})
	if evalErr == nil && eval != nil {
		dec := holdingdecision.EvaluateObservation(eval, holdingdecision.DefaultPolicy())
		by := map[string]string{}
		for _, row := range dec.Holdings {
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

	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"ok":      true,
		"current": current,
		"target":  target,
		"diff":    diff,
		"note":    "调仓观察（Rebalance Observation）· 不是交易建议 · 不生成 BUY/SELL Intent 或订单",
	})
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
