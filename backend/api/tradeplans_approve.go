package api

import (
	"errors"
	"net/http"
	"strings"

	"go-stock/backend/approvegate"
	"go-stock/backend/logger"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"

	"gorm.io/gorm"
)

// TradePlanApproveRequest POST /api/tradeplans/approve body.
type TradePlanApproveRequest struct {
	PlanID uint   `json:"plan_id"`
	Actor  string `json:"actor"`
	Source string `json:"source"`
}

// TradePlanApproveResponse POST /api/tradeplans/approve envelope.
type TradePlanApproveResponse struct {
	Code              int                    `json:"code"`
	OK                bool                   `json:"ok"`
	ResultCode        string                 `json:"result_code"`
	PlanID            uint                   `json:"plan_id,omitempty"`
	Status            string                 `json:"status,omitempty"`
	ApprovedAt        string                 `json:"approved_at,omitempty"`
	ApprovedBy        string                 `json:"approved_by,omitempty"`
	ApprovedSource    string                 `json:"approved_source,omitempty"`
	AlreadyApproved   bool                   `json:"already_approved,omitempty"`
	RiskPassed        *bool                  `json:"risk_passed,omitempty"`
	ReadinessReady    *bool                  `json:"readiness_ready,omitempty"`
	Blockers          []ReadinessFindingView `json:"blockers,omitempty"`
	Message           string                 `json:"message,omitempty"`
}

func (h *TradePlansHandler) handleApprove(w http.ResponseWriter, r *http.Request) {
	var req TradePlanApproveRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, TradePlanApproveResponse{
			Code:       TradePlanCodeInvalidPlanID,
			OK:         false,
			ResultCode: approvegate.CodeInvalidPlanID,
			Message:    "invalid request body: " + err.Error(),
		})
		return
	}
	if req.PlanID == 0 {
		writeJSON(w, http.StatusBadRequest, TradePlanApproveResponse{
			Code:       TradePlanCodeInvalidPlanID,
			OK:         false,
			ResultCode: approvegate.CodeInvalidPlanID,
			Message:    "plan_id is required",
		})
		return
	}

	actor := strings.TrimSpace(req.Actor)
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "http_api"
	}

	res, err := h.runApprove(req.PlanID, actor, source)
	if err != nil {
		if isNoTradePlanErr(err) || errors.Is(err, gorm.ErrRecordNotFound) {
			logger.SugaredLogger.Infof(
				"ApproveTradePlanAPI plan_id=%d action=approve actor=%s source=%s result=PLAN_NOT_FOUND ok=false",
				req.PlanID, actor, source,
			)
			writeJSON(w, http.StatusOK, TradePlanApproveResponse{
				Code:       TradePlanCodeNoPlan,
				OK:         false,
				ResultCode: "PLAN_NOT_FOUND",
				PlanID:     req.PlanID,
				Message:    "trade plan not found",
			})
			return
		}
		if strings.Contains(err.Error(), "plan id is required") {
			logger.SugaredLogger.Infof(
				"ApproveTradePlanAPI plan_id=%d action=approve actor=%s source=%s result=%s ok=false",
				req.PlanID, actor, source, approvegate.CodeInvalidPlanID,
			)
			writeJSON(w, http.StatusBadRequest, TradePlanApproveResponse{
				Code:       TradePlanCodeInvalidPlanID,
				OK:         false,
				ResultCode: approvegate.CodeInvalidPlanID,
				PlanID:     req.PlanID,
				Message:    err.Error(),
			})
			return
		}
		logger.SugaredLogger.Infof(
			"ApproveTradePlanAPI plan_id=%d action=approve actor=%s source=%s result=INTERNAL_ERROR ok=false",
			req.PlanID, actor, source,
		)
		writeJSON(w, http.StatusInternalServerError, TradePlanApproveResponse{
			Code:       TradePlanCodeInternalError,
			OK:         false,
			ResultCode: "INTERNAL_ERROR",
			PlanID:     req.PlanID,
			Message:    err.Error(),
		})
		return
	}

	out := mapApproveWriteResult(req.PlanID, res)
	logger.SugaredLogger.Infof(
		"ApproveTradePlanAPI plan_id=%d action=approve actor=%s source=%s result=%s ok=%v",
		req.PlanID, actor, source, out.ResultCode, out.OK,
	)
	writeJSON(w, http.StatusOK, out)
}

