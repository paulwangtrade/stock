package assistant

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/riskreport"
	"go-stock/backend/strategysnapshot"
)

// BuildInput is the read-only assembly input for AssistantContextBuilder.
// Callers inject already-loaded facts; Builder does not write trading state.
type BuildInput struct {
	Scene     AssistantScene
	Locale    string
	StockCode string
	StockName string
	Now       time.Time

	Plan      *models.TradePlan
	Snapshot  *strategysnapshot.StrategySnapshot
	RiskReport *riskreport.RiskReport
	Execution *papertrading.ExecutionSummaryView
}

// ContextBuilder assembles AssistantContext from trading-adjacent read models.
type ContextBuilder interface {
	Build(in BuildInput) (*AssistantContext, error)
}

type contextBuilder struct{}

// NewContextBuilder returns the default Insight Context Builder.
func NewContextBuilder() ContextBuilder {
	return &contextBuilder{}
}

func (b *contextBuilder) Build(in BuildInput) (*AssistantContext, error) {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	scene := in.Scene
	if scene == "" {
		scene = SceneTradePlanExplain
	}

	ctx := &AssistantContext{
		SchemaVersion: ContextSchemaVersion,
		Scene:         scene,
		BuiltAt:       now,
		Status:        "ok",
		Facts: FactBundle{
			Disclaimers: defaultDisclaimers(),
		},
		Missing:   []MissingFact{},
		Redactions: []string{},
	}

	if in.Plan != nil {
		ctx.Facts.Plan = projectPlan(in.Plan)
	} else {
		ctx.Missing = append(ctx.Missing, MissingFact{Key: "trade_plan", Reason: "not provided"})
	}

	if in.Snapshot != nil {
		ctx.Facts.StrategySnap = projectSnapshot(in.Snapshot)
	} else {
		ctx.Missing = append(ctx.Missing, MissingFact{Key: "strategy_snapshot", Reason: "not provided"})
	}

	if in.RiskReport != nil && in.RiskReport.Status != riskreport.StatusGated && in.RiskReport.Status != riskreport.StatusFailed {
		ctx.Facts.Risk = projectRisk(in.RiskReport)
	} else if in.RiskReport == nil {
		ctx.Missing = append(ctx.Missing, MissingFact{Key: "risk_report", Reason: "not provided"})
	} else {
		ctx.Missing = append(ctx.Missing, MissingFact{Key: "risk_report", Reason: "gated_or_failed: " + in.RiskReport.Status})
	}

	if in.Execution != nil {
		ctx.Facts.Execution = projectExecution(in.Execution)
	} else {
		ctx.Missing = append(ctx.Missing, MissingFact{Key: "execution_summary", Reason: "not provided"})
	}

	code := strings.TrimSpace(in.StockCode)
	if code != "" || strings.TrimSpace(in.StockName) != "" {
		ctx.Facts.Stock = &StockFact{Code: code, Name: strings.TrimSpace(in.StockName)}
	}

	// Redact sensitive keys if any leaked into plan message (defensive).
	if in.Plan != nil && looksSensitive(in.Plan.Message) {
		ctx.Redactions = append(ctx.Redactions, "plan.message")
	}

	if len(ctx.Missing) > 0 {
		ctx.Degraded = true
		ctx.Status = "degraded"
	}

	ctx.PromptSkeleton = buildPromptSkeleton(ctx)
	return ctx, nil
}

func projectPlan(p *models.TradePlan) *PlanFact {
	return &PlanFact{
		PlanID:            p.ID,
		TradeDate:         p.TradeDate,
		PlanVersion:       p.PlanVersion,
		Status:            p.Status,
		SourceSession:      p.SourceSession,
		RiskStatus:        p.RiskStatus,
		RiskAcceptedCount: p.RiskAcceptedCount,
		RiskFilteredCount: p.RiskFilteredCount,
		ItemCount:         len(p.Items),
		PricingStage:      p.PricingStage,
	}
}

func projectSnapshot(s *strategysnapshot.StrategySnapshot) *StrategySnapFact {
	f := &StrategySnapFact{
		SnapshotID:      s.SnapshotID,
		Scope:           string(s.Scope),
		PlanID:          s.PlanID,
		PlanItemID:      s.PlanItemID,
		StrategyName:    s.StrategyVersion.StrategyName,
		StrategyVersion: s.StrategyVersion.Version,
		SignalTag:       s.SignalResult.Tag,
		SignalScore:     s.SignalResult.Score,
		RiskCode:        s.RiskDecision.RiskCode,
		RefPrice:        s.MarketDataRef.RefPrice,
		RefSource:       s.MarketDataRef.RefSource,
	}
	return f
}

