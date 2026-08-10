package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go-stock/backend/approvegate"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"gorm.io/gorm"
)

const (
	freezeResultFrozen        = "FROZEN"
	freezeResultAlreadyFrozen = "ALREADY_FROZEN"
	freezeResultNotApproved   = "NOT_APPROVED"
	freezeResultNotDraft      = "NOT_DRAFT"
	freezeResultCASMiss       = "CAS_MISS"
	freezeResultInvalidPlanID = "INVALID_PLAN_ID"
	freezeResultDenied        = "DENIED"
	freezeResultPlanNotFound  = "PLAN_NOT_FOUND"
	freezeResultInternal      = "INTERNAL_ERROR"
)

// TradePlanCodeNotApproved is returned when Freeze is attempted before Approve.
const TradePlanCodeNotApproved = 40904

// TradePlanFreezeRequest POST /api/tradeplans/freeze body.
type TradePlanFreezeRequest struct {
	PlanID uint   `json:"plan_id"`
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
	Source string `json:"source"`
}

// TradePlanFreezeResponse POST /api/tradeplans/freeze envelope（对齐 Approve 风格）.
type TradePlanFreezeResponse struct {
	Code           int                    `json:"code"`
	OK             bool                   `json:"ok"`
	ResultCode     string                 `json:"result_code"`
	PlanID         uint                   `json:"plan_id,omitempty"`
	Status         string                 `json:"status,omitempty"`
	IsFrozen       bool                   `json:"is_frozen,omitempty"`
	FreezeAt       string                 `json:"freeze_at,omitempty"`
	FreezeBy       string                 `json:"freeze_by,omitempty"`
	FreezeReason   string                 `json:"freeze_reason,omitempty"`
	FreezeSource   string                 `json:"freeze_source,omitempty"` // 请求 source 回显；表无列则仅日志/响应
	ApprovedAt     string                 `json:"approved_at,omitempty"`
	ApprovedBy     string                 `json:"approved_by,omitempty"`
	AlreadyFrozen  bool                   `json:"already_frozen,omitempty"`
	RiskPassed     *bool                  `json:"risk_passed,omitempty"`
	ReadinessReady *bool                  `json:"readiness_ready,omitempty"`
	Blockers       []ReadinessFindingView `json:"blockers,omitempty"`
	Message        string                 `json:"message,omitempty"`
}

func (h *TradePlansHandler) handleFreeze(w http.ResponseWriter, r *http.Request) {
	var req TradePlanFreezeRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, TradePlanFreezeResponse{
			Code:       TradePlanCodeInvalidPlanID,
			OK:         false,
			ResultCode: freezeResultInvalidPlanID,
			Message:    "invalid request body: " + err.Error(),
		})
		return
	}

	req.Actor = strings.TrimSpace(req.Actor)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Source = strings.TrimSpace(req.Source)
	if req.Source == "" {
		req.Source = "http_api"
	}
	if req.PlanID == 0 {
		writeJSON(w, http.StatusBadRequest, TradePlanFreezeResponse{
			Code:       TradePlanCodeInvalidPlanID,
			OK:         false,
			ResultCode: freezeResultInvalidPlanID,
			Message:    "plan_id is required",
		})
		return
	}
	if req.Actor == "" {
		writeJSON(w, http.StatusBadRequest, TradePlanFreezeResponse{
			Code:       TradePlanCodeBadActor,
			OK:         false,
			ResultCode: freezeResultInvalidPlanID,
			PlanID:     req.PlanID,
			Message:    "actor is required",
		})
		return
	}

	resp := h.runFreeze(req)
	logger.SugaredLogger.Infof(
		"FreezeTradePlanAPI plan_id=%d action=freeze actor=%s source=%s result=%s ok=%v",
		req.PlanID, req.Actor, req.Source, resp.ResultCode, resp.OK,
	)
	status := http.StatusOK
	if resp.Code == TradePlanCodeInvalidPlanID || resp.Code == TradePlanCodeBadActor {
		status = http.StatusBadRequest
	} else if resp.Code == TradePlanCodeInternalError {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, resp)
}

