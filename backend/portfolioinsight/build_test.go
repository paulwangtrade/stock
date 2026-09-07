package portfolioinsight

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/providershadow"
	"go-stock/backend/rebalance"
)

func f64(v float64) *float64 { return &v }

func TestBuild_MissingInputsSafe(t *testing.T) {
	got := Build(Input{})
	require.NotNil(t, got)
	require.True(t, got.RecordOnly)
	require.True(t, got.NotTradingAdvice)
	require.True(t, got.NotAutoTrade)
	require.True(t, got.NotATradePlan)
	require.True(t, got.NotExecution)
	require.Equal(t, RiskUnavailable, got.PortfolioSummary.RiskLevel)
	require.Contains(t, got.DataGaps, "risk_snapshot_missing")
	require.Nil(t, got.PortfolioSummary.CashRatio)
	require.Nil(t, got.PortfolioSummary.SectorConcentration)
	require.Empty(t, got.WhyReduce)
}

func TestBuild_SectorUnavailableNotMisleading(t *testing.T) {
	risk := &portfoliorisk.PortfolioRiskSnapshot{
		Found: true,
		AsOf:  time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		Exposure: portfoliorisk.ExposureBlock{
			Available: true, CashRatio: f64(0.22), HeadroomVsCap: f64(0.10), CapGross: f64(0.85),
		},
		Concentration: portfoliorisk.ConcentrationBlock{
			Available: true, NameCount: 5, Top1Weight: f64(0.18), CapSingle: f64(0.20),
		},
		Sector: portfoliorisk.SectorBlock{
			Available: false,
			Note:      portfoliorisk.NoteSectorUnavailable,
		},
	}
	got := Build(Input{Risk: risk})
	require.Contains(t, got.DataGaps, "sector_unavailable")
	require.Nil(t, got.PortfolioSummary.SectorConcentration)
	require.Contains(t, got.PortfolioSummary.SectorUnavailableNote, "暂不可用")
	require.NotContains(t, got.PortfolioSummary.SectorUnavailableNote, "0%")
	// why_buy sector factor must mark available=false, not invent overheat
	require.NotNil(t, got.WhyBuyLimited)
	var sectorFactor *BuyLimitFactor
	for i := range got.WhyBuyLimited.Factors {
		if got.WhyBuyLimited.Factors[i].Code == "SECTOR" {
			sectorFactor = &got.WhyBuyLimited.Factors[i]
			break
		}
	}
	require.NotNil(t, sectorFactor)
	require.False(t, sectorFactor.Available)
	require.Contains(t, sectorFactor.PlainText, "未将")
}

func TestBuild_ExplanationStable(t *testing.T) {
	in := sampleInput()
	a := Build(in)
	b := Build(in)
	require.Equal(t, a.SourcesFingerprint, b.SourcesFingerprint)
	ja, err := json.Marshal(a.WhyReduce)
	require.NoError(t, err)
	jb, err := json.Marshal(b.WhyReduce)
	require.NoError(t, err)
	require.JSONEq(t, string(ja), string(jb))
	require.Equal(t, a.PortfolioSummary.RiskLevel, b.PortfolioSummary.RiskLevel)
	require.Equal(t, a.PortfolioSummary.RiskLevelReasons, b.PortfolioSummary.RiskLevelReasons)
}

func TestBuild_WhyReduceFromRuleHits(t *testing.T) {
	in := sampleInput()
	got := Build(in)
	require.Len(t, got.WhyReduce, 2)
	require.Equal(t, "sz000001", got.WhyReduce[0].Symbol)
	require.Equal(t, rules.ActionReduce, got.WhyReduce[0].ActionObserved)
	require.Contains(t, strings.Join(got.WhyReduce[0].PlainReasons, "|"), "止盈")
	require.Equal(t, DisclaimerZH, got.WhyReduce[0].Disclaimer)
	// HOLD must not produce why_reduce
	for _, w := range got.WhyReduce {
		require.NotEqual(t, rules.ActionHold, w.ActionObserved)
	}
}

func TestBuild_WhyBuyLimitedFactors(t *testing.T) {
	in := sampleInput()
	got := Build(in)
	require.NotNil(t, got.WhyBuyLimited)
	codes := map[string]bool{}
	for _, f := range got.WhyBuyLimited.Factors {
		codes[f.Code] = true
		require.NotEmpty(t, f.PlainText)
	}
	require.True(t, codes["GROSS"] || codes["SECTOR"] || codes["TIGHTEN"] || codes["CASH"])
	require.NotNil(t, got.WhyBuyLimited.ShadowContrast)
	require.Contains(t, got.WhyBuyLimited.ShadowContrast.Note, "仅观察")
}

