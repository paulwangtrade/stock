package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-stock/backend/assistant"
	"go-stock/backend/assistant/provider"

	"github.com/stretchr/testify/require"
)

func sampleContext() *assistant.AssistantContext {
	return &assistant.AssistantContext{
		SchemaVersion: assistant.ContextSchemaVersion,
		Scene:         assistant.SceneRiskExplain,
		BuiltAt:       time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
		Status:        "ok",
		Facts: assistant.FactBundle{
			Risk: &assistant.RiskFact{
				ReportID: "rr-1",
				Overall:  42,
				Band:     "medium",
			},
			Stock: &assistant.StockFact{Code: "600000", Name: "示例股"},
			Disclaimers: assistant.DefaultResponseDisclaimers(),
		},
		PromptSkeleton: "SYSTEM:\nExplain only. No trade advice.\n",
	}
}

func TestMockAIProvider_Success(t *testing.T) {
	p := provider.NewMockAIProvider()
	require.Equal(t, "mock", p.Name())

	out, err := p.Analyze(context.Background(), sampleContext())
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, assistant.ResponseStatusOK, out.Status)
	require.Equal(t, "mock", out.Provider)
	require.Equal(t, assistant.SceneRiskExplain, out.Scene)
	require.NotEmpty(t, out.Title)
	require.NotEmpty(t, out.Summary)
	require.NotEmpty(t, out.Body)
	require.Contains(t, out.Body, "风险")
	require.NotEmpty(t, out.Bullets)
	require.NotEmpty(t, out.Disclaimers)
	require.Equal(t, assistant.ResponseSchemaVersion, out.SchemaVersion)
	require.False(t, out.GeneratedAt.IsZero())

	// Must not look like trade instructions.
	require.NotContains(t, out.Body, "建议买入")
	require.NotContains(t, out.Body, "limit_price")
}

func TestMockAIProvider_Failure(t *testing.T) {
	p := provider.NewMockAIProvider()
	p.Fail = provider.ErrMockForced

	out, err := p.Analyze(context.Background(), sampleContext())
	require.Error(t, err)
	require.Nil(t, out)
	require.True(t, errors.Is(err, provider.ErrMockForced))
}

func TestMockAIProvider_EmptyContext_Error(t *testing.T) {
	p := provider.NewMockAIProvider()

	out, err := p.Analyze(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, out)
	require.True(t, errors.Is(err, provider.ErrNilContext))

	empty := &assistant.AssistantContext{
		SchemaVersion: assistant.ContextSchemaVersion,
		Scene:         assistant.SceneStockAnalysis,
		Status:        "ok",
		Facts:         assistant.FactBundle{},
	}
	out2, err2 := p.Analyze(context.Background(), empty)
	require.Error(t, err2)
	require.Nil(t, out2)
	require.True(t, errors.Is(err2, provider.ErrEmptyContext))
}

func TestMockAIProvider_EmptyContext_AllowEmpty(t *testing.T) {
	p := provider.NewMockAIProvider()
	p.AllowEmpty = true

	out, err := p.Analyze(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, assistant.ResponseStatusEmpty, out.Status)
	require.Contains(t, out.Summary, "不编造")

	empty := &assistant.AssistantContext{Status: "ok", Facts: assistant.FactBundle{}}
	out2, err2 := p.Analyze(context.Background(), empty)
	require.NoError(t, err2)
	require.Equal(t, assistant.ResponseStatusEmpty, out2.Status)
}

func TestMockAIProvider_DegradedContext(t *testing.T) {
	p := provider.NewMockAIProvider()
	ac := sampleContext()
	ac.Status = "degraded"
	ac.Degraded = true
	ac.Missing = []assistant.MissingFact{{Key: "strategy_snapshot", Reason: "not provided"}}

	out, err := p.Analyze(context.Background(), ac)
	require.NoError(t, err)
	require.Equal(t, assistant.ResponseStatusDegraded, out.Status)
	require.Contains(t, out.Body, "缺失")
}

func TestAIProvider_InterfaceCompliance(t *testing.T) {
	var _ provider.AIProvider = provider.NewMockAIProvider()
}
