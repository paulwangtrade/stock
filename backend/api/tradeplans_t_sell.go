package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/strategy"
)

const (
	TSellDraftCodeInsufficientAvailable = "insufficient_available"
	TSellDraftCodeCannotSell            = "cannot_sell"
	TSellDraftCodeInvalidStock          = "invalid_stock"
	TSellDraftCodeInvalidQuantity       = "invalid_quantity"
)

// TSellDraftRequest POST /api/tradeplans/t-sell/draft body.
type TSellDraftRequest struct {
	TradeDate     string `json:"trade_date"`
	StockCode     string `json:"stock_code"`
	Quantity      int64  `json:"quantity"`
	Actor         string `json:"actor"`
	Reason        string `json:"reason"`
	SourceSession string `json:"source_session"`
}

// TSellDraftResponse POST /api/tradeplans/t-sell/draft success envelope.
type TSellDraftResponse struct {
	OK            bool   `json:"ok"`
	PlanID        uint   `json:"plan_id"`
	Status        string `json:"status"`
	Side          string `json:"side"`
	TradeDate     string `json:"trade_date"`
	ItemCount     int    `json:"item_count"`
	SourceSession string `json:"source_session"`
	Message       string `json:"message,omitempty"`
}

// TSellDraftErrorResponse error envelope for t-sell draft.
type TSellDraftErrorResponse struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *TradePlansHandler) handleTSellDraft(w http.ResponseWriter, r *http.Request) {
	var req TSellDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, TSellDraftErrorResponse{
			OK: false, Code: "INVALID_JSON", Message: err.Error(),
		})
		return
	}

	plan, err := strategy.BuildDraftTSellTradePlan(strategy.TSellDraftRequest{
		TradeDate:     req.TradeDate,
		StockCode:     req.StockCode,
		Quantity:      req.Quantity,
		Actor:         req.Actor,
		Reason:        req.Reason,
		SourceSession: req.SourceSession,
	})
	if err != nil {
		status, code := mapTSellDraftError(err)
		logger.SugaredLogger.Infof(
			"TSellDraftAPI reject code=%s symbol=%s qty=%d err=%v",
			code, strings.TrimSpace(req.StockCode), req.Quantity, err,
		)
		writeJSON(w, status, TSellDraftErrorResponse{OK: false, Code: code, Message: err.Error()})
		return
	}

	logger.SugaredLogger.Infof(
		"TSellDraftAPI created plan_id=%d trade_date=%s symbol=%s qty=%d actor=%s",
		plan.ID, plan.TradeDate, req.StockCode, req.Quantity, strings.TrimSpace(req.Actor),
	)
	writeJSON(w, http.StatusCreated, TSellDraftResponse{
		OK:            true,
		PlanID:        plan.ID,
		Status:        models.TradePlanStatusDraft,
		Side:          "sell",
		TradeDate:     plan.TradeDate,
		ItemCount:     len(plan.Items),
		SourceSession: plan.SourceSession,
	})
}

func mapTSellDraftError(err error) (int, string) {
	switch {
	case errors.Is(err, strategy.ErrSellInvalidStock), errors.Is(err, strategy.ErrSellNoPosition):
		return http.StatusNotFound, TSellDraftCodeInvalidStock
	case errors.Is(err, strategy.ErrSellNotSellable):
		return http.StatusConflict, TSellDraftCodeCannotSell
	case errors.Is(err, strategy.ErrSellInsufficientAvailable):
		return http.StatusConflict, TSellDraftCodeInsufficientAvailable
	case errors.Is(err, strategy.ErrSellInvalidQuantity):
		return http.StatusBadRequest, TSellDraftCodeInvalidQuantity
	default:
		return http.StatusBadRequest, "BAD_REQUEST"
	}
}
