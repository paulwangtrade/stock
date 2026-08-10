package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"

	"gorm.io/gorm"
)

// TradePlanCodeNoPlan is the readiness/upcoming empty-plan business code (alias of 40401).
const TradePlanCodeNoPlan = TradePlanCodeNoUpcoming

// TradePlanReadinessResponse GET /api/tradeplans/readiness envelope.
type TradePlanReadinessResponse struct {
	Code       int                           `json:"code"`
	OK         bool                          `json:"ok"`
	TradeDate  string                        `json:"trade_date"`
	PlanID     uint                          `json:"plan_id,omitempty"`
	Readiness  *ExecutionIntentReadinessView `json:"readiness"`
	Message    string                        `json:"message,omitempty"`
}

// ExecutionIntentReadinessView HTTP snake_case readiness DTO (Phase6.5.6.14.2.1).
type ExecutionIntentReadinessView struct {
	PlanID         uint                   `json:"plan_id"`
	TradeDate      string                 `json:"trade_date"`
	LifecycleStage string                 `json:"lifecycle_stage"`
	Ready          bool                   `json:"ready"`
	Blockers       []ReadinessFindingView `json:"blockers"`
	Warnings       []ReadinessFindingView `json:"warnings"`
}

// ReadinessFindingView is one blocker/warning line.
type ReadinessFindingView struct {
	RuleCode string         `json:"rule_code"`
	Code     string         `json:"code"`
	Severity string         `json:"severity"` // block | warn
	Message  string         `json:"message"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

// tradePlanFullLoader loads complete TradePlan rows including Intent columns/items.
// Must not be satisfied by Upcoming VisibilityView.
type tradePlanFullLoader interface {
	GetByID(id uint) (*models.TradePlan, error)
	GetLatestByTradeDate(tradeDate string) (*models.TradePlan, error)
}

func (h *TradePlansHandler) fullLoader() tradePlanFullLoader {
	if h != nil {
		if l, ok := h.repo.(tradePlanFullLoader); ok {
			return l
		}
	}
	return data.NewTradePlanRepo()
}

func (h *TradePlansHandler) handleReadiness(w http.ResponseWriter, r *http.Request) {
	loader := h.fullLoader()

	planID, tradeDate, err := resolveReadinessQuery(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, TradePlanReadinessResponse{
			Code:      TradePlanCodeBadTradeDate,
			OK:        false,
			TradeDate: tradeDate,
			Message:   err.Error(),
		})
		return
	}

	plan, err := loadFullTradePlanForReadiness(loader, planID, tradeDate)
	if err != nil {
		if isNoTradePlanErr(err) {
			writeJSON(w, http.StatusOK, TradePlanReadinessResponse{
				Code:      TradePlanCodeNoPlan,
				OK:        false,
				TradeDate: tradeDate,
				PlanID:    planID,
				Readiness: nil,
				Message:   "no trade plan",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, TradePlanReadinessResponse{
			Code:      TradePlanCodeInternalError,
			OK:        false,
			TradeDate: tradeDate,
			PlanID:    planID,
			Message:   err.Error(),
		})
		return
	}

	view := evaluateAndMapReadinessView(plan)
	writeJSON(w, http.StatusOK, TradePlanReadinessResponse{
		Code:      TradePlanCodeOK,
		OK:        true,
		TradeDate: plan.TradeDate,
		PlanID:    plan.ID,
		Readiness: view,
	})
}

func resolveReadinessQuery(r *http.Request) (planID uint, tradeDate string, err error) {
	q := r.URL.Query()
	rawID := strings.TrimSpace(q.Get("plan_id"))
	if rawID != "" {
		id64, perr := strconv.ParseUint(rawID, 10, 64)
		if perr != nil || id64 == 0 {
			return 0, strings.TrimSpace(q.Get("trade_date")), errors.New("invalid plan_id")
		}
		planID = uint(id64)
	}
	tradeDate, err = resolveUpcomingTradeDate(r)
	if err != nil {
		return planID, tradeDate, err
	}
	return planID, tradeDate, nil
}

func loadFullTradePlanForReadiness(loader tradePlanFullLoader, planID uint, tradeDate string) (*models.TradePlan, error) {
	if loader == nil {
		loader = data.NewTradePlanRepo()
	}
	if planID != 0 {
		return loader.GetByID(planID)
	}
	return loader.GetLatestByTradeDate(tradeDate)
}

func isNoTradePlanErr(err error) bool {
	if err == nil {
		return false
	}
	if data.IsNoUpcomingTradePlan(err) || data.IsNoReadyTradePlan(err) {
		return true
	}
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// evaluateAndMapReadinessView runs 14.1 evaluator and maps to HTTP DTO.
// ready is forced from blockers length — never from qualitygate.Passed.
func evaluateAndMapReadinessView(plan *models.TradePlan) *ExecutionIntentReadinessView {
	res := readiness.EvaluateExecutionIntentReadiness(plan, &readiness.Options{
		MarketData: qualitygate.MarketDataSnapshot{SkipGapEval: true},
	})
	return mapExecutionIntentReadinessView(plan, res)
}

func mapExecutionIntentReadinessView(plan *models.TradePlan, res readiness.ExecutionIntentReadinessResult) *ExecutionIntentReadinessView {
	blockers := mapReadinessFindings(res.Blockers, "block")
	warnings := mapReadinessFindings(res.Warnings, "warn")
	tradeDate := ""
	planID := res.PlanID
	if plan != nil {
		tradeDate = plan.TradeDate
		planID = plan.ID
	}
	return &ExecutionIntentReadinessView{
		PlanID:         planID,
		TradeDate:      tradeDate,
		LifecycleStage: res.LifecycleStage,
		// Hard rule 14.2.1: ready == len(blockers)==0 (do NOT use QualityGate Passed).
		Ready:    len(blockers) == 0,
		Blockers: blockers,
		Warnings: warnings,
	}
}

func mapReadinessFindings(in []readiness.Finding, severity string) []ReadinessFindingView {
	out := make([]ReadinessFindingView, 0, len(in))
	for _, f := range in {
		sev := severity
		if s := strings.ToLower(strings.TrimSpace(f.Severity)); s == "block" || s == "warn" {
			sev = s
		} else if strings.EqualFold(f.Severity, readiness.SeverityBlock) {
			sev = "block"
		} else if strings.EqualFold(f.Severity, readiness.SeverityWarn) {
			sev = "warn"
		}
		out = append(out, ReadinessFindingView{
			RuleCode: f.RuleCode,
			Code:     f.Code,
			Severity: sev,
			Message:  f.Message,
			Evidence: f.Evidence,
		})
	}
	return out
}
