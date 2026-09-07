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
	WatchlistDraftCodePlanExists   = "PLAN_EXISTS"
	WatchlistDraftCodeNotWatching  = "NOT_WATCHING"
	WatchlistDraftCodeInvalidStock = "INVALID_STOCK"
	WatchlistDraftCodeMissingFields = "MISSING_FIELDS"
)

// WatchlistDraftRequest POST /api/tradeplans/watchlist-draft body.
type WatchlistDraftRequest struct {
	AccountID     uint   `json:"account_id"`
	TradeDate     string `json:"trade_date"`
	StockCode     string `json:"stock_code"`
	StockName     string `json:"stock_name"`
	OpportunityID string `json:"opportunity_id"`
	ScanBatchKey  string `json:"scan_batch_key"`
	Actor         string `json:"actor"`
}

// WatchlistDraftResponse success envelope.
type WatchlistDraftResponse struct {
	OK            bool   `json:"ok"`
	PlanID        uint   `json:"plan_id"`
	Status        string `json:"status"`
	Side          string `json:"side"`
	TradeDate     string `json:"trade_date"`
	PlanVersion   int    `json:"plan_version"`
	ItemCount     int    `json:"item_count"`
	SourceSession string `json:"source_session"`
	Message       string `json:"message,omitempty"`
}

// WatchlistDraftErrorResponse error envelope.
type WatchlistDraftErrorResponse struct {
	OK               bool   `json:"ok"`
	Code             string `json:"code"`
	Message          string `json:"message"`
	PlanID           uint   `json:"plan_id,omitempty"`
	TradePlanStatus  string `json:"trade_plan_status,omitempty"`
	TradeDate        string `json:"trade_date,omitempty"`
}

func (h *TradePlansHandler) handleWatchlistDraft(w http.ResponseWriter, r *http.Request) {
	var req WatchlistDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, WatchlistDraftErrorResponse{
			OK: false, Code: "INVALID_JSON", Message: err.Error(),
		})
		return
	}

	plan, err := strategy.BuildDraftWatchlistTradePlan(strategy.WatchlistDraftRequest{
		AccountID:     req.AccountID,
		TradeDate:     req.TradeDate,
		StockCode:     req.StockCode,
		StockName:     req.StockName,
		OpportunityID: req.OpportunityID,
		ScanBatchKey:  req.ScanBatchKey,
		Actor:         req.Actor,
	})
	if err != nil {
		status, code, conflict := mapWatchlistDraftError(err)
		logger.SugaredLogger.Infof(
			"WatchlistDraftAPI reject code=%s symbol=%s err=%v",
			code, strings.TrimSpace(req.StockCode), err,
		)
		body := WatchlistDraftErrorResponse{OK: false, Code: code, Message: err.Error()}
		if conflict != nil {
			body.PlanID = conflict.PlanID
			body.TradePlanStatus = conflict.TradePlanStatus
			body.TradeDate = conflict.TradeDate
			body.Message = "该股票已有交易计划，禁止重复创建覆盖"
		}
		writeJSON(w, status, body)
		return
	}

	logger.SugaredLogger.Infof(
		"WatchlistDraftAPI created plan_id=%d trade_date=%s symbol=%s actor=%s",
		plan.ID, plan.TradeDate, req.StockCode, strings.TrimSpace(req.Actor),
	)
	writeJSON(w, http.StatusCreated, WatchlistDraftResponse{
		OK:            true,
		PlanID:        plan.ID,
		Status:        models.TradePlanStatusDraft,
		Side:          "buy",
		TradeDate:     plan.TradeDate,
		PlanVersion:   plan.PlanVersion,
		ItemCount:     len(plan.Items),
		SourceSession: plan.SourceSession,
	})
}

func mapWatchlistDraftError(err error) (int, string, *strategy.WatchlistDraftConflict) {
	var conflict *strategy.WatchlistDraftConflict
	if errors.As(err, &conflict) {
		return http.StatusConflict, WatchlistDraftCodePlanExists, conflict
	}
	switch {
	case errors.Is(err, strategy.ErrWatchlistDraftPlanExists):
		return http.StatusConflict, WatchlistDraftCodePlanExists, nil
	case errors.Is(err, strategy.ErrWatchlistDraftNotWatching):
		return http.StatusConflict, WatchlistDraftCodeNotWatching, nil
	case errors.Is(err, strategy.ErrWatchlistDraftInvalidStock):
		return http.StatusBadRequest, WatchlistDraftCodeInvalidStock, nil
	case errors.Is(err, strategy.ErrWatchlistDraftMissingFields):
		return http.StatusBadRequest, WatchlistDraftCodeMissingFields, nil
	default:
		return http.StatusBadRequest, "BAD_REQUEST", nil
	}
}
