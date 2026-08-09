// Package api — Phase13-D product capability UI wiring (FeatureGate + Explain + Risk + Usage).
// Read-only / Shell-only: does not mutate TradePlan generation, Strategy, Execution, Broker, Gateway.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/assistant"
	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/riskreport"
	"go-stock/backend/strategyexplain"
	"go-stock/backend/usagemetrics"
)

const (
	ProductCodeOK            = 0
	ProductCodeBadRequest    = 40001
	ProductCodeMethodNotAllowed = 40500
	ProductCodeInternal      = 50000
)

// ProductCapabilitiesHandler serves /api/product/* for Shell UI wiring.
type ProductCapabilitiesHandler struct{}

func NewProductCapabilitiesHandler() *ProductCapabilitiesHandler {
	return &ProductCapabilitiesHandler{}
}

func isProductCapabilitiesPath(path string) bool {
	switch path {
	case "/api/product/feature-gate",
		"/api/product/strategy-explanation",
		"/api/product/risk-report",
		"/api/product/assistant-context",
		"/api/product/usage",
		"/api/product/usage/summary",
		"/api/product/usage/count",
		"/api/product/usage/analytics",
		"/api/product/plans",
		"/api/product/feature-comparison",
		"/api/product/upgrade-demo":
		return true
	default:
		return false
	}
}

func (h *ProductCapabilitiesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/api/product/feature-gate":
		h.handleFeatureGate(w, r)
	case "/api/product/strategy-explanation":
		h.handleStrategyExplanation(w, r)
	case "/api/product/risk-report":
		h.handleRiskReport(w, r)
	case "/api/product/assistant-context":
		h.handleAssistantContext(w, r)
	case "/api/product/usage":
		h.handleUsage(w, r)
	case "/api/product/usage/summary":
		h.handleUsageSummary(w, r)
	case "/api/product/usage/count":
		h.handleUsageCount(w, r)
	case "/api/product/usage/analytics":
		h.handleUsageAnalytics(w, r)
	case "/api/product/plans":
		h.handlePlans(w, r)
	case "/api/product/feature-comparison":
		h.handleFeatureComparison(w, r)
	case "/api/product/upgrade-demo":
		h.handleUpgradeDemo(w, r)
	default:
		http.NotFound(w, r)
	}
}

// RegisterProductCapabilitiesRoutes mounts product capability routes on mux.
func RegisterProductCapabilitiesRoutes(mux *http.ServeMux) {
	h := NewProductCapabilitiesHandler()
	mux.Handle("/api/product/feature-gate", h)
	mux.Handle("/api/product/strategy-explanation", h)
	mux.Handle("/api/product/risk-report", h)
	mux.Handle("/api/product/assistant-context", h)
	mux.Handle("/api/product/usage", h)
	mux.Handle("/api/product/usage/summary", h)
	mux.Handle("/api/product/usage/count", h)
	mux.Handle("/api/product/usage/analytics", h)
	mux.Handle("/api/product/plans", h)
	mux.Handle("/api/product/feature-comparison", h)
	mux.Handle("/api/product/upgrade-demo", h)
}

// ProductCapabilitiesAssetMiddleware mounts /api/product/* on Wails AssetServer.
func ProductCapabilitiesAssetMiddleware(next http.Handler) http.Handler {
	h := NewProductCapabilitiesHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isProductCapabilitiesPath(strings.TrimSuffix(r.URL.Path, "/")) {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func shellUserFromRequest(r *http.Request) *featuregate.User {
	tier := featuregate.NormalizeTier(featuregate.Tier(strings.TrimSpace(r.URL.Query().Get("tier"))))
	user := &featuregate.User{ID: "shell:" + string(tier), Tier: tier}
	_ = entitlement.Default().EnsureTierDefaults(user)
	return user
}

func (h *ProductCapabilitiesHandler) handleFeatureGate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	user := shellUserFromRequest(r)
	features := featuregate.KnownFeatures()
	out := make([]map[string]any, 0, len(features))
	for _, f := range features {
		d := featuregate.CanAccess(user, f)
		out = append(out, map[string]any{
			"feature": string(d.Feature),
			"allowed": d.Allowed,
			"tier":    string(d.Tier),
			"reason":  string(d.Reason),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"tier":     string(user.Tier),
		"features": out,
	})
}

func (h *ProductCapabilitiesHandler) handleStrategyExplanation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	user := shellUserFromRequest(r)
	q := r.URL.Query()
	planID, _ := strconv.ParseUint(strings.TrimSpace(q.Get("plan_id")), 10, 64)
	planItemID, _ := strconv.ParseUint(strings.TrimSpace(q.Get("plan_item_id")), 10, 64)
	snapshotID := strings.TrimSpace(q.Get("snapshot_id"))
	includeExit := strings.EqualFold(strings.TrimSpace(q.Get("include_exit")), "1") ||
		strings.EqualFold(strings.TrimSpace(q.Get("include_exit")), "true")

	if snapshotID == "" && (planID == 0 || planItemID == 0) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false,
			"message": "snapshot_id or (plan_id + plan_item_id) required",
		})
		return
	}

	_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedObservation, usagemetrics.EventOpened, map[string]string{
		"scene":     "strategy_explanation",
		"usage_key": "strategy_explanation_opened",
		"source":    "http",
	})

	out, err := strategyexplain.Default().Explain(context.Background(), strategyexplain.ExplainRequest{
		User:        user,
		SnapshotID:  snapshotID,
		PlanID:      uint(planID),
		PlanItemID:  uint(planItemID),
		IncludeExit: includeExit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": ProductCodeInternal, "ok": false, "message": err.Error(),
		})
		return
	}
	if out != nil && out.Status != strategyexplain.StatusGated {
		_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedObservation, usagemetrics.EventViewed, map[string]string{
			"scene":     "strategy_explanation",
			"usage_key": "strategy_explanation_viewed",
			"source":    "http",
			"status":    out.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true, "explanation": out,
	})
}

