package attention_test

import (
	"testing"
	"time"

	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"

	"github.com/stretchr/testify/require"
)

func TestBuild_NormalPortfolio(t *testing.T) {
	scoreHigh := 88.0
	scoreLow := 40.0
	view := attention.Build(attention.Inputs{
		TradeDate: "2026-08-18",
		AsOf:      time.Now(),
		Decision: &decision.PortfolioDecisionSummary{
			OverallAttention: decision.AttentionWatch,
			Explanation:      "存在效率与机会留意点。",
			Quality:          decision.QualityOK,
			OpportunityAttention: decision.OpportunityAttention{
				Status:        decision.AttentionWatch,
				UserAction:    decision.AttentionWatch,
				CandidateCode: "sz000001",
				CandidateScore: &scoreHigh,
				Highlights: []decision.OpportunityHighlight{{
					HoldingCode:  "sz000002",
					HoldingScore: &scoreLow,
					Reason:       "候选分高于持仓",
				}},
			},
			CapitalEfficiencyAttention: decision.CapitalEfficiencyAttention{
				Status:             decision.AttentionWatch,
				LowEfficiencyCount: 1,
				Items: []decision.EfficiencyItem{{
					StockCode:       "sz000002",
					EfficiencyLevel: decision.EffLow,
				}},
			},
		},
		Intelligence: &intelligence.Bundle{
			Found: true,
			Positions: []intelligence.PositionIntelligenceView{{
				StockCode:       "sz000003",
				StockName:       "测试股",
				PositionStatus:  intelligence.StatusHoldingProfit,
				RiskLevel:       intelligence.RiskLow,
				AttentionReason: "",
			}},
		},
		Daily: &summary.DailyInvestmentSummaryView{
			Quality: summary.QualityOK,
			TomorrowFocus: []string{"关注明日开盘流动性"},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPass},
		},
	})
	require.Equal(t, attention.QualityOK, view.Quality)
	require.NotEmpty(t, view.Items)
	require.Contains(t, []string{attention.ActionWatch, attention.ActionReview}, view.OverallAction)
	assertActionsSafe(t, view)
	// priority ascending
	for i := 1; i < len(view.Items); i++ {
		require.LessOrEqual(t, view.Items[i-1].Priority, view.Items[i].Priority)
	}
	hasOpp, hasEff := false, false
	for _, it := range view.Items {
		if it.ItemType == attention.TypeOpportunity {
			hasOpp = true
		}
		if it.ItemType == attention.TypeEfficiency {
			hasEff = true
		}
	}
	require.True(t, hasOpp)
	require.True(t, hasEff)
}

func TestBuild_NoPositions(t *testing.T) {
	view := attention.Build(attention.Inputs{
		TradeDate: "2026-08-18",
		Decision: &decision.PortfolioDecisionSummary{
			OverallAttention: decision.AttentionHold,
			Explanation:      "暂无强制关注。",
			Quality:          decision.QualityOK,
			PortfolioHealth:  decision.PortfolioHealth{Found: true, PositionCount: 0, Status: decision.AttentionHold},
		},
		Intelligence: &intelligence.Bundle{Found: true, Positions: nil},
		Daily:        &summary.DailyInvestmentSummaryView{Quality: summary.QualityOK},
		Monitor:      &tradingdaymonitor.TradingDayMonitorView{Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPending}},
	})
	require.Equal(t, attention.ActionHold, view.OverallAction)
	require.Empty(t, view.Items)
	require.Equal(t, attention.QualityOK, view.Quality)
}

func TestBuild_NoOpportunity(t *testing.T) {
	view := attention.Build(attention.Inputs{
		TradeDate: "2026-08-18",
		Decision: &decision.PortfolioDecisionSummary{
			OverallAttention: decision.AttentionHold,
			Quality:          decision.QualityOK,
			OpportunityAttention: decision.OpportunityAttention{
				Status:     decision.AttentionHold,
				UserAction: decision.AttentionHold,
			},
		},
		Intelligence: &intelligence.Bundle{Found: true, Positions: []intelligence.PositionIntelligenceView{{
			StockCode:      "sz000001",
			PositionStatus: intelligence.StatusHoldingProfit,
			RiskLevel:      intelligence.RiskLow,
		}}},
		Daily:   &summary.DailyInvestmentSummaryView{Quality: summary.QualityOK},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPass}},
	})
	for _, it := range view.Items {
		require.NotEqual(t, attention.TypeOpportunity, it.ItemType)
	}
	require.Equal(t, attention.ActionHold, view.OverallAction)
}

