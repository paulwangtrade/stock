package assistant_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-stock/backend/assistant"
	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/riskreport"
	"go-stock/backend/strategysnapshot"
	"go-stock/backend/usagemetrics"

	"github.com/stretchr/testify/require"
)

func proUser(t *testing.T) *featuregate.User {
	t.Helper()
	u := &featuregate.User{ID: "assist-pro", Tier: featuregate.TierPro}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(u))
	require.True(t, featuregate.Allow(u, featuregate.FeatureAIAnalysis))
	return u
}

func sampleInputs() (*models.TradePlan, *strategysnapshot.StrategySnapshot, *riskreport.RiskReport, *papertrading.ExecutionSummaryView) {
	plan := &models.TradePlan{
		ID: 42, TradeDate: "2026-08-10", PlanVersion: 2, Status: models.TradePlanStatusDraft,
		RiskStatus: "ok", RiskAcceptedCount: 1, PricingStage: "after_close_intent",
		Items: []models.TradePlanItem{{ID: 1, StockCode: "sz000001"}},
	}
	accepted := true
	snap := &strategysnapshot.StrategySnapshot{
		SnapshotID: "sshot:42:1:2", Scope: strategysnapshot.ScopeItem, PlanID: 42, PlanItemID: 1,
		StrategyVersion: strategysnapshot.StrategyVersion{StrategyName: "demo", Version: "run:9"},
		SignalResult:    strategysnapshot.SignalResult{Tag: "强", Score: 0.8, SignalSnapshotID: 55},
		MarketDataRef:   strategysnapshot.MarketDataReference{RefPrice: 10.5, RefSource: "kline_close"},
		RiskDecision:    strategysnapshot.RiskDecision{ItemAccepted: &accepted},
	}
	rr := &riskreport.RiskReport{
		ReportID: "riskrpt:1", Status: riskreport.StatusOK, DataQuality: "OK",
		Score: riskreport.RiskScore{Overall: 45, Band: riskreport.BandMedium},
		Factors: []riskreport.RiskFactor{{Code: "PORTFOLIO_CONCENTRATION"}},
		Warnings: []riskreport.RiskWarning{{Message: "集中度偏高"}},
		Suggestions: []riskreport.RiskSuggestion{{Kind: riskreport.SuggestReview, Message: "建议复核仓位"}},
	}
	exec := &papertrading.ExecutionSummaryView{
		Enabled: true, TradeDate: "2026-08-10", TotalOrders: 3, FilledOrders: 2, FailedOrders: 1, FillRate: 2.0 / 3.0,
		DataSourceNote: "paper_sim",
	}
	return plan, snap, rr, exec
}

func TestContextBuilder_FullInputs(t *testing.T) {
	plan, snap, rr, exec := sampleInputs()
	ctx, err := assistant.NewContextBuilder().Build(assistant.BuildInput{
		Scene: assistant.SceneTradePlanExplain, Plan: plan, Snapshot: snap, RiskReport: rr, Execution: exec,
		StockCode: "sz000001", StockName: "平安银行",
		Now: time.Date(2026, 8, 9, 18, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, "ok", ctx.Status)
	require.False(t, ctx.Degraded)
	require.NotNil(t, ctx.Facts.Plan)
	require.Equal(t, uint(42), ctx.Facts.Plan.PlanID)
	require.NotNil(t, ctx.Facts.StrategySnap)
	require.Equal(t, "强", ctx.Facts.StrategySnap.SignalTag)
	require.NotNil(t, ctx.Facts.Risk)
	require.Equal(t, 45, ctx.Facts.Risk.Overall)
	require.NotNil(t, ctx.Facts.Execution)
	require.Equal(t, 3, ctx.Facts.Execution.TotalOrders)
	require.Contains(t, ctx.PromptSkeleton, "Do NOT recommend buy/sell")
	require.NotContains(t, strings.ToLower(ctx.PromptSkeleton), `"side":"buy"`)
	require.NotEmpty(t, ctx.Facts.Disclaimers)
}

func TestContextBuilder_MissingInputs_Degraded(t *testing.T) {
	ctx, err := assistant.NewContextBuilder().Build(assistant.BuildInput{
		Scene: assistant.SceneRiskExplain,
	})
	require.NoError(t, err)
	require.True(t, ctx.Degraded)
	require.Equal(t, "degraded", ctx.Status)
	require.NotEmpty(t, ctx.Missing)
	require.Contains(t, ctx.PromptSkeleton, "MISSING")
}

func TestService_BuildContext_RecordsUsage(t *testing.T) {
	plan, snap, rr, exec := sampleInputs()
	user := proUser(t)
	before, err := usagemetrics.Default().Count(context.Background(), usagemetrics.QueryFilter{
		UserID: user.ID, Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventContextGenerated,
	})
	require.NoError(t, err)

	out, err := assistant.NewService(nil).BuildContext(context.Background(), assistant.ContextRequest{
		User: user, Scene: assistant.SceneTradePlanExplain,
		Plan: plan, Snapshot: snap, RiskReport: rr, Execution: exec,
	})
	require.NoError(t, err)
	require.Equal(t, "ok", out.Status)

	after, err := usagemetrics.Default().Count(context.Background(), usagemetrics.QueryFilter{
		UserID: user.ID, Feature: featuregate.FeatureAIAnalysis, EventType: usagemetrics.EventContextGenerated,
	})
	require.NoError(t, err)
	require.Greater(t, after, before)
}

func TestService_Gated(t *testing.T) {
	free := &featuregate.User{ID: "assist-free", Tier: featuregate.TierFree}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(free))
	out, err := assistant.NewService(nil).BuildContext(context.Background(), assistant.ContextRequest{
		User: free, Scene: assistant.SceneTradePlanExplain,
	})
	require.NoError(t, err)
	require.Equal(t, "gated", out.Status)
}

func TestContext_NoTradeAdviceFields(t *testing.T) {
	plan, snap, rr, exec := sampleInputs()
	ctx, err := assistant.NewContextBuilder().Build(assistant.BuildInput{
		Plan: plan, Snapshot: snap, RiskReport: rr, Execution: exec,
	})
	require.NoError(t, err)
	require.NotNil(t, ctx.Facts.Plan)
	require.Contains(t, ctx.PromptSkeleton, "Do NOT recommend buy/sell")
	require.Contains(t, ctx.PromptSkeleton, "do NOT invent orders")
	require.Contains(t, ctx.PromptSkeleton, "order JSON")
	// PlanFact is identity/status only — no executable order payload on the struct.
	require.Equal(t, uint(42), ctx.Facts.Plan.PlanID)
	require.Equal(t, models.TradePlanStatusDraft, ctx.Facts.Plan.Status)
}