func (h *ProductCapabilitiesHandler) handleRiskReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	user := shellUserFromRequest(r)
	tradeDate := strings.TrimSpace(r.URL.Query().Get("trade_date"))

	_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedRisk, usagemetrics.EventOpened, map[string]string{
		"scene":     "advanced_risk_report",
		"usage_key": "risk_report_opened",
		"source":    "http",
	})

	out, err := riskreport.Default().Build(context.Background(), riskreport.BuildRequest{
		UserID:    user.ID,
		Tier:      string(user.Tier),
		TradeDate: tradeDate,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": ProductCodeInternal, "ok": false, "message": err.Error(),
		})
		return
	}
	if out != nil && out.Status != riskreport.StatusGated {
		_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAdvancedRisk, usagemetrics.EventViewed, map[string]string{
			"scene":     "advanced_risk_report",
			"usage_key": "risk_report_viewed",
			"source":    "http",
			"status":    out.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true, "report": out,
	})
}

func (h *ProductCapabilitiesHandler) handleAssistantContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	user := shellUserFromRequest(r)
	scene := assistant.AssistantScene(strings.TrimSpace(r.URL.Query().Get("scene")))
	if scene == "" {
		scene = assistant.SceneRiskExplain
	}
	tradeDate := strings.TrimSpace(r.URL.Query().Get("trade_date"))

	_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAIAnalysis, usagemetrics.EventOpened, map[string]string{
		"scene":     string(scene),
		"usage_key": "assistant_context_opened",
		"source":    "http",
	})

	// Optional: attach a local risk report fact when building risk_explain (no external AI).
	var rr *riskreport.RiskReport
	if scene == assistant.SceneRiskExplain || scene == assistant.ScenePositionSummary {
		built, err := riskreport.Default().Build(context.Background(), riskreport.BuildRequest{
			UserID:    user.ID,
			Tier:      string(user.Tier),
			TradeDate: tradeDate,
		})
		if err == nil {
			rr = built
		}
	}

	out, err := assistant.Default().BuildContext(context.Background(), assistant.ContextRequest{
		User:       user,
		Scene:      scene,
		RiskReport: rr,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": ProductCodeInternal, "ok": false, "message": err.Error(),
		})
		return
	}
	if out != nil && out.Status != "gated" {
		_, _ = usagemetrics.RecordFeatureEvent(context.Background(), user, featuregate.FeatureAIAnalysis, usagemetrics.EventViewed, map[string]string{
			"scene":     string(scene),
			"usage_key": "assistant_context_viewed",
			"source":    "http",
			"status":    out.Status,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true, "context": out,
	})
}

type productUsageBody struct {
	Tier      string            `json:"tier"`
	Feature   string            `json:"feature"`
	Event     string            `json:"event"`
	UsageKey  string            `json:"usage_key"`
	Scene     string            `json:"scene"`
	Metadata  map[string]string `json:"metadata"`
}

func (h *ProductCapabilitiesHandler) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "POST required",
		})
		return
	}
	var body productUsageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "invalid json",
		})
		return
	}
	tier := featuregate.NormalizeTier(featuregate.Tier(strings.TrimSpace(body.Tier)))
	if strings.TrimSpace(body.Tier) == "" {
		tier = featuregate.NormalizeTier(featuregate.Tier(strings.TrimSpace(r.URL.Query().Get("tier"))))
	}
	user := &featuregate.User{ID: "shell:" + string(tier), Tier: tier}
	_ = entitlement.Default().EnsureTierDefaults(user)

	feat := featuregate.Feature(strings.TrimSpace(body.Feature))
	if !featuregate.IsKnown(feat) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "unknown feature",
		})
		return
	}
	ev := usagemetrics.EventType(strings.TrimSpace(body.Event))
	if ev == "" {
		ev = usagemetrics.EventOpened
	}
	if !usagemetrics.IsKnownEventType(ev) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "unknown event",
		})
		return
	}
	meta := body.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	if strings.TrimSpace(body.Scene) != "" {
		meta["scene"] = strings.TrimSpace(body.Scene)
	}
	if strings.TrimSpace(body.UsageKey) != "" {
		meta["usage_key"] = strings.TrimSpace(body.UsageKey)
	}
	meta["source"] = "http_ui"
	recorded, err := usagemetrics.RecordFeatureEvent(context.Background(), user, feat, ev, meta)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": ProductCodeInternal, "ok": false, "message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true, "event": recorded,
	})
}

