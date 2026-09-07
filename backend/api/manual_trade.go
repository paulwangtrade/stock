package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/execution"
	"go-stock/backend/models"
)

// ManualTradeHandler 模拟盘人工交易 Intent HTTP API（Phase3-PR4-C）。
// 唯一执行出口：ManualTradeFacade → ExecutionService；禁止直连纸面会计层。
type ManualTradeHandler struct {
	facade execution.ManualTradeExecutor
	repo   *data.ManualTradeIntentRepo
}

// NewManualTradeHandler 创建 handler；facade 为 nil 时使用默认 PaperBroker 端口。
func NewManualTradeHandler(facade execution.ManualTradeExecutor) *ManualTradeHandler {
	if facade == nil {
		facade = defaultManualTradeFacade()
	}
	return &ManualTradeHandler{
		facade: facade,
		repo:   data.NewManualTradeIntentRepo(),
	}
}

func defaultManualTradeFacade() execution.ManualTradeExecutor {
	return execution.NewManualTradeFacade(
		execution.NewExecutionService(execution.NewPaperBroker(nil)),
		nil,
	)
}

// ManualTradeAPIEnvelope 统一包装（code + data / error）。
type ManualTradeAPIEnvelope struct {
	Code  int    `json:"code"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// CreateManualTradeIntentRequest POST /api/manual_trade/intent
type CreateManualTradeIntentRequest struct {
	AccountID uint    `json:"account_id"`
	Symbol    string  `json:"symbol"`
	StockName string  `json:"stock_name"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	Reason    string  `json:"reason"`
	OrderKind string  `json:"order_kind"`
}

// ManualTradeIntentIDData 创建/确认 data。
type ManualTradeIntentIDData struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

// ManualTradeIntentIDRequest 确认/执行输入。
type ManualTradeIntentIDRequest struct {
	IntentID uint `json:"intent_id"`
}

// ExecuteManualTradeIntentData 执行结果 data。
type ExecuteManualTradeIntentData struct {
	Status        string `json:"status"`
	OrderID       uint   `json:"order_id,omitempty"`
	ClientOrderID string `json:"client_order_id,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

func writeManualTradeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, ManualTradeAPIEnvelope{Code: 0, Data: data})
}

func writeManualTradeErr(w http.ResponseWriter, httpStatus int, msg string) {
	code := httpStatus
	if code < 400 {
		code = 1
	}
	writeJSON(w, httpStatus, ManualTradeAPIEnvelope{Code: code, Error: msg})
}

// ServeHTTP 路由：
//
//	POST /api/manual_trade/intent
//	POST /api/manual_trade/intent/confirm
//	POST /api/manual_trade/intent/execute
func (h *ManualTradeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewManualTradeHandler(nil)
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if r.Method != http.MethodPost {
		writeManualTradeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	switch path {
	case "/api/manual_trade/intent":
		h.handleCreate(w, r)
	case "/api/manual_trade/intent/confirm":
		h.handleConfirm(w, r)
	case "/api/manual_trade/intent/execute":
		h.handleExecute(w, r)
	default:
		writeManualTradeErr(w, http.StatusNotFound, "not found")
	}
}

func (h *ManualTradeHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateManualTradeIntentRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeManualTradeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Symbol) == "" {
		writeManualTradeErr(w, http.StatusBadRequest, "symbol is required")
		return
	}
	side := strings.ToLower(strings.TrimSpace(req.Side))
	if side == "" {
		side = "buy"
	}
	if side != "buy" && side != "sell" {
		writeManualTradeErr(w, http.StatusBadRequest, "side must be buy or sell")
		return
	}
	orderKind := strings.TrimSpace(req.OrderKind)
	if orderKind == "" {
		orderKind = models.ManualTradeOrderKindNormal
	}

	intent := &models.ManualTradeIntent{
		AccountID:    req.AccountID,
		Symbol:       strings.TrimSpace(req.Symbol),
		StockName:    strings.TrimSpace(req.StockName),
		Side:         side,
		Price:        req.Price,
		Volume:       req.Volume,
		Reason:       strings.TrimSpace(req.Reason),
		OrderKind:    orderKind,
		ManualSource: models.ManualSourcePaperTradingPanel,
	}
	if err := h.repo.CreateManualTradeIntent(intent); err != nil {
		writeManualTradeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeManualTradeOK(w, ManualTradeIntentIDData{ID: intent.ID, Status: intent.Status})
}

func (h *ManualTradeHandler) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var req ManualTradeIntentIDRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeManualTradeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.IntentID == 0 {
		writeManualTradeErr(w, http.StatusBadRequest, "intent_id is required")
		return
	}
	if err := h.repo.UpdateManualTradeIntentStatus(req.IntentID, models.ManualTradeIntentStatusConfirmed, nil); err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, data.ErrInvalidManualTradeIntentTransition) {
			status = http.StatusInternalServerError
		}
		writeManualTradeErr(w, status, err.Error())
		return
	}
	got, err := h.repo.GetManualTradeIntent(req.IntentID)
	if err != nil {
		writeManualTradeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeManualTradeOK(w, ManualTradeIntentIDData{ID: got.ID, Status: got.Status})
}

func (h *ManualTradeHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req ManualTradeIntentIDRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeManualTradeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.IntentID == 0 {
		writeManualTradeErr(w, http.StatusBadRequest, "intent_id is required")
		return
	}

	execErr := h.facade.ConfirmAndExecuteManualTrade(context.Background(), req.IntentID)
	got, getErr := h.repo.GetManualTradeIntent(req.IntentID)
	if getErr != nil {
		writeManualTradeErr(w, http.StatusInternalServerError, getErr.Error())
		return
	}

	if execErr != nil {
		if errors.Is(execErr, execution.ErrManualTradeIntentNotExecutable) ||
			errors.Is(execErr, execution.ErrUnsupportedManualTradeOrderKind) {
			writeManualTradeErr(w, http.StatusBadRequest, execErr.Error())
			return
		}
		writeManualTradeOK(w, ExecuteManualTradeIntentData{
			Status:       got.Status,
			ErrorCode:    got.ErrorCode,
			ErrorMessage: got.ErrorMessage,
		})
		return
	}

	writeManualTradeOK(w, ExecuteManualTradeIntentData{
		Status:        got.Status,
		OrderID:       got.OrderID,
		ClientOrderID: got.ClientOrderID,
	})
}

// RegisterManualTradeRoutes 挂载手工交易路由。
func RegisterManualTradeRoutes(mux *http.ServeMux, facade execution.ManualTradeExecutor) {
	h := NewManualTradeHandler(facade)
	mux.Handle("/api/manual_trade/intent", h)
	mux.Handle("/api/manual_trade/intent/confirm", h)
	mux.Handle("/api/manual_trade/intent/execute", h)
}

// ManualTradeAssetMiddleware 供 Wails AssetServer 挂载 /api/manual_trade/*。
func ManualTradeAssetMiddleware(next http.Handler) http.Handler {
	h := NewManualTradeHandler(nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/manual_trade/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
