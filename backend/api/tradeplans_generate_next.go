package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingcalendar"
)

const manualGenerateTrigger = "manual"

// TradePlanGenerateNextRequest is the manual after-close workflow request.
// Provide at most one of SourceDate / TradeDate; empty both keeps default today→next trading day.
type TradePlanGenerateNextRequest struct {
	Actor      string `json:"actor"`
	SourceDate string `json:"source_date,omitempty"`
	TradeDate  string `json:"trade_date,omitempty"`
}

// TradePlanGenerateNextResponse exposes the workflow result without internal models.
type TradePlanGenerateNextResponse struct {
	Code            int    `json:"code"`
	OK              bool   `json:"ok"`
	Trigger         string `json:"trigger"`
	Actor           string `json:"actor"`
	SourceDate      string `json:"source_date,omitempty"`
	TradeDate       string `json:"trade_date,omitempty"`
	CandidatePoolID uint   `json:"candidate_pool_id,omitempty"`
	PlanID          uint   `json:"plan_id,omitempty"`
	PlanVersion     int    `json:"plan_version,omitempty"`
	RiskPassed      bool   `json:"risk_passed"`
	FailedStep      string `json:"failed_step,omitempty"`
	Message         string `json:"message,omitempty"`
}

func (h *TradePlansHandler) handleGenerateNext(w http.ResponseWriter, r *http.Request) {
	var req TradePlanGenerateNextRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
			Code: TradePlanCodeBadActor, Trigger: manualGenerateTrigger,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	req.Actor = strings.TrimSpace(req.Actor)
	req.SourceDate = strings.TrimSpace(req.SourceDate)
	req.TradeDate = strings.TrimSpace(req.TradeDate)
	if req.Actor == "" {
		writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
			Code: TradePlanCodeBadActor, Trigger: manualGenerateTrigger,
			Message: "actor is required",
		})
		return
	}
	if req.SourceDate != "" && req.TradeDate != "" {
		writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
			Code: TradePlanCodeBadTradeDate, Trigger: manualGenerateTrigger,
			Actor: req.Actor, SourceDate: req.SourceDate, TradeDate: req.TradeDate,
			Message: "source_date and trade_date cannot both be specified",
		})
		return
	}
	if req.SourceDate != "" {
		if _, err := time.Parse("2006-01-02", req.SourceDate); err != nil {
			writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
				Code: TradePlanCodeBadTradeDate, Trigger: manualGenerateTrigger,
				Actor: req.Actor, SourceDate: req.SourceDate, Message: err.Error(),
			})
			return
		}
	}

	sourceDate := req.SourceDate
	if req.TradeDate != "" {
		if _, err := time.Parse("2006-01-02", req.TradeDate); err != nil {
			writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
				Code: TradePlanCodeBadTradeDate, Trigger: manualGenerateTrigger,
				Actor: req.Actor, TradeDate: req.TradeDate, Message: err.Error(),
			})
			return
		}
		day, err := tradingcalendar.ParseDate(req.TradeDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
				Code: TradePlanCodeBadTradeDate, Trigger: manualGenerateTrigger,
				Actor: req.Actor, TradeDate: req.TradeDate, Message: err.Error(),
			})
			return
		}
		if !tradingcalendar.IsTradingDay(day) {
			writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
				Code: TradePlanCodeBadTradeDate, Trigger: manualGenerateTrigger,
				Actor: req.Actor, TradeDate: req.TradeDate,
				Message: "trade_date must be a trading day",
			})
			return
		}
		prev, err := tradingcalendar.PrevTradingDayString(req.TradeDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, TradePlanGenerateNextResponse{
				Code: TradePlanCodeBadTradeDate, Trigger: manualGenerateTrigger,
				Actor: req.Actor, TradeDate: req.TradeDate, Message: err.Error(),
			})
			return
		}
		sourceDate = prev
	}

	logger.SugaredLogger.Infof(
		"GenerateNextTradePlan trigger=%s actor=%s sourceDate=%s tradeDateReq=%s",
		manualGenerateTrigger, req.Actor, sourceDate, req.TradeDate,
	)
	runner := h.generateNext
	if runner == nil {
		runner = strategy.RunAfterClosePlanWorkflow
	}
	result, err := runner(sourceDate)
	response := mapGenerateNextResponse(req, result)
	if err != nil {
		response.Code = TradePlanCodeInternalError
		response.OK = false
		if response.Message == "" {
			response.Message = err.Error()
		}
		logger.SugaredLogger.Errorf(
			"GenerateNextTradePlan trigger=%s actor=%s sourceDate=%s tradeDate=%s planId=%d version=%d riskPassed=%t failedStep=%s error=%v",
			manualGenerateTrigger, req.Actor, response.SourceDate, response.TradeDate,
			response.PlanID, response.PlanVersion, response.RiskPassed, response.FailedStep, err,
		)
		writeJSON(w, http.StatusOK, response)
		return
	}

	logger.SugaredLogger.Infof(
		"GenerateNextTradePlan trigger=%s actor=%s sourceDate=%s tradeDate=%s planId=%d version=%d riskPassed=%t failedStep=%s",
		manualGenerateTrigger, req.Actor, response.SourceDate, response.TradeDate,
		response.PlanID, response.PlanVersion, response.RiskPassed, response.FailedStep,
	)
	writeJSON(w, http.StatusOK, response)
}

func mapGenerateNextResponse(
	req TradePlanGenerateNextRequest,
	result *strategy.AfterCloseWorkflowResult,
) TradePlanGenerateNextResponse {
	out := TradePlanGenerateNextResponse{
		Code: TradePlanCodeOK, Trigger: manualGenerateTrigger,
		Actor: req.Actor, SourceDate: req.SourceDate,
	}
	if result == nil {
		out.Code = TradePlanCodeInternalError
		out.Message = "after-close workflow returned nil result"
		return out
	}
	out.OK = result.OK
	out.SourceDate = result.SourceDate
	out.TradeDate = result.TradeDate
	out.CandidatePoolID = result.CandidatePoolID
	out.PlanID = result.TradePlanID
	out.PlanVersion = result.PlanVersion
	out.RiskPassed = result.RiskPassed
	out.FailedStep = result.FailedStep
	out.Message = result.Message
	return out
}
