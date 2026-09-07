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

// ResearchTradeHandler 研究交易 Intent HTTP API（Phase3-PR3-A）。
// 唯一执行出口：ResearchTradeFacade → ExecutionService；禁止直连纸面会计层。
type ResearchTradeHandler struct {
	facade execution.ResearchTradeExecutor
	repo   *data.ResearchTradeIntentRepo
}

// NewResearchTradeHandler 创建 handler；facade 为 nil 时使用默认 PaperBroker 端口。
func NewResearchTradeHandler(facade execution.ResearchTradeExecutor) *ResearchTradeHandler {
	if facade == nil {
		facade = defaultResearchTradeFacade()
	}
	return &ResearchTradeHandler{
		facade: facade,
		repo:   data.NewResearchTradeIntentRepo(),
	}
}

func defaultResearchTradeFacade() execution.ResearchTradeExecutor {
	return execution.NewResearchTradeFacade(
		execution.NewExecutionService(execution.NewPaperBroker(nil)),
		nil,
	)
}

// CreateResearchTradeIntentRequest POST /api/research_trade/intent
type CreateResearchTradeIntentRequest struct {
	Symbol              string  `json:"symbol"`
	StockName           string  `json:"stock_name"`
	Price               float64 `json:"price"`
	Volume              int64   `json:"volume"`
	AccountID           uint    `json:"account_id"`
	CandidateSnapshotID uint    `json:"candidate_snapshot_id"`
	SignalScore         float64 `json:"signal_score"`
	SignalTag           string  `json:"signal_tag"`
	Reason              string  `json:"reason"`
}

// ResearchTradeIntentIDResponse 创建/确认返回。
type ResearchTradeIntentIDResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

// ResearchTradeIntentIDRequest 确认/执行输入。
type ResearchTradeIntentIDRequest struct {
	IntentID uint `json:"intent_id"`
}

// ExecuteResearchTradeIntentResponse 执行结果（成功 submitted / 失败 rejected）。
type ExecuteResearchTradeIntentResponse struct {
	Status        string `json:"status"`
	OrderID       uint   `json:"order_id,omitempty"`
	ClientOrderID string `json:"client_order_id,omitempty"`
	ErrorCode     string `json:"error_code,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// ServeHTTP 路由：
//
//	POST /api/research_trade/intent
//	POST /api/research_trade/intent/confirm
//	POST /api/research_trade/intent/execute
func (h *ResearchTradeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewResearchTradeHandler(nil)
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	switch path {
	case "/api/research_trade/intent":
		h.handleCreate(w, r)
	case "/api/research_trade/intent/confirm":
		h.handleConfirm(w, r)
	case "/api/research_trade/intent/execute":
		h.handleExecute(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *ResearchTradeHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req CreateResearchTradeIntentRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Symbol) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "symbol is required"})
		return
	}
	intent := &models.ResearchTradeIntent{
		Symbol:              strings.TrimSpace(req.Symbol),
		StockName:           strings.TrimSpace(req.StockName),
		Side:                "buy",
		Price:               req.Price,
		Volume:              req.Volume,
		AccountID:           req.AccountID,
		CandidateSnapshotID: req.CandidateSnapshotID,
		SignalScore:         req.SignalScore,
		SignalTag:           strings.TrimSpace(req.SignalTag),
		Reason:              strings.TrimSpace(req.Reason),
	}
	if err := h.repo.CreateResearchTradeIntent(intent); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ResearchTradeIntentIDResponse{ID: intent.ID, Status: intent.Status})
}

func (h *ResearchTradeHandler) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var req ResearchTradeIntentIDRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.IntentID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "intent_id is required"})
		return
	}
	if err := h.repo.UpdateResearchTradeIntentStatus(req.IntentID, models.ResearchTradeIntentStatusConfirmed, nil); err != nil {
		status := http.StatusBadRequest
		if !errors.Is(err, data.ErrInvalidResearchTradeIntentTransition) {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	got, err := h.repo.GetResearchTradeIntent(req.IntentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ResearchTradeIntentIDResponse{ID: got.ID, Status: got.Status})
}

func (h *ResearchTradeHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req ResearchTradeIntentIDRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.IntentID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "intent_id is required"})
		return
	}

	execErr := h.facade.ConfirmAndExecuteResearchBuy(context.Background(), req.IntentID)
	got, getErr := h.repo.GetResearchTradeIntent(req.IntentID)
	if getErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": getErr.Error()})
		return
	}

	if execErr != nil {
		if errors.Is(execErr, execution.ErrResearchTradeIntentNotExecutable) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": execErr.Error()})
			return
		}
		// 业务失败：Intent 已标 rejected，返回执行结果体
		writeJSON(w, http.StatusOK, ExecuteResearchTradeIntentResponse{
			Status:       got.Status,
			ErrorCode:    got.ErrorCode,
			ErrorMessage: got.ErrorMessage,
		})
		return
	}

	writeJSON(w, http.StatusOK, ExecuteResearchTradeIntentResponse{
		Status:        got.Status,
		OrderID:       got.OrderID,
		ClientOrderID: got.ClientOrderID,
	})
}

// RegisterResearchTradeRoutes 挂载研究交易路由。
func RegisterResearchTradeRoutes(mux *http.ServeMux, facade execution.ResearchTradeExecutor) {
	h := NewResearchTradeHandler(facade)
	mux.Handle("/api/research_trade/intent", h)
	mux.Handle("/api/research_trade/intent/confirm", h)
	mux.Handle("/api/research_trade/intent/execute", h)
}

// ResearchTradeAssetMiddleware 供 Wails AssetServer 挂载 /api/research_trade/*。
func ResearchTradeAssetMiddleware(next http.Handler) http.Handler {
	h := NewResearchTradeHandler(nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/research_trade/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
