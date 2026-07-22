package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/data"
)

// Trade plan visibility API business codes (envelope code field).
const (
	TradePlanCodeOK            = 0
	TradePlanCodeBadTradeDate  = 40001
	TradePlanCodeNoUpcoming    = 40401
	TradePlanCodeInternalError = 50000
)

// UpcomingTradePlanResponse GET /api/tradeplans/upcoming envelope.
type UpcomingTradePlanResponse struct {
	Code      int                   `json:"code"`
	OK        bool                  `json:"ok"`
	TradeDate string                `json:"trade_date"`
	Plan      *UpcomingTradePlanDTO `json:"plan"`
	Message   string                `json:"message,omitempty"`
}

// UpcomingTradePlanDTO HTTP 只读计划视图（snake_case；不暴露 models/data 内部 DTO）。
type UpcomingTradePlanDTO struct {
	ID            uint                       `json:"id"`
	TradeDate     string                     `json:"trade_date"`
	PlanVersion   int                        `json:"plan_version"`
	Status        string                     `json:"status"`
	SourceSession string                     `json:"source_session"`
	PoolID        uint                       `json:"pool_id"`
	Risk          UpcomingTradePlanRiskDTO   `json:"risk"`
	Freeze        UpcomingTradePlanFreezeDTO `json:"freeze"`
	Items         []UpcomingTradePlanItemDTO `json:"items"`
}

// UpcomingTradePlanRiskDTO 持久化 Risk 只读摘要。
type UpcomingTradePlanRiskDTO struct {
	Passed  bool     `json:"passed"`
	Reasons []string `json:"reasons"`
}

// UpcomingTradePlanFreezeDTO 审批/冻结审计只读摘要。
type UpcomingTradePlanFreezeDTO struct {
	IsFrozen     bool   `json:"is_frozen"`
	FreezeAt     string `json:"freeze_at,omitempty"`
	FreezeBy     string `json:"freeze_by,omitempty"`
	FreezeReason string `json:"freeze_reason,omitempty"`
	ApprovedAt   string `json:"approved_at,omitempty"`
	ApprovedBy   string `json:"approved_by,omitempty"`
}

// UpcomingTradePlanItemDTO 计划明细。
type UpcomingTradePlanItemDTO struct {
	StockCode    string  `json:"stock_code"`
	StockName    string  `json:"stock_name"`
	Side         string  `json:"side"`
	Priority     int     `json:"priority"`
	TargetAmount float64 `json:"target_amount"`
	Status       string  `json:"status"`
	Score        float64 `json:"score"`
	RiskCode     string  `json:"risk_code,omitempty"`
	RiskMessage  string  `json:"risk_message,omitempty"`
	StrategyName string  `json:"strategy_name,omitempty"`
}

type upcomingTradePlanQuerier interface {
	GetUpcomingTradePlan(today string) (*data.TradePlanVisibilityView, error)
}

// TradePlansHandler TradePlan 只读 HTTP API（Phase6.5.2）。
type TradePlansHandler struct {
	repo upcomingTradePlanQuerier
}

// NewTradePlansHandler 创建 handler。
func NewTradePlansHandler() *TradePlansHandler {
	return &TradePlansHandler{repo: data.NewTradePlanRepo()}
}

// ServeHTTP GET /api/tradeplans/upcoming
func (h *TradePlansHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewTradePlansHandler()
	}
	if h.repo == nil {
		h.repo = data.NewTradePlanRepo()
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path != "/api/tradeplans/upcoming" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	h.handleUpcoming(w, r)
}

func (h *TradePlansHandler) handleUpcoming(w http.ResponseWriter, r *http.Request) {
	tradeDate, err := resolveUpcomingTradeDate(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, UpcomingTradePlanResponse{
			Code:      TradePlanCodeBadTradeDate,
			OK:        false,
			TradeDate: tradeDate,
			Message:   err.Error(),
		})
		return
	}

	view, err := h.repo.GetUpcomingTradePlan(tradeDate)
	if err != nil {
		if data.IsNoUpcomingTradePlan(err) {
			writeJSON(w, http.StatusOK, UpcomingTradePlanResponse{
				Code:      TradePlanCodeNoUpcoming,
				OK:        false,
				TradeDate: tradeDate,
				Plan:      nil,
				Message:   "no upcoming trade plan",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, UpcomingTradePlanResponse{
			Code:      TradePlanCodeInternalError,
			OK:        false,
			TradeDate: tradeDate,
			Message:   err.Error(),
		})
		return
	}

	plan := mapUpcomingTradePlanDTO(view)
	writeJSON(w, http.StatusOK, UpcomingTradePlanResponse{
		Code:      TradePlanCodeOK,
		OK:        true,
		TradeDate: tradeDate,
		Plan:      plan,
	})
}

func resolveUpcomingTradeDate(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("trade_date"))
	if raw == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return raw, err
	}
	return raw, nil
}

func mapUpcomingTradePlanDTO(view *data.TradePlanVisibilityView) *UpcomingTradePlanDTO {
	if view == nil {
		return nil
	}
	reasons := view.RiskReasons
	if reasons == nil {
		reasons = []string{}
	}
	items := make([]UpcomingTradePlanItemDTO, 0, len(view.Items))
	for _, it := range view.Items {
		items = append(items, UpcomingTradePlanItemDTO{
			StockCode:    it.StockCode,
			StockName:    it.StockName,
			Side:         it.Side,
			Priority:     it.Priority,
			TargetAmount: it.TargetAmount,
			Status:       it.Status,
			Score:        it.Score,
			RiskCode:     it.RiskCode,
			RiskMessage:  it.RiskMessage,
			StrategyName: it.StrategyName,
		})
	}
	return &UpcomingTradePlanDTO{
		ID:            view.ID,
		TradeDate:     view.TradeDate,
		PlanVersion:   view.PlanVersion,
		Status:        view.Status,
		SourceSession: view.SourceSession,
		PoolID:        view.PoolID,
		Risk: UpcomingTradePlanRiskDTO{
			Passed:  view.RiskPassed,
			Reasons: reasons,
		},
		Freeze: UpcomingTradePlanFreezeDTO{
			IsFrozen:     view.IsFrozen,
			FreezeAt:     formatRFC3339Ptr(view.FreezeAt),
			FreezeBy:     view.FreezeBy,
			FreezeReason: view.FreezeReason,
			ApprovedAt:   formatRFC3339Ptr(view.ApprovedAt),
			ApprovedBy:   view.ApprovedBy,
		},
		Items: items,
	}
}

func formatRFC3339Ptr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// RegisterTradePlansRoutes 挂载 TradePlan 只读路由（供 httptest）。
func RegisterTradePlansRoutes(mux *http.ServeMux) {
	mux.Handle("/api/tradeplans/upcoming", NewTradePlansHandler())
}

// TradePlansAssetMiddleware 供 Wails AssetServer 挂载 /api/tradeplans/*。
func TradePlansAssetMiddleware(next http.Handler) http.Handler {
	h := NewTradePlansHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/tradeplans/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