func TestBuild_NoForbiddenPhrases(t *testing.T) {
	got := Build(sampleInput())
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	text := string(raw)
	for _, bad := range forbiddenPhrases {
		require.NotContains(t, text, bad)
	}
}

func TestFromShadowRecord(t *testing.T) {
	cap := 300_000.0
	rec := &providershadow.ShadowComparisonRecord{
		Fingerprint: "abc",
		AllocationBudgetSummary: &providershadow.AllocationBudgetSummary{
			ImpliedLegacyNotional: 500_000,
			Binding:               "gross",
			Portfolio: providershadow.BudgetView{
				AvailableCapital: cap,
				Binding:          "gross",
			},
		},
		RiskConstraintTrace: &providershadow.RiskAdjustmentTrace{
			Applied: true, SuggestHasPatches: true,
		},
		RiskCutSummary: &providershadow.RiskCutSummary{
			GrossHeadroomBinding: true,
		},
	}
	v := FromShadowRecord(rec)
	require.True(t, v.Present)
	require.True(t, v.TightenApplied)
	require.True(t, v.GrossHeadroomBinding)
	require.Equal(t, "gross", v.BudgetBinding)
	require.InDelta(t, 500_000, *v.LegacyBuyNotional, 1e-6)
	require.InDelta(t, cap, *v.PortfolioBuyNotional, 1e-6)
}

func sampleInput() Input {
	return Input{
		AsOf:      time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-21",
		Risk: &portfoliorisk.PortfolioRiskSnapshot{
			Found:             true,
			InputsFingerprint: "risk-fp-1",
			Exposure: portfoliorisk.ExposureBlock{
				Available: true, CashRatio: f64(0.15), HeadroomVsCap: f64(0), CapGross: f64(0.85),
			},
			Concentration: portfoliorisk.ConcentrationBlock{
				Available: true, NameCount: 8, Top1Weight: f64(0.22), CapSingle: f64(0.20),
			},
			Sector: portfoliorisk.SectorBlock{
				Available: true, MaxSectorWeight: f64(0.25),
				SectorExposure: []portfoliorisk.SectorWeight{
					{Sector: "银行", Weight: 0.28, NameCount: 3},
					{Sector: "地产", Weight: 0.12, NameCount: 1},
				},
			},
			Market: portfoliorisk.MarketBlock{Available: true, BlockNewEntries: false},
		},
		Holdings: []rules.HoldingDecision{
			{
				Symbol: "sz000001", FinalAction: rules.ActionReduce, Action: rules.ActionReduce,
				ReasonCodes: []string{rules.ReasonPnLTakeProfit, rules.ReasonRiskNameOverCap},
				RuleHits: []rules.RuleHit{
					{RuleID: rules.RulePnLTakeProfit, ReasonCode: rules.ReasonPnLTakeProfit, ActionCandidate: rules.ActionReduce},
				},
				Evidence:       map[string]any{"return_rate": 0.25, "weight": 0.22},
				SuggestOnly:    true,
				ExecutableHint: false,
			},
			{
				Symbol: "sz000002", FinalAction: rules.ActionExit, Action: rules.ActionExit,
				ReasonCodes: []string{rules.ReasonRiskSectorHot},
				SuggestOnly: true,
			},
			{
				Symbol: "sh600000", FinalAction: rules.ActionHold, Action: rules.ActionHold,
				ReasonCodes: []string{rules.ReasonDefaultHold},
			},
		},
		AllocationShadow: &AllocationShadowView{
			Present: true, TightenApplied: true, SuggestHasPatches: true,
			BudgetBinding: "gross", GrossHeadroomBinding: true,
			LegacyBuyNotional: f64(500_000), PortfolioBuyNotional: f64(200_000),
			ShadowFingerprint: "shadow-fp-1",
		},
		Rebalance: &rebalance.RebalanceSuggestion{
			SuggestOnly: true, InputsFingerprint: "reb-fp-1",
			RiskImpact: rebalance.RiskImpact{NamesBuyClipped: 1},
		},
	}
}
