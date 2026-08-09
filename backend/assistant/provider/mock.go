package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/assistant"
)

// Sentinel errors for Mock / adapter tests.
var (
	ErrNilContext   = errors.New("provider: assistant context is nil")
	ErrEmptyContext = errors.New("provider: assistant context has no usable facts")
	ErrMockForced   = errors.New("provider: mock forced failure")
)

// MockAIProvider is a zero-network test double for AIProvider (dev/test only).
// It locally templates an explanation from AssistantContext — no external service,
// no user-data upload, no TradePlan/Strategy/Execution writes.
type MockAIProvider struct {
	// Fail when set, Analyze returns this error (provider failure path).
	Fail error
	// AllowEmpty when true, empty/nil contexts return status=empty instead of error.
	AllowEmpty bool
	now        func() time.Time
}

// NewMockAIProvider returns a success-oriented mock provider.
func NewMockAIProvider() *MockAIProvider {
	return &MockAIProvider{now: time.Now}
}

// Name implements AIProvider.
func (m *MockAIProvider) Name() string { return "mock" }

// Analyze implements AIProvider using local templating only.
func (m *MockAIProvider) Analyze(_ context.Context, ac *assistant.AssistantContext) (*assistant.AssistantResponse, error) {
	nowFn := m.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()

	if m.Fail != nil {
		return nil, m.Fail
	}

	if ac == nil {
		if m.AllowEmpty {
			return emptyResponse(now, m.Name(), ""), nil
		}
		return nil, ErrNilContext
	}

	if isEmptyContext(ac) {
		if m.AllowEmpty {
			return emptyResponse(now, m.Name(), string(ac.Scene)), nil
		}
		return nil, ErrEmptyContext
	}

	status := assistant.ResponseStatusOK
	if ac.Status == "degraded" || ac.Degraded || len(ac.Missing) > 0 {
		status = assistant.ResponseStatusDegraded
	}
	if ac.Status == "gated" {
		status = assistant.ResponseStatusGated
	}
	if ac.Status == "failed" {
		status = assistant.ResponseStatusFailed
	}

	title, summary, body, bullets, citations := mockCompose(ac)
	body = sanitizeExplanation(body)
	for i := range bullets {
		bullets[i] = sanitizeExplanation(bullets[i])
	}

	return &assistant.AssistantResponse{
		SchemaVersion: assistant.ResponseSchemaVersion,
		Scene:         ac.Scene,
		Status:        status,
		Title:         title,
		Summary:       summary,
		Body:          body,
		Bullets:       bullets,
		Citations:     citations,
		GeneratedAt:   now,
		Provider:      m.Name(),
		Confidence:    "unknown",
		Disclaimers:   assistant.DefaultResponseDisclaimers(),
	}, nil
}

func emptyResponse(now time.Time, providerName, scene string) *assistant.AssistantResponse {
	sc := assistant.AssistantScene(scene)
	return &assistant.AssistantResponse{
		SchemaVersion: assistant.ResponseSchemaVersion,
		Scene:         sc,
		Status:        assistant.ResponseStatusEmpty,
		Title:         "无可解释上下文",
		Summary:       "未提供可用事实，Mock Provider 不编造内容。",
		Body:          "AssistantContext 为空或缺少事实。请先由 Context Builder 装配只读事实后再分析。",
		GeneratedAt:   now,
		Provider:      providerName,
		Confidence:    "unknown",
		Disclaimers:   assistant.DefaultResponseDisclaimers(),
	}
}

func isEmptyContext(ac *assistant.AssistantContext) bool {
	if ac == nil {
		return true
	}
	f := ac.Facts
	hasFact := f.Plan != nil || f.StrategySnap != nil || f.Risk != nil || f.Execution != nil || f.Stock != nil
	hasPrompt := strings.TrimSpace(ac.PromptSkeleton) != ""
	return !hasFact && !hasPrompt
}