func parseUsageTimeParam(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, strconv.ErrSyntax
}

func parseUsageAnalyticsFilter(r *http.Request) (usagemetrics.AnalyticsFilter, error) {
	q := r.URL.Query()
	from, err := parseUsageTimeParam(q.Get("from"))
	if err != nil {
		return usagemetrics.AnalyticsFilter{}, err
	}
	to, err := parseUsageTimeParam(q.Get("to"))
	if err != nil {
		return usagemetrics.AnalyticsFilter{}, err
	}
	feat := featuregate.Feature(strings.TrimSpace(q.Get("feature")))
	ev := usagemetrics.EventType(strings.TrimSpace(q.Get("event_type")))
	if feat != "" && !featuregate.IsKnown(feat) {
		return usagemetrics.AnalyticsFilter{}, strconv.ErrSyntax
	}
	if ev != "" && !usagemetrics.IsKnownEventType(ev) {
		return usagemetrics.AnalyticsFilter{}, strconv.ErrSyntax
	}
	return usagemetrics.AnalyticsFilter{
		UserID:    strings.TrimSpace(q.Get("user_id")),
		Feature:   feat,
		EventType: ev,
		From:      from,
		To:        to,
	}, nil
}

func resolveUsageUserID(r *http.Request, filter usagemetrics.AnalyticsFilter) string {
	if strings.TrimSpace(filter.UserID) != "" {
		return strings.TrimSpace(filter.UserID)
	}
	tierRaw := strings.TrimSpace(r.URL.Query().Get("tier"))
	if tierRaw == "" {
		return ""
	}
	tier := featuregate.NormalizeTier(featuregate.Tier(tierRaw))
	return "shell:" + string(tier)
}

// GET /api/product/usage/summary — User Feature Summary (UsageSummary rows).
func (h *ProductCapabilitiesHandler) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	filter, err := parseUsageAnalyticsFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "invalid from/to/feature/event_type",
		})
		return
	}
	userID := resolveUsageUserID(r, filter)
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "user_id or tier required",
		})
		return
	}
	rows, err := usagemetrics.DefaultAnalyticsReader().UserSummary(context.Background(), userID, filter.From, filter.To)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code": ProductCodeInternal, "ok": false, "message": err.Error(),
		})
		return
	}
	if rows == nil {
		rows = []usagemetrics.UsageSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"user_id": userID,
		"from":    nullableTime(filter.From),
		"to":      nullableTime(filter.To),
		"summary": rows,
	})
}

// GET /api/product/usage/count — Feature Usage Count.
func (h *ProductCapabilitiesHandler) handleUsageCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	filter, err := parseUsageAnalyticsFilter(r)
	if err != nil || filter.Feature == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "feature required; optional from/to/event_type/user_id",
		})
		return
	}
	filter.UserID = resolveUsageUserID(r, filter)
	n, err := usagemetrics.DefaultAnalyticsReader().FeatureCount(context.Background(), filter)
	if err != nil {
		msg := err.Error()
		code := ProductCodeInternal
		status := http.StatusInternalServerError
		if strings.Contains(msg, "invalid feature") || strings.Contains(msg, "feature is required") {
			code = ProductCodeBadRequest
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{
			"code": code, "ok": false, "message": msg,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"feature":    string(filter.Feature),
		"event_type": string(filter.EventType),
		"user_id":    filter.UserID,
		"from":       nullableTime(filter.From),
		"to":         nullableTime(filter.To),
		"count":      n,
	})
}

// GET /api/product/usage/analytics — commercial rollup (Pro feature evaluation / Product Dashboard).
func (h *ProductCapabilitiesHandler) handleUsageAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	filter, err := parseUsageAnalyticsFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"code": ProductCodeBadRequest, "ok": false, "message": "invalid from/to/feature/event_type",
		})
		return
	}
	// Optional user scope; omit user_id for cross-user commercial rollup.
	filter.UserID = strings.TrimSpace(r.URL.Query().Get("user_id"))
	reader := usagemetrics.DefaultAnalyticsReader()
	rows, err := reader.CommercialSummary(context.Background(), filter)
	if err != nil {
		msg := err.Error()
		code := ProductCodeInternal
		status := http.StatusInternalServerError
		if strings.Contains(msg, "invalid") || strings.Contains(msg, "not in commercial") {
			code = ProductCodeBadRequest
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{
			"code": code, "ok": false, "message": msg,
		})
		return
	}
	if rows == nil {
		rows = []usagemetrics.UsageSummary{}
	}
	feats := make([]string, 0, len(reader.Catalog()))
	for _, f := range reader.Catalog() {
		feats = append(feats, string(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"commercial_features": feats,
		"from":                nullableTime(filter.From),
		"to":                  nullableTime(filter.To),
		"summary":             rows,
	})
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
