package api

import (
	"net/http"
	"strings"
	"time"

	"go-stock/backend/approvegate"
	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/morningpreparation"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingautomation"
	"go-stock/backend/tradingcalendar"
	"go-stock/backend/tradingwindow"
)

// Trade plan visibility API business codes (envelope code field).
const (
	TradePlanCodeOK              = 0
	TradePlanCodeBadTradeDate    = 40001
	TradePlanCodeInvalidPlanID   = 40002
	TradePlanCodeBadActor        = 40003
	TradePlanCodeNoUpcoming      = 40401
	TradePlanCodeNotDraft        = 40901
	TradePlanCodeCASMiss         = 40902
	TradePlanCodeAlreadyApproved = 40903
	TradePlanCodeRiskDeny        = 42201
	TradePlanCodeReadinessDeny   = 42202
	TradePlanCodeInternalError   = 50000
)

// UpcomingTradePlanResponse GET /api/tradeplans/upcoming envelope.
type UpcomingTradePlanResponse struct {
	Code           int                   `json:"code"`
	OK             bool                  `json:"ok"`
	TradeDate      string                `json:"trade_date"`
	NextTradingDay string                `json:"next_trading_day,omitempty"`
	PlanID         uint                  `json:"plan_id,omitempty"`
	Plan           *UpcomingTradePlanDTO `json:"plan"`
	Message        string                `json:"message,omitempty"`
}

// UpcomingTradePlanDTO HTTP 只读计划视图（snake_case；不暴露 models/data 内部 DTO）。
type UpcomingTradePlanDTO struct {
	ID            uint                       `json:"id"`
	TradeDate     string                     `json:"trade_date"`
	PlanVersion   int                        `json:"plan_version"`
	Status        string                     `json:"status"`
	SourceSession string                     `json:"source_session"`
	GeneratedAt   string                     `json:"generated_at,omitempty"`
	PoolID        uint                       `json:"pool_id"`
	Risk          UpcomingTradePlanRiskDTO   `json:"risk"`
	Freeze        UpcomingTradePlanFreezeDTO `json:"freeze"`
	Window        UpcomingTradePlanWindowDTO `json:"window"`
	Morning       UpcomingTradePlanMorningDTO `json:"morning"`
	Automation    UpcomingTradePlanAutomationDTO `json:"automation"`
	Items         []UpcomingTradePlanItemDTO `json:"items"`
}

// UpcomingTradePlanAutomationDTO derived trading automation policy snapshot (Phase10-C.8.2).
type UpcomingTradePlanAutomationDTO struct {
	Mode                      string `json:"mode"`
	Materialization           string `json:"materialization"`
	Approval                  string `json:"approval"`
	Freeze                    string `json:"freeze"`
	MaterializationReason     string `json:"materialization_reason,omitempty"`
	ApprovalReason            string `json:"approval_reason,omitempty"`
	FreezeReason              string `json:"freeze_reason,omitempty"`
	MaterializeTime           string `json:"materialize_time,omitempty"`
	ApprovalTime              string `json:"approval_time,omitempty"`
	FreezeTime                string `json:"freeze_time,omitempty"`
	FreezeDeadline            string `json:"freeze_deadline,omitempty"`
}

// UpcomingTradePlanWindowDTO derived open-window observation (does not replace plan status).
type UpcomingTradePlanWindowDTO struct {
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	ReasonLabel     string `json:"reason_label"`
	OpenWindowStart string `json:"open_window_start"`
	OpenWindowEnd   string `json:"open_window_end"`
	FreezeDeadline  string `json:"freeze_deadline"`
}