func mockCompose(ac *assistant.AssistantContext) (title, summary, body string, bullets []string, citations []assistant.Citation) {
	scene := string(ac.Scene)
	if scene == "" {
		scene = string(assistant.SceneTradePlanExplain)
	}
	title = fmt.Sprintf("【Mock】%s 解释摘要", scene)
	summary = "基于本地 AssistantContext 的模板化说明（无外部 AI、无上传）。"

	var b strings.Builder
	b.WriteString("以下内容由 MockAIProvider 根据只读事实拼装，仅用于辅助理解。\n")
	if ac.Facts.Stock != nil && (ac.Facts.Stock.Code != "" || ac.Facts.Stock.Name != "") {
		b.WriteString(fmt.Sprintf("- 标的：%s %s\n", ac.Facts.Stock.Code, ac.Facts.Stock.Name))
		citations = append(citations, assistant.Citation{Kind: "stock", Ref: ac.Facts.Stock.Code, Label: ac.Facts.Stock.Name})
		bullets = append(bullets, fmt.Sprintf("标的 %s", strings.TrimSpace(ac.Facts.Stock.Code+" "+ac.Facts.Stock.Name)))
	}
	if ac.Facts.Plan != nil {
		b.WriteString(fmt.Sprintf("- 计划：id=%d date=%s status=%s items=%d\n",
			ac.Facts.Plan.PlanID, ac.Facts.Plan.TradeDate, ac.Facts.Plan.Status, ac.Facts.Plan.ItemCount))
		citations = append(citations, assistant.Citation{Kind: "trade_plan", Ref: fmt.Sprintf("%d", ac.Facts.Plan.PlanID)})
		bullets = append(bullets, fmt.Sprintf("计划状态 %s（条目 %d）", ac.Facts.Plan.Status, ac.Facts.Plan.ItemCount))
	}
	if ac.Facts.StrategySnap != nil {
		b.WriteString(fmt.Sprintf("- 策略快照：%s signal=%s score=%.2f\n",
			ac.Facts.StrategySnap.SnapshotID, ac.Facts.StrategySnap.SignalTag, ac.Facts.StrategySnap.SignalScore))
		citations = append(citations, assistant.Citation{Kind: "strategy_snapshot", Ref: ac.Facts.StrategySnap.SnapshotID})
		bullets = append(bullets, fmt.Sprintf("信号 %s", ac.Facts.StrategySnap.SignalTag))
	}
	if ac.Facts.Risk != nil {
		b.WriteString(fmt.Sprintf("- 风险：band=%s score=%d\n", ac.Facts.Risk.Band, ac.Facts.Risk.Overall))
		citations = append(citations, assistant.Citation{Kind: "risk_report", Ref: ac.Facts.Risk.ReportID})
		bullets = append(bullets, fmt.Sprintf("风险档位 %s", ac.Facts.Risk.Band))
	}
	if ac.Facts.Execution != nil {
		b.WriteString(fmt.Sprintf("- 执行摘要：orders=%d filled=%d fill_rate=%.2f\n",
			ac.Facts.Execution.TotalOrders, ac.Facts.Execution.FilledOrders, ac.Facts.Execution.FillRate))
		citations = append(citations, assistant.Citation{Kind: "execution_summary", Label: ac.Facts.Execution.TradeDate})
		bullets = append(bullets, fmt.Sprintf("成交率 %.0f%%", ac.Facts.Execution.FillRate*100))
	}
	for _, m := range ac.Missing {
		b.WriteString(fmt.Sprintf("- 缺失：%s（%s）\n", m.Key, m.Reason))
	}
	b.WriteString("\n约束：只解释与总结；不给出买卖指令或可执行订单字段。\n")
	body = b.String()
	if len(bullets) == 0 {
		bullets = []string{"上下文事实已接收，Mock 仅做结构化复述"}
	}
	return title, summary, body, bullets, citations
}

// sanitizeExplanation strips common trade-instruction phrasing from mock text.
func sanitizeExplanation(s string) string {
	lower := strings.ToLower(s)
	banned := []string{
		"建议买入", "建议卖出", "立即买入", "立即卖出",
		"buy now", "sell now", "place order", "limit_price", "target_volume",
		`"side":"buy"`, `"side":"sell"`,
	}
	for _, w := range banned {
		if strings.Contains(lower, strings.ToLower(w)) || strings.Contains(s, w) {
			return "[已过滤疑似交易指令措辞] " + strings.ReplaceAll(s, w, "***")
		}
	}
	return s
}
