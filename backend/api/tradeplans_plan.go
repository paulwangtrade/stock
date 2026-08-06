package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go-stock/backend/data"

	"gorm.io/gorm"
)

func (h *TradePlansHandler) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(r.URL.Query().Get("plan_id"))
	if rawID == "" {
		writeJSON(w, http.StatusBadRequest, UpcomingTradePlanResponse{
			Code:    TradePlanCodeInvalidPlanID,
			OK:      false,
			Message: "plan_id is required",
		})
		return
	}
	id64, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id64 == 0 {
		writeJSON(w, http.StatusBadRequest, UpcomingTradePlanResponse{
			Code:    TradePlanCodeInvalidPlanID,
			OK:      false,
			Message: "invalid plan_id",
		})
		return
	}
	planID := uint(id64)

	repo := data.NewTradePlanRepo()
	view, err := repo.GetTradePlanVisibilityByID(planID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || isNoTradePlanErr(err) {
			writeJSON(w, http.StatusOK, UpcomingTradePlanResponse{
				Code:    TradePlanCodeNoUpcoming,
				OK:      false,
				PlanID:  planID,
				Message: "no trade plan",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, UpcomingTradePlanResponse{
			Code:    TradePlanCodeInternalError,
			OK:      false,
			PlanID:  planID,
			Message: err.Error(),
		})
		return
	}

	dto := mapUpcomingTradePlanDTO(view)
	enrichUpcomingItemNames(dto, h.nameLookup)
	tradeDate := view.TradeDate
	writeJSON(w, http.StatusOK, UpcomingTradePlanResponse{
		Code:           TradePlanCodeOK,
		OK:             true,
		TradeDate:      tradeDate,
		NextTradingDay: resolveNextTradingDay(tradeDate),
		PlanID:         planID,
		Plan:           dto,
	})
}