func (h *TradePlansHandler) runFreeze(req TradePlanFreezeRequest) TradePlanFreezeResponse {
	repo := data.NewTradePlanRepo()
	plan, err := repo.GetByID(req.PlanID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || isNoTradePlanErr(err) {
			return TradePlanFreezeResponse{
				Code:       TradePlanCodeNoPlan,
				OK:         false,
				ResultCode: freezeResultPlanNotFound,
				PlanID:     req.PlanID,
				Message:    "trade plan not found",
			}
		}
		return TradePlanFreezeResponse{
			Code:       TradePlanCodeInternalError,
			OK:         false,
			ResultCode: freezeResultInternal,
			PlanID:     req.PlanID,
			Message:    err.Error(),
		}
	}

	out := baseFreezeResponse(plan, req.Source)

	// Idempotent: already frozen → success without re-write.
	if plan.IsFrozen() {
		out.Code = TradePlanCodeOK
		out.OK = true
		out.ResultCode = freezeResultAlreadyFrozen
		out.AlreadyFrozen = true
		out.Message = "trade plan already frozen"
		return out
	}

	if plan.ApprovedAt == nil || plan.ApprovedAt.IsZero() {
		out.Code = TradePlanCodeNotApproved
		out.OK = false
		out.ResultCode = freezeResultNotApproved
		out.Message = "freeze denied: plan is not approved"
		return out
	}

	if !plan.IsDraft() {
		out.Code = TradePlanCodeNotDraft
		out.OK = false
		out.ResultCode = freezeResultNotDraft
		out.Message = fmt.Sprintf("freeze denied: status=%s, want draft", plan.Status)
		return out
	}

	// Pre-check: Risk / Readiness must not block (API gate; service still re-validates ApprovedAt/CAS).
	elig, eligErr := h.checkFreezeEligibility(req.PlanID)
	if eligErr != nil {
		return TradePlanFreezeResponse{
			Code:       TradePlanCodeInternalError,
			OK:         false,
			ResultCode: freezeResultInternal,
			PlanID:     req.PlanID,
			Message:    eligErr.Error(),
		}
	}
	if elig != nil {
		out.Blockers = mapReadinessFindings(elig.Blockers, "block")
		if elig.RiskResult != nil {
			v := elig.RiskResult.Passed
			out.RiskPassed = &v
		}
		if elig.ReadinessResult != nil {
			v := elig.ReadinessResult.Ready
			out.ReadinessReady = &v
		}
		if len(elig.Blockers) > 0 {
			out.Code = classifyApproveDenyCode(elig.Blockers)
			out.OK = false
			out.ResultCode = freezeResultDenied
			out.Message = "freeze denied by risk/readiness gate"
			return out
		}
	}

	reason := req.Reason
	if reason == "" {
		reason = "ui freeze"
	}

	got, freezeErr := strategy.FreezeTradePlan(plan, req.Actor, reason)
	if freezeErr != nil {
		return mapFreezeError(req, plan, freezeErr)
	}
	out = baseFreezeResponse(got, req.Source)
	out.Code = TradePlanCodeOK
	out.OK = true
	out.ResultCode = freezeResultFrozen
	out.Message = "trade plan frozen"
	return out
}

func (h *TradePlansHandler) checkFreezeEligibility(planID uint) (*approvegate.ApproveEligibilityResult, error) {
	opts := h.approveOpts
	eligOpts := (*approvegate.EligibilityOptions)(nil)
	if opts != nil {
		eligOpts = opts.Eligibility
	}
	if eligOpts == nil {
		eligOpts = &approvegate.EligibilityOptions{
			ReadinessOpts: &readiness.Options{
				MarketData: qualitygate.MarketDataSnapshot{SkipGapEval: true},
			},
		}
	}
	return approvegate.CheckApproveEligibility(planID, eligOpts)
}

func baseFreezeResponse(plan *models.TradePlan, source string) TradePlanFreezeResponse {
	if plan == nil {
		return TradePlanFreezeResponse{FreezeSource: source}
	}
	return TradePlanFreezeResponse{
		PlanID:       plan.ID,
		Status:       plan.Status,
		IsFrozen:     plan.IsFrozen(),
		FreezeAt:     formatRFC3339Ptr(plan.FreezeAt),
		FreezeBy:     plan.FreezeBy,
		FreezeReason: plan.FreezeReason,
		FreezeSource: source,
		ApprovedAt:   formatRFC3339Ptr(plan.ApprovedAt),
		ApprovedBy:   plan.ApprovedBy,
	}
}

func mapFreezeError(req TradePlanFreezeRequest, before *models.TradePlan, err error) TradePlanFreezeResponse {
	msg := err.Error()
	out := baseFreezeResponse(before, req.Source)
	out.OK = false
	out.Message = msg
	out.PlanID = req.PlanID

	switch {
	case strings.Contains(msg, "ApprovedAt is nil") || strings.Contains(msg, "is not approved"):
		out.Code = TradePlanCodeNotApproved
		out.ResultCode = freezeResultNotApproved
	case strings.Contains(msg, "want draft"):
		out.Code = TradePlanCodeNotDraft
		out.ResultCode = freezeResultNotDraft
	case strings.Contains(msg, "CAS miss"):
		out.Code = TradePlanCodeCASMiss
		out.ResultCode = freezeResultCASMiss
	case strings.Contains(msg, "id is required"):
		out.Code = TradePlanCodeInvalidPlanID
		out.ResultCode = freezeResultInvalidPlanID
	default:
		out.Code = TradePlanCodeInternalError
		out.ResultCode = freezeResultInternal
	}
	return out
}