// UpcomingTradePlanMorningDTO derived morning readiness (Phase10-C.8.1).
type UpcomingTradePlanMorningDTO struct {
	Status                string `json:"status"`
	MaterializationStatus string `json:"materialization_status"`
	FreezeStatus          string `json:"freeze_status"`
	DeadlineStatus        string `json:"deadline_status"`
	Reason                string `json:"reason"`
	ReasonLabel           string `json:"reason_label"`
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

// UpcomingTradePlanItemDTO 计划明细（含 Execution Preview 只读字段；仅增字段不改既有语义）。
type UpcomingTradePlanItemDTO struct {
	ID           uint    `json:"id,omitempty"` // Phase13-D: strategy explanation entry
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
	// Execution Preview（DB 已有；AfterClose 时 limit_price/target_volume 可为 0）
	RefPrice     float64 `json:"ref_price"`
	LimitPrice   float64 `json:"limit_price"`
	TargetVolume int64   `json:"target_volume"`
	EntryRule    string  `json:"entry_rule,omitempty"`
	IntentStatus string  `json:"intent_status,omitempty"`
	Reason       string  `json:"reason,omitempty"`
}

type upcomingTradePlanQuerier interface {
	GetUpcomingTradePlan(today string) (*data.TradePlanVisibilityView, error)
}

// TradePlansHandler TradePlan HTTP API（Upcoming/Readiness 只读 + Approve 最小写入）。
type TradePlansHandler struct {
	repo                    upcomingTradePlanQuerier
	approveOpts             *approvegate.ApproveOptions // optional; tests inject Risk/Readiness
	generateNext            func(string) (*strategy.AfterCloseWorkflowResult, error)
	nameLookup              stockNameLookup // optional; tests inject; nil → defaultStockNameLookup
	materializeMorning      morningMaterializeRunner // optional; tests inject; nil → RunMorningIntentMaterialize
	executionReadinessEval  executionReadinessEvaluator // optional; tests inject; nil → LoadAndEvaluate
	preTradeRiskEval        preTradeRiskEvaluator       // optional; tests inject; nil → pretrade.NewService
}

// NewTradePlansHandler 创建 handler。
func NewTradePlansHandler() *TradePlansHandler {
	return &TradePlansHandler{repo: data.NewTradePlanRepo()}
}

// WithApproveOptions returns the handler with Approve Gate options (tests / diagnostics).
func (h *TradePlansHandler) WithApproveOptions(opts *approvegate.ApproveOptions) *TradePlansHandler {
	if h == nil {
		h = NewTradePlansHandler()
	}
	h.approveOpts = opts
	return h
}

// WithGenerateNextRunner injects the after-close workflow runner for tests.
func (h *TradePlansHandler) WithGenerateNextRunner(
	runner func(string) (*strategy.AfterCloseWorkflowResult, error),
) *TradePlansHandler {
	if h == nil {
		h = NewTradePlansHandler()
	}
	h.generateNext = runner
	return h
}

// WithStockNameLookup injects a read-only stock name resolver for tests.
func (h *TradePlansHandler) WithStockNameLookup(lookup func(codes []string) map[string]string) *TradePlansHandler {
	if h == nil {
		h = NewTradePlansHandler()
	}
	h.nameLookup = lookup
	return h
}

// WithExecutionReadinessEval injects execution-readiness evaluator for tests.
func (h *TradePlansHandler) WithExecutionReadinessEval(eval executionReadinessEvaluator) *TradePlansHandler {
	if h == nil {
		h = NewTradePlansHandler()
	}
	h.executionReadinessEval = eval
	return h
}

// ServeHTTP routes TradePlan endpoints under /api/tradeplans/*.
// GET: upcoming, readiness. POST: approve, freeze, generate-next, materialize-morning. Other methods → 405.
func (h *TradePlansHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		h = NewTradePlansHandler()
	}
	if h.repo == nil {
		h.repo = data.NewTradePlanRepo()
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/api/tradeplans/generate-next":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleGenerateNext(w, r)
	case "/api/tradeplans/approve":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleApprove(w, r)
	case "/api/tradeplans/freeze":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleFreeze(w, r)
	case "/api/tradeplans/materialize-morning":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleMaterializeMorning(w, r)
	case "/api/tradeplans/rescale-cash":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleRescaleCash(w, r)
	case "/api/tradeplans/t-sell/draft":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleTSellDraft(w, r)
	case "/api/tradeplans/watchlist-draft":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleWatchlistDraft(w, r)
	case "/api/tradeplans/upcoming":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleUpcoming(w, r)
	case "/api/tradeplans/same-day-candidates":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleSameDayCandidates(w, r)
	case "/api/tradeplans/readiness":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleReadiness(w, r)
	case "/api/tradeplans/plan":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handlePlanByID(w, r)
	default:
		if planID, ok := parseTradePlanPreTradeRiskPath(path); ok {
			h.handlePreTradeRisk(w, r, planID)
			return
		}
		if planID, ok := parseTradePlanExecutionReadinessPath(path); ok {
			h.handleExecutionReadiness(w, r, planID)
			return
		}
		if planID, ok := parseTradePlanLifecyclePath(path); ok {
			h.handleLifecycle(w, r, planID)
			return
		}
		if planID, ok := parseTradePlanOriginPath(path); ok {
			h.handleOrigin(w, r, planID)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *TradePlansHandler) handleUpcoming(w http.ResponseWriter, r *http.Request) {
	tradeDate, err := resolveUpcomingTradeDate(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, UpcomingTradePlanResponse{
			Code:           TradePlanCodeBadTradeDate,
			OK:             false,
			TradeDate:      tradeDate,
			NextTradingDay: resolveNextTradingDay(tradeDate),
			Message:        err.Error(),
		})
		return
	}

	view, err := h.repo.GetUpcomingTradePlan(tradeDate)
	if err != nil {
		if data.IsNoUpcomingTradePlan(err) {
			writeJSON(w, http.StatusOK, UpcomingTradePlanResponse{
				Code:           TradePlanCodeNoUpcoming,
				OK:             false,
				TradeDate:      tradeDate,
				NextTradingDay: resolveNextTradingDay(tradeDate),
				Plan:           nil,
				Message:        "no upcoming trade plan",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, UpcomingTradePlanResponse{
			Code:           TradePlanCodeInternalError,
			OK:             false,
			TradeDate:      tradeDate,
			NextTradingDay: resolveNextTradingDay(tradeDate),
			Message:        err.Error(),
		})
		return
	}

	plan := mapUpcomingTradePlanDTO(view)
	enrichUpcomingItemNames(plan, h.nameLookup)
	writeJSON(w, http.StatusOK, UpcomingTradePlanResponse{
		Code:           TradePlanCodeOK,
		OK:             true,
		TradeDate:      tradeDate,
		NextTradingDay: resolveNextTradingDay(tradeDate),
		Plan:           plan,
	})
}

// resolveNextTradingDay returns NextTradingDayString(tradeDate); empty on parse/calendar error.
func resolveNextTradingDay(tradeDate string) string {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return ""
	}
	next, err := tradingcalendar.NextTradingDayString(tradeDate)
	if err != nil {
		return ""
	}
	return next
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
			ID:           it.ID,
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
			RefPrice:     it.RefPrice,
			LimitPrice:   it.LimitPrice,
			TargetVolume: it.TargetVolume,
			EntryRule:    it.EntryRule,
			IntentStatus: it.IntentStatus,
			Reason:       it.Reason,
		})
	}
	return &UpcomingTradePlanDTO{
		ID:            view.ID,
		TradeDate:     view.TradeDate,
		PlanVersion:   view.PlanVersion,
		Status:        view.Status,
		SourceSession: view.SourceSession,
		GeneratedAt:   formatRFC3339Ptr(view.GeneratedAt),
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
		Window: UpcomingTradePlanWindowDTO{
			Status:          view.WindowStatus,
			Reason:          view.WindowReason,
			ReasonLabel:     tradingwindow.ReasonLabel(view.WindowReason),
			OpenWindowStart: view.OpenWindowStart,
			OpenWindowEnd:   view.OpenWindowEnd,
			FreezeDeadline:  view.FreezeDeadline,
		},
		Morning: mapMorningReadinessDTO(view),
		Automation: mapAutomationDTO(view),
		Items: items,
	}
}

