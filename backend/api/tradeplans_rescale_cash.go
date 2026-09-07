package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-stock/backend/strategy"
)

// TradePlanRescaleCashRequest POST /api/tradeplans/rescale-cash
type TradePlanRescaleCashRequest struct {
	Actor         string  `json:"actor"`
	PlanID        uint    `json:"plan_id"`
	AvailableCash float64 `json:"available_cash,omitempty"` // optional override; else Snapshot
}

// TradePlanRescaleCashResponse compares source vs new plan_version.
type TradePlanRescaleCashResponse struct {
	Code              int     `json:"code"`
	OK                bool    `json:"ok"`
	Changed           bool    `json:"changed"`
	Mode              string  `json:"mode,omitempty"`
	Actor             string  `json:"actor,omitempty"`
	SourcePlanID      uint    `json:"source_plan_id,omitempty"`
	SourcePlanVersion int     `json:"source_plan_version,omitempty"`
	NewPlanID         uint    `json:"new_plan_id,omitempty"`
	NewPlanVersion    int     `json:"new_plan_version,omitempty"`
	AvailableCash     float64 `json:"available_cash,omitempty"`
	RequiredBefore    float64 `json:"required_before,omitempty"`
	RequiredAfter     float64 `json:"required_after,omitempty"`
	NamesBefore       int     `json:"names_before,omitempty"`
	NamesAfter        int     `json:"names_after,omitempty"`
	AmountPerStockOld float64 `json:"amount_per_stock_old,omitempty"`
	AmountPerStockNew float64 `json:"amount_per_stock_new,omitempty"`
	Message           string  `json:"message,omitempty"`
}

func (h *TradePlansHandler) handleRescaleCash(w http.ResponseWriter, r *http.Request) {
	var req TradePlanRescaleCashRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, TradePlanRescaleCashResponse{
			Code: TradePlanCodeBadActor, Message: "invalid request body: " + err.Error(),
		})
		return
	}
	req.Actor = strings.TrimSpace(req.Actor)
	if req.Actor == "" {
		writeJSON(w, http.StatusBadRequest, TradePlanRescaleCashResponse{
			Code: TradePlanCodeBadActor, Message: "actor is required",
		})
		return
	}
	if req.PlanID == 0 {
		writeJSON(w, http.StatusBadRequest, TradePlanRescaleCashResponse{
			Code: TradePlanCodeBadTradeDate, Actor: req.Actor, Message: "plan_id is required",
		})
		return
	}

	res, err := strategy.RescaleTradePlanForCash(strategy.CashRescaleRequest{
		PlanID:        req.PlanID,
		AvailableCash: req.AvailableCash,
		Actor:         req.Actor,
	})
	if err != nil {
		code := http.StatusBadRequest
		writeJSON(w, code, TradePlanRescaleCashResponse{
			Code: 1, OK: false, Actor: req.Actor,
			SourcePlanID: req.PlanID,
			Message:      err.Error(),
			AvailableCash: func() float64 {
				if res != nil {
					return res.AvailableCash
				}
				return req.AvailableCash
			}(),
			RequiredBefore: func() float64 {
				if res != nil {
					return res.RequiredBefore
				}
				return 0
			}(),
		})
		return
	}

	writeJSON(w, http.StatusOK, TradePlanRescaleCashResponse{
		Code:              0,
		OK:                true,
		Changed:           res.Changed,
		Mode:              res.Mode,
		Actor:             req.Actor,
		SourcePlanID:      res.SourcePlanID,
		SourcePlanVersion: res.SourcePlanVersion,
		NewPlanID:         res.NewPlanID,
		NewPlanVersion:    res.NewPlanVersion,
		AvailableCash:     res.AvailableCash,
		RequiredBefore:    res.RequiredBefore,
		RequiredAfter:     res.RequiredAfter,
		NamesBefore:       res.NamesBefore,
		NamesAfter:        res.NamesAfter,
		AmountPerStockOld: res.AmountPerStockOld,
		AmountPerStockNew: res.AmountPerStockNew,
		Message:           res.Message,
	})
}
