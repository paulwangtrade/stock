package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/data"
)

// Ops trading day status API business codes.
const (
	OpsTradingDayCodeOK            = 0
	OpsTradingDayCodeBadTradeDate  = 40001
	OpsTradingDayCodeInternalError = 50000
)

// TradingDayStatusResponse GET /api/ops/trading_day_status envelope.
type TradingDayStatusResponse struct {
	Code      int                        `json:"code"`
	OK        bool                       `json:"ok"`
	TradeDate string                     `json:"trade_date"`
	Data      *data.TradingDayStatusView `json:"data"`
	Message   string                     `json:"message,omitempty"`
}

type tradingDayStatusBuilder interface {
	BuildTradingDayStatus(tradeDate string) (*data.TradingDayStatusView, error)
}

type defaultTradingDayStatusBuilder struct{}

func (defaultTradingDayStatusBuilder) BuildTradingDayStatus(tradeDate string) (*data.TradingDayStatusView, error) {
	return data.BuildTradingDayStatus(tradeDate)
}

// OpsTradingDayHandler Production Readiness readonly HTTP API (Phase6.5.4.1).
type OpsTradingDayHandler struct {
	builder tradingDayStatusBuilder
}

// NewOpsTradingDayHandler creates handler.
func NewOpsTradingDayHandler() *OpsTradingDayHandler {
	return &OpsTradingDayHandler{builder: defaultTradingDayStatusBuilder{}}
}

// ServeHTTP GET /api/ops/trading_day_status
func (h *OpsTradingDayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewOpsTradingDayHandler()
	}
	if h.builder == nil {
		h.builder = defaultTradingDayStatusBuilder{}
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != "/api/ops/trading_day_status" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	h.handleTradingDayStatus(w, r)
}

func (h *OpsTradingDayHandler) handleTradingDayStatus(w http.ResponseWriter, r *http.Request) {
	tradeDate, err := resolveOpsTradeDate(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, TradingDayStatusResponse{
			Code:      OpsTradingDayCodeBadTradeDate,
			OK:        false,
			TradeDate: tradeDate,
			Message:   err.Error(),
		})
		return
	}

	view, err := h.builder.BuildTradingDayStatus(tradeDate)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, TradingDayStatusResponse{
			Code:      OpsTradingDayCodeInternalError,
			OK:        false,
			TradeDate: tradeDate,
			Message:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, TradingDayStatusResponse{
		Code:      OpsTradingDayCodeOK,
		OK:        true,
		TradeDate: tradeDate,
		Data:      view,
	})
}

func resolveOpsTradeDate(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("trade_date"))
	if raw == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return raw, err
	}
	return raw, nil
}

// RegisterOpsTradingDayRoutes mounts ops readonly routes (httptest).
func RegisterOpsTradingDayRoutes(mux *http.ServeMux) {
	mux.Handle("/api/ops/trading_day_status", NewOpsTradingDayHandler())
}

// OpsTradingDayAssetMiddleware mounts /api/ops/* on Wails AssetServer.
func OpsTradingDayAssetMiddleware(next http.Handler) http.Handler {
	h := NewOpsTradingDayHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/ops/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
