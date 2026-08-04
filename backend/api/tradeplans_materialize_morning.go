package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-stock/backend/logger"
	"go-stock/backend/papertrading"
	"go-stock/backend/strategy"
)

const (
	TradePlanCodeMaterializeDeny = 40910
)

// TradePlanMaterializeMorningRequest is the controlled morning materialize request.
type TradePlanMaterializeMorningRequest struct {
	PlanID uint `json:"plan_id"`
}

// TradePlanMaterializeMorningResponse is the Phase10-A.1 API envelope.
type TradePlanMaterializeMorningResponse struct {
	Success           bool                   `json:"success"`
	PlanID            uint                   `json:"plan_id"`
	MaterializedItems int                    `json:"materialized_items"`
	ReadinessReady    bool                   `json:"readiness_ready"`
	Blockers          []ReadinessFindingView `json:"blockers"`
	Code              int                    `json:"code"`
	Message           string                 `json:"message,omitempty"`
	FailedStep        string                 `json:"failed_step,omitempty"`
	PricingStage      string                 `json:"pricing_stage,omitempty"`
}

type morningMaterializeRunner func(planID uint, opts *strategy.MorningIntentMaterializeOpts) (*strategy.MorningIntentMaterializeResult, error)

// WithMorningMaterializeRunner injects the orchestrator (tests).
func (h *TradePlansHandler) WithMorningMaterializeRunner(runner morningMaterializeRunner) *TradePlansHandler {
	if h == nil {
		h = NewTradePlansHandler()
	}
	h.materializeMorning = runner
	return h
}

func (h *TradePlansHandler) handleMaterializeMorning(w http.ResponseWriter, r *http.Request) {
	var req TradePlanMaterializeMorningRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, TradePlanMaterializeMorningResponse{
			Code:     TradePlanCodeInvalidPlanID,
			Message:  "invalid request body: " + err.Error(),
			Blockers: []ReadinessFindingView{},
		})
		return
	}
	if req.PlanID == 0 {
		writeJSON(w, http.StatusBadRequest, TradePlanMaterializeMorningResponse{
			Code:     TradePlanCodeInvalidPlanID,
			Message:  "plan_id is required",
			Blockers: []ReadinessFindingView{},
		})
		return
	}

	runner := h.materializeMorning
	if runner == nil {
		runner = strategy.RunMorningIntentMaterialize
	}

	opts := &strategy.MorningIntentMaterializeOpts{
		OpenPriceFn: existingRealtimeOpenPriceFn,
	}
	result, err := runner(req.PlanID, opts)
	resp := mapMaterializeMorningResponse(result)
	if err != nil {
		if resp.Code == 0 {
			resp.Code = TradePlanCodeInternalError
		}
		resp.Success = false
		if resp.Message == "" {
			resp.Message = err.Error()
		}
		logger.SugaredLogger.Errorf(
			"MaterializeMorning planId=%d failedStep=%s err=%v",
			req.PlanID, resp.FailedStep, err,
		)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if result != nil && !result.Success && result.FailedStep == "precheck" {
		resp.Code = TradePlanCodeMaterializeDeny
	}
	logger.SugaredLogger.Infof(
		"MaterializeMorning planId=%d success=%t materialized=%d ready=%t failedStep=%s",
		resp.PlanID, resp.Success, resp.MaterializedItems, resp.ReadinessReady, resp.FailedStep,
	)
	writeJSON(w, http.StatusOK, resp)
}

func mapMaterializeMorningResponse(result *strategy.MorningIntentMaterializeResult) TradePlanMaterializeMorningResponse {
	out := TradePlanMaterializeMorningResponse{
		Code:     TradePlanCodeOK,
		Blockers: []ReadinessFindingView{},
	}
	if result == nil {
		out.Code = TradePlanCodeInternalError
		out.Message = "materialize returned nil result"
		return out
	}
	out.Success = result.Success
	out.PlanID = result.PlanID
	out.MaterializedItems = result.MaterializedItems
	out.ReadinessReady = result.ReadinessReady
	out.Message = result.Message
	out.FailedStep = result.FailedStep
	out.PricingStage = result.PricingStage
	out.Blockers = mapReadinessFindings(result.Blockers, "block")
	if out.Blockers == nil {
		out.Blockers = []ReadinessFindingView{}
	}
	return out
}

// existingRealtimeOpenPriceFn reuses papertrading realtime open — no fabricated prices.
func existingRealtimeOpenPriceFn(stockCode string) (float64, bool) {
	code := strings.TrimSpace(stockCode)
	if code == "" {
		return 0, false
	}
	q, ok := papertrading.RealtimeOpenPriceProvider{}.OpenQuote(code, "")
	if !ok || q.Open <= 0 {
		return 0, false
	}
	return q.Open, true
}