func (h *TradePlansHandler) runApprove(planID uint, actor, source string) (*approvegate.ApproveWriteResult, error) {
	opts := h.approveOpts
	if opts == nil {
		opts = &approvegate.ApproveOptions{
			Eligibility: &approvegate.EligibilityOptions{
				ReadinessOpts: &readiness.Options{
					MarketData: qualitygate.MarketDataSnapshot{SkipGapEval: true},
				},
			},
		}
	}
	return approvegate.ApproveTradePlanByID(planID, actor, source, opts)
}

func mapApproveWriteResult(reqPlanID uint, res *approvegate.ApproveWriteResult) TradePlanApproveResponse {
	if res == nil {
		return TradePlanApproveResponse{
			Code:       TradePlanCodeInternalError,
			OK:         false,
			ResultCode: "INTERNAL_ERROR",
			PlanID:     reqPlanID,
			Message:    "nil approve result",
		}
	}

	out := TradePlanApproveResponse{
		ResultCode:      res.Code,
		AlreadyApproved: res.AlreadyApproved,
		Message:         res.Message,
		Blockers:        mapReadinessFindings(res.Blockers, "block"),
	}
	if res.Eligibility != nil {
		out.PlanID = res.Eligibility.PlanID
		if res.Eligibility.RiskResult != nil {
			v := res.Eligibility.RiskResult.Passed
			out.RiskPassed = &v
		}
		if res.Eligibility.ReadinessResult != nil {
			v := res.Eligibility.ReadinessResult.Ready
			out.ReadinessReady = &v
		}
	}
	if out.PlanID == 0 {
		out.PlanID = reqPlanID
	}
	if res.Plan != nil {
		out.PlanID = res.Plan.ID
		out.Status = res.Plan.Status
		out.ApprovedAt = formatRFC3339Ptr(res.Plan.ApprovedAt)
		out.ApprovedBy = res.Plan.ApprovedBy
		out.ApprovedSource = res.Plan.ApprovedSource
	}

	switch {
	case res.OK:
		out.Code = TradePlanCodeOK
		out.OK = true
		if out.ResultCode == "" {
			out.ResultCode = approvegate.CodeApproveWriteOK
		}
	case res.AlreadyApproved || res.Code == approvegate.CodeAlreadyApproved:
		out.Code = TradePlanCodeAlreadyApproved
		out.OK = false
		out.ResultCode = approvegate.CodeAlreadyApproved
	case res.Code == approvegate.CodeCASMiss:
		out.Code = TradePlanCodeCASMiss
		out.OK = false
	case res.Code == approvegate.CodeInvalidPlanID:
		out.Code = TradePlanCodeInvalidPlanID
		out.OK = false
	default:
		out.Code = classifyApproveDenyCode(res.Blockers)
		out.OK = false
		if out.ResultCode == "" {
			out.ResultCode = approvegate.CodeDenied
		}
	}
	return out
}

func classifyApproveDenyCode(blockers []readiness.Finding) int {
	for _, b := range blockers {
		switch b.Code {
		case approvegate.CodeRiskNotPassed, approvegate.CodeRiskEvalError:
			return TradePlanCodeRiskDeny
		case approvegate.CodeNotDraft:
			return TradePlanCodeNotDraft
		}
	}
	for _, b := range blockers {
		if b.Code == approvegate.CodeNoItems {
			return TradePlanCodeInvalidPlanID
		}
	}
	if len(blockers) > 0 {
		return TradePlanCodeReadinessDeny
	}
	return TradePlanCodeReadinessDeny
}