func TestBuild_RiskElevated(t *testing.T) {
	view := attention.Build(attention.Inputs{
		TradeDate: "2026-08-18",
		Decision: &decision.PortfolioDecisionSummary{
			OverallAttention: decision.AttentionReview,
			Explanation:      "持仓风险升高。",
			Quality:          decision.QualityOK,
			RiskAttention: decision.RiskAttention{
				Status:        decision.AttentionReview,
				PretradeLevel: "BLOCKED",
				CashEnough:    boolPtr(false),
			},
		},
		Intelligence: &intelligence.Bundle{
			Found: true,
			Positions: []intelligence.PositionIntelligenceView{{
				StockCode:       "sz000099",
				StockName:       "高风险股",
				PositionStatus:  intelligence.StatusNeedReview,
				RiskLevel:       intelligence.RiskHigh,
				AttentionReason: "亏损扩大需复盘",
			}},
		},
		Daily: &summary.DailyInvestmentSummaryView{Quality: summary.QualityOK},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusFail, Reason: "CASH_SHORT", PlanID: 9},
		},
	})
	require.Equal(t, attention.ActionReview, view.OverallAction)
	require.GreaterOrEqual(t, view.Counts.Review, 1)
	assertActionsSafe(t, view)
	require.Equal(t, attention.TypeTrading, view.Items[0].ItemType) // priority 5 first
	hasRiskIntel := false
	for _, it := range view.Items {
		if it.Source == attention.SourceIntelligence && it.SuggestedAction == attention.ActionReview {
			hasRiskIntel = true
		}
		require.NotEqual(t, "BUY", it.SuggestedAction)
		require.NotEqual(t, "SELL", it.SuggestedAction)
		require.NotEqual(t, "AUTO_ACTION", it.SuggestedAction)
	}
	require.True(t, hasRiskIntel)
}

func TestBuild_DataDegraded(t *testing.T) {
	view := attention.Build(attention.Inputs{
		TradeDate: "2026-08-18",
		Decision:  nil,
		IntelErr:  errString("intel down"),
		Daily:     nil,
		Monitor:   nil,
	})
	require.Equal(t, attention.QualityDegraded, view.Quality)
	require.Contains(t, view.MissingInputs, "decision_summary")
	require.Contains(t, view.MissingInputs, "position_intelligence")
	require.Contains(t, view.MissingInputs, "daily_summary")
	require.Contains(t, view.MissingInputs, "trading_day_monitor")
	require.Equal(t, attention.ActionHold, view.OverallAction)
}

func TestClampAction_RejectsTradeVerbs(t *testing.T) {
	// Exercise via Build with poisoned upstream action projected through opportunity.
	view := attention.Build(attention.Inputs{
		TradeDate: "2026-08-18",
		Decision: &decision.PortfolioDecisionSummary{
			OverallAttention: "BUY",
			Explanation:      "should clamp",
			Quality:          decision.QualityOK,
			OpportunityAttention: decision.OpportunityAttention{
				Status:        "SELL",
				UserAction:    "AUTO_ACTION",
				CandidateCode: "sz000001",
				Highlights: []decision.OpportunityHighlight{{
					HoldingCode: "sz000002",
					Reason:      "poison",
				}},
			},
		},
		Intelligence: &intelligence.Bundle{Found: true},
		Daily:        &summary.DailyInvestmentSummaryView{Quality: summary.QualityOK},
		Monitor:      &tradingdaymonitor.TradingDayMonitorView{},
	})
	assertActionsSafe(t, view)
	require.Equal(t, attention.ActionReview, view.OverallAction)
}

func assertActionsSafe(t *testing.T, view *attention.DailyAttentionView) {
	t.Helper()
	require.Contains(t, []string{attention.ActionHold, attention.ActionWatch, attention.ActionReview}, view.OverallAction)
	for _, it := range view.Items {
		require.Contains(t, []string{attention.ActionHold, attention.ActionWatch, attention.ActionReview}, it.SuggestedAction)
		require.NotEmpty(t, it.ItemType)
		require.NotEmpty(t, it.Source)
		require.NotEmpty(t, it.Title)
		require.NotEmpty(t, it.Reason)
		require.NotEmpty(t, it.Severity)
		require.Greater(t, it.Priority, 0)
	}
}

func boolPtr(v bool) *bool { return &v }

type errString string

func (e errString) Error() string { return string(e) }
