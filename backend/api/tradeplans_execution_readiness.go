package api

import (
	"net/http"
	"strconv"
	"strings"

	tpreadiness "go-stock/backend/tradingplan/readiness"
)

func parseTradePlanExecutionReadinessPath(path string) (planID uint, ok bool) {
	path = strings.TrimSuffix(path, "/")
	const prefix = "/api/tradeplans/"
	const suffix = "/execution-readiness"
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

// ExecutionReadinessResponse GET /api/tradeplans/{id}/execution-readiness envelope.
type ExecutionReadinessResponse struct {
	Code           int     `json:"code"`
	OK             bool    `json:"ok"`
	Status         string  `json:"status"`
	CashEnough     bool    `json:"cash_enough"`
	AvailableCash  float64 `json:"available_cash"`
	RequiredCash   float64 `json:"required_cash"`
	TotalEquity    float64 `json:"total_equity"`
	Conflicts      any     `json:"conflicts"`
	Concentration  string  `json:"concentration"`
	Issues         any     `json:"issues,omitempty"`
	AfterExecution any     `json:"after_execution,omitempty"`
	Message        string  `json:"message,omitempty"`
}

type executionReadinessEvaluator func(planID uint) (tpreadiness.ExecutionReadiness, error)

func (h *TradePlansHandler) handleExecutionReadiness(w http.ResponseWriter, r *http.Request, planID uint) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ExecutionReadinessResponse{
			Code: 405, OK: false, Message: "GET required",
		})
		return
	}
	evaluate := h.executionReadinessEval
	if evaluate == nil {
		evaluate = tpreadiness.LoadAndEvaluate
	}
	result, err := evaluate(planID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, ExecutionReadinessResponse{
			Code: TradePlanCodeNoUpcoming, OK: false, Message: "plan not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, mapExecutionReadinessResponse(result))
}

func mapExecutionReadinessResponse(r tpreadiness.ExecutionReadiness) ExecutionReadinessResponse {
	conflicts := r.ExistingPositions
	if conflicts == nil {
		conflicts = []tpreadiness.PositionConflict{}
	}
	issues := r.Issues
	if issues == nil {
		issues = []tpreadiness.ReadinessIssue{}
	}
	after := r.AfterExecution
	if after == nil {
		after = []tpreadiness.PositionWeightProjection{}
	}
	return ExecutionReadinessResponse{
		Code:           TradePlanCodeOK,
		OK:             true,
		Status:         r.Status,
		CashEnough:     r.CashEnough,
		AvailableCash:  r.AvailableCash,
		RequiredCash:   r.RequiredCash,
		TotalEquity:    r.TotalEquity,
		Conflicts:      conflicts,
		Concentration:  r.Concentration,
		Issues:         issues,
		AfterExecution: after,
	}
}
