package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/execution"
)

// RealOrdersHandler RealStub 只读 HTTP API（不接真券商、不改 Paper）。
type RealOrdersHandler struct{}

// NewRealOrdersHandler 创建 handler。
func NewRealOrdersHandler() *RealOrdersHandler {
	return &RealOrdersHandler{}
}

// RealOrderListResponse GET /api/real/orders
type RealOrderListResponse struct {
	Orders []broker.TradeOrder `json:"orders"`
}

// RealOrderDetailResponse GET /api/real/order/:id
type RealOrderDetailResponse struct {
	Order broker.TradeOrder   `json:"order"`
	Fills []data.RealStubFill `json:"fills"`
}

// ServeHTTP 路由：GET /api/real/orders | GET /api/real/order/{id}
func (h *RealOrdersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewRealOrdersHandler()
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/api/real/orders":
		h.handleList(w, r)
	case strings.HasPrefix(path, "/api/real/order/"):
		id := strings.TrimPrefix(path, "/api/real/order/")
		id = strings.Trim(id, "/")
		h.handleGet(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *RealOrdersHandler) handleList(w http.ResponseWriter, r *http.Request) {
	orders, err := execution.ListRealStubOrders()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if orders == nil {
		orders = []broker.TradeOrder{}
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	if statusFilter != "" {
		filtered := make([]broker.TradeOrder, 0, len(orders))
		for _, o := range orders {
			if o.Status == statusFilter || o.BrokerStatus == statusFilter {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}
	writeJSON(w, http.StatusOK, RealOrderListResponse{Orders: orders})
}

func (h *RealOrdersHandler) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	_ = r
	if strings.TrimSpace(id) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty order id"})
		return
	}
	detail, err := execution.GetRealStubOrderDetail(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	fills := detail.Fills
	if fills == nil {
		fills = []data.RealStubFill{}
	}
	writeJSON(w, http.StatusOK, RealOrderDetailResponse{Order: detail.Order, Fills: fills})
}

// RegisterRealOrderRoutes 将 RealStub 路由挂到 mux。
func RegisterRealOrderRoutes(mux *http.ServeMux) {
	h := NewRealOrdersHandler()
	mux.Handle("/api/real/orders", h)
	mux.Handle("/api/real/order/", h)
}

// RealOrdersAssetMiddleware 供 Wails AssetServer 挂载 /api/real/*。
func RealOrdersAssetMiddleware(next http.Handler) http.Handler {
	h := NewRealOrdersHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/real/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