func mapMorningReadinessDTO(view *data.TradePlanVisibilityView) UpcomingTradePlanMorningDTO {
	if view == nil || view.ID == 0 {
		return UpcomingTradePlanMorningDTO{}
	}
	plan, err := data.NewTradePlanRepo().GetByID(view.ID)
	if err != nil || plan == nil {
		return UpcomingTradePlanMorningDTO{}
	}
	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: plan.TradeDate,
		Now:       time.Now(),
		Plan:      plan,
	})
	return UpcomingTradePlanMorningDTO{
		Status:                obs.Status,
		MaterializationStatus: obs.MaterializationStatus,
		FreezeStatus:          obs.FreezeStatus,
		DeadlineStatus:        obs.DeadlineStatus,
		Reason:                obs.Reason,
		ReasonLabel:           morningpreparation.ReasonLabel(obs.Reason),
	}
}

func mapAutomationDTO(view *data.TradePlanVisibilityView) UpcomingTradePlanAutomationDTO {
	cfg := tradingautomation.GetConfig()
	mat, appr, freeze := tradingautomation.LastSteps()
	var plan *models.TradePlan
	if view != nil && view.ID > 0 {
		if p, err := data.NewTradePlanRepo().GetByID(view.ID); err == nil {
			plan = p
		}
	}
	obs := tradingautomation.EvaluateMorningAutomation("", plan, &mat, &appr, &freeze)
	return UpcomingTradePlanAutomationDTO{
		Mode:                  obs.Mode,
		Materialization:       obs.Materialization,
		Approval:              obs.Approval,
		Freeze:                obs.Freeze,
		MaterializationReason: obs.MaterializationReason,
		ApprovalReason:        obs.ApprovalReason,
		FreezeReason:          obs.FreezeReason,
		MaterializeTime:       cfg.MaterializeTime,
		ApprovalTime:          cfg.ApprovalTime,
		FreezeTime:            cfg.FreezeTime,
		FreezeDeadline:        cfg.FreezeDeadline,
	}
}

func formatRFC3339Ptr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// RegisterTradePlansRoutes 挂载 TradePlan 路由（供 httptest）。
func RegisterTradePlansRoutes(mux *http.ServeMux) {
	RegisterTradePlansHandler(mux, NewTradePlansHandler())
}

// RegisterTradePlansHandler 挂载指定 handler（测试可注入 ApproveOptions）。
func RegisterTradePlansHandler(mux *http.ServeMux, h *TradePlansHandler) {
	if h == nil {
		h = NewTradePlansHandler()
	}
	mux.Handle("/api/tradeplans/upcoming", h)
	mux.Handle("/api/tradeplans/same-day-candidates", h)
	mux.Handle("/api/tradeplans/readiness", h)
	mux.Handle("/api/tradeplans/approve", h)
	mux.Handle("/api/tradeplans/freeze", h)
	mux.Handle("/api/tradeplans/generate-next", h)
	mux.Handle("/api/tradeplans/materialize-morning", h)
	mux.Handle("/api/tradeplans/rescale-cash", h)
	mux.Handle("/api/tradeplans/t-sell/draft", h)
	mux.Handle("/api/tradeplans/watchlist-draft", h)
	mux.Handle("/api/tradeplans/plan", h)
	mux.Handle("/api/tradeplans/", h) // /{id}/lifecycle subtree
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