func projectRisk(r *riskreport.RiskReport) *RiskFact {
	f := &RiskFact{
		ReportID:    r.ReportID,
		Overall:     r.Score.Overall,
		Band:        r.Score.Band,
		DataQuality: r.DataQuality,
	}
	for _, fac := range r.Factors {
		f.FactorCodes = append(f.FactorCodes, fac.Code)
	}
	for _, w := range r.Warnings {
		f.WarningMsgs = append(f.WarningMsgs, w.Message)
	}
	for _, s := range r.Suggestions {
		// Only pass review/monitor/inform text — never invent sell/buy orders.
		if s.Kind == riskreport.SuggestReview || s.Kind == riskreport.SuggestMonitor || s.Kind == riskreport.SuggestInform {
			f.SuggestMsgs = append(f.SuggestMsgs, s.Message)
		}
	}
	return f
}

func projectExecution(e *papertrading.ExecutionSummaryView) *ExecutionFact {
	return &ExecutionFact{
		TradeDate:    e.TradeDate,
		TotalOrders:  e.TotalOrders,
		FilledOrders: e.FilledOrders,
		FailedOrders: e.FailedOrders,
		FillRate:     e.FillRate,
		DataNote:     e.DataSourceNote,
	}
}

func looksSensitive(s string) bool {
	l := strings.ToLower(s)
	for _, k := range []string{"password", "secret", "api_key", "apikey", "token=", "license_key"} {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}

func defaultDisclaimers() []string {
	return []string{
		"本上下文仅供理解既有交易事实，不构成投资建议。",
		"禁止据此生成买卖指令、改价、撤单或修改 TradePlan / Execution。",
		"AssistantContext 不含下单字段；外部 AI 若输出指令应被丢弃。",
	}
}

func buildPromptSkeleton(ctx *AssistantContext) string {
	var b strings.Builder
	b.WriteString("SYSTEM:\n")
	b.WriteString("You are a read-only trading insight assistant for go-stock.\n")
	b.WriteString("Explain facts only. Do NOT recommend buy/sell, do NOT invent orders, ")
	b.WriteString("do NOT output limit_price/target_volume/order JSON.\n")
	b.WriteString("Respond in ")
	b.WriteString("zh-CN")
	b.WriteString(".\n\n")
	b.WriteString("SCENE: ")
	b.WriteString(string(ctx.Scene))
	b.WriteString("\n\nFACTS:\n")
	if ctx.Facts.Plan != nil {
		b.WriteString(fmt.Sprintf("- plan_id=%d trade_date=%s status=%s version=%d items=%d risk=%s\n",
			ctx.Facts.Plan.PlanID, ctx.Facts.Plan.TradeDate, ctx.Facts.Plan.Status,
			ctx.Facts.Plan.PlanVersion, ctx.Facts.Plan.ItemCount, ctx.Facts.Plan.RiskStatus))
	}
	if ctx.Facts.StrategySnap != nil {
		b.WriteString(fmt.Sprintf("- snapshot=%s strategy=%s/%s signal_tag=%s ref=%.4f(%s)\n",
			ctx.Facts.StrategySnap.SnapshotID, ctx.Facts.StrategySnap.StrategyName, ctx.Facts.StrategySnap.StrategyVersion,
			ctx.Facts.StrategySnap.SignalTag, ctx.Facts.StrategySnap.RefPrice, ctx.Facts.StrategySnap.RefSource))
	}
	if ctx.Facts.Risk != nil {
		b.WriteString(fmt.Sprintf("- risk_score=%d band=%s factors=%d\n",
			ctx.Facts.Risk.Overall, ctx.Facts.Risk.Band, len(ctx.Facts.Risk.FactorCodes)))
	}
	if ctx.Facts.Execution != nil {
		b.WriteString(fmt.Sprintf("- execution orders=%d filled=%d failed=%d fill_rate=%.2f\n",
			ctx.Facts.Execution.TotalOrders, ctx.Facts.Execution.FilledOrders,
			ctx.Facts.Execution.FailedOrders, ctx.Facts.Execution.FillRate))
	}
	if ctx.Facts.Stock != nil {
		b.WriteString(fmt.Sprintf("- stock=%s %s\n", ctx.Facts.Stock.Code, ctx.Facts.Stock.Name))
	}
	if len(ctx.Missing) > 0 {
		b.WriteString("\nMISSING:\n")
		for _, m := range ctx.Missing {
			b.WriteString(fmt.Sprintf("- %s: %s\n", m.Key, m.Reason))
		}
	}
	b.WriteString("\nDISCLAIMERS:\n")
	for _, d := range ctx.Facts.Disclaimers {
		b.WriteString("- ")
		b.WriteString(d)
		b.WriteString("\n")
	}
	b.WriteString("\nUSER: Explain the above facts without giving trade instructions.\n")
	return b.String()
}
