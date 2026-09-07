package mobileprojection_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/mobileprojection"
	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolioobservation"

	"github.com/stretchr/testify/require"
)

func sampleObservation() *portfolioobservation.PortfolioObservationView {
	nc := 5
	cash := 0.25
	gross := 0.75
	top1 := 0.18
	headroom := 0.10
	return &portfolioobservation.PortfolioObservationView{
		SchemaVersion: portfolioobservation.SchemaVersion,
		AsOf:          time.Date(2026, 8, 22, 6, 0, 0, 0, time.UTC),
		TradeDate:     "2026-08-22",
		AccountID:     "paper-account-secret-001",
		RecordOnly:    true,
		ReadOnly:      true,
		CurrentPortfolio: portfolioobservation.CurrentPortfolio{
			Available:      true,
			RiskLevel:      "moderate",
			RiskLevelLabel: "中等",
			RiskReasons:    []string{"exposure_within_cap"},
			NameCount:      &nc,
			Exposure: portfolioobservation.ExposureView{
				Available:     true,
				GrossExposure: &gross,
				GrossNotional: ptrFloat(1_000_000),
				Equity:        ptrFloat(1_000_000),
				HeadroomVsCap: &headroom,
			},
			Concentration: portfolioobservation.ConcentrationView{
				Available:  true,
				Top1Weight: &top1,
				NameCount:  5,
			},
			Cash: portfolioobservation.CashView{
				Available: true,
				CashRatio: &cash,
			},
			Sector: portfolioobservation.SectorView{Available: false},
		},
		DecisionExplain: portfolioobservation.DecisionExplain{
			WhyBuyLess: []portfolioobservation.ExplainFactor{
				{Code: "GROSS", PlainText: "敞口接近上限", Available: true, Source: "risk"},
			},
			ReduceRows: []portfolioobservation.ReduceSuggestionRow{
				{Symbol: "600000", Action: "REDUCE", SuggestSellQty: 1000, Reason: "must not leak"},
			},
		},
		DataGaps: []string{"sell_suggestion_missing_or_disabled"},
	}
}

func sampleAttention() *attention.DailyAttentionView {
	return &attention.DailyAttentionView{
		TradeDate:     "2026-08-22",
		AsOf:          time.Date(2026, 8, 22, 6, 0, 0, 0, time.UTC),
		OverallAction: attention.ActionWatch,
		Headline:      "2 items",
		Items: []attention.DailyAttention{
			{
				ID: "a1", ItemType: attention.TypeRisk, Priority: 1,
				Title: "risk", Reason: "watch exposure", Source: attention.SourceIntelligence,
				Severity: attention.SeverityMedium, SuggestedAction: "SELL",
				RelatedPlanID: 42, StockCode: "600000", StockName: "浦发银行",
			},
		},
		Counts:     attention.AttentionCounts{Watch: 1, Total: 1},
		Quality:    attention.QualityOK,
		Disclaimer: attention.ActionHold,
	}
}

func TestBuild_DefaultOff_Skipped(t *testing.T) {
	t.Parallel()
	out := mobileprojection.Build(mobileprojection.Input{
		Observation: sampleObservation(),
	})
	require.NotNil(t, out)
	require.False(t, out.Enabled)
	require.True(t, out.Skipped)
	require.Equal(t, "disabled", out.SkipReason)
	require.Nil(t, out.Summary)
	require.Nil(t, out.Risk)
	require.Nil(t, out.Attention)
	require.Nil(t, out.Insight)
}

func TestBuild_Enabled_ProjectsSummaryRiskAttentionInsight(t *testing.T) {
	t.Parallel()
	out := mobileprojection.Build(mobileprojection.Input{
		Enabled:       true,
		DeviceIDHash8: "a1b2c3d4",
		Observation:   sampleObservation(),
		Attention:     sampleAttention(),
	})
	require.False(t, out.Skipped)
	require.NotNil(t, out.Summary)
	require.NotNil(t, out.Risk)
	require.NotNil(t, out.Attention)
	require.NotNil(t, out.Insight)

	require.True(t, out.Summary.Found)
	require.Equal(t, 5, out.Summary.NameCount)
	require.NotNil(t, out.Summary.CashRatio)
	require.Nil(t, findJSONKey(t, out, "account_id"))
	require.Nil(t, findJSONKey(t, out, "account_cash"))
	require.Nil(t, findJSONKey(t, out, "gross_notional"))
	require.Nil(t, findJSONKey(t, out, "equity"))
	require.Nil(t, findJSONKey(t, out, "suggest_sell_qty"))
	require.Nil(t, findJSONKey(t, out, "related_plan_id"))

	require.Equal(t, attention.ActionWatch, out.Summary.DecisionAttention)
	require.Equal(t, attention.ActionReview, out.Attention.Items[0].SuggestedAction)

	require.Equal(t, "moderate", out.Risk.RiskLevel)
	require.True(t, out.Insight.Available)
}

func TestBuild_FoundFalse_NoFakeZeros(t *testing.T) {
	t.Parallel()
	obs := sampleObservation()
	obs.CurrentPortfolio.Available = false
	obs.CurrentPortfolio.NameCount = nil
	out := mobileprojection.Build(mobileprojection.Input{Enabled: true, Observation: obs})
	require.NotNil(t, out.Summary)
	require.False(t, out.Summary.Found)
	require.Contains(t, out.Summary.DataGaps, "snapshot_missing")
}

func TestBuild_AIInsightInput_Sanitized(t *testing.T) {
	t.Parallel()
	ins := &mobileprojection.AIInsight{
		Available:     true,
		NarrativeMode: "rules_plus_llm",
		WhyBuyLimited: mobileprojection.ExplainSection{
			Available:  true,
			Headline:   "BUY now qty 100",
			Paragraphs: []string{"secret token leaked"},
			Bullets: []mobileprojection.ExplainBullet{
				{Code: "GROSS", PlainText: "ok", Source: "observation"},
			},
		},
	}
	out := mobileprojection.Build(mobileprojection.Input{
		Enabled: true, Observation: sampleObservation(), AIInsight: ins,
	})
	require.NotNil(t, out.Insight)
	require.NotContains(t, strings.ToUpper(out.Insight.WhyBuyLimited.Headline), "BUY")
}

func findJSONKey(t *testing.T, v any, key string) any {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var m any
	require.NoError(t, json.Unmarshal(b, &m))
	return walkJSON(m, strings.ToLower(key))
}

func walkJSON(node any, key string) any {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			if strings.ToLower(k) == key {
				return v
			}
			if found := walkJSON(v, key); found != nil {
				return found
			}
		}
	case []any:
		for _, v := range n {
			if found := walkJSON(v, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func ptrFloat(f float64) *float64 { return &f }

func TestIsolation_NoForbiddenImports(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir)
		dir = parent
	}
	pkg := filepath.Join(dir, "backend", "mobileprojection")
	forbidden := []string{
		"go-stock/backend/execution",
		"go-stock/backend/broker",
		"go-stock/backend/strategy",
		"go-stock/backend/data",
		"go-stock/backend/papertrading",
		"go-stock/backend/approvegate",
	}
	entries, err := os.ReadDir(pkg)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(pkg, e.Name()))
		require.NoError(t, err)
		text := string(b)
		for _, imp := range forbidden {
			require.NotContains(t, text, imp, e.Name())
		}
	}
}
