package decision_test

import (
	"testing"
	"time"

	"go-stock/backend/portfolio/decision"

	"github.com/stretchr/testify/require"
)

func ptr(v float64) *float64 { return &v }

func TestBuild_NormalPortfolio(t *testing.T) {
	view := decision.Build(decision.Inputs{
		TradeDate:    "2026-08-18",
		AsOf:         time.Now(),
		AccountFound: true,
		Equity:       2_000_000,
		Cash:         400_000,
		Holdings: []decision.HoldingRow{
			{
				StockCode: "sz000001", CapitalUsed: 40_000, Weight: 0.02,
				InvestmentScore: ptr(85), PositionStatus: "HOLDING_PROFIT", RiskLevel: "LOW",
			},
		},
		Candidate: decision.CandidateRow{Present: true, Code: "sz000001", Score: ptr(86)},
		PreTrade:  decision.PreTradeInput{Present: true, Level: "PASS", CashEnough: true, Concentration: "NORMAL"},
		MissingOpportunityPackage: true,
		MissingEfficiencyPackage:  true,
	})
	require.Equal(t, decision.AttentionHold, view.OverallAttention)
	require.Equal(t, decision.AttentionHold, view.PortfolioHealth.Status)
	require.Equal(t, decision.AttentionHold, view.OpportunityAttention.UserAction)
	require.Equal(t, decision.AttentionHold, view.CapitalEfficiencyAttention.Status)
	require.NotContains(t, view.Explanation, "BUY")
	require.Contains(t, view.Explanation, "不自动买卖")
	require.Equal(t, decision.QualityDegraded, view.Quality) // G.1/G.2 packages missing flags
}

func TestBuild_NoPositions(t *testing.T) {
	view := decision.Build(decision.Inputs{
		AccountFound: true,
		Equity:       1_000_000,
		Cash:         1_000_000,
		Holdings:     nil,
		Candidate:    decision.CandidateRow{},
	})
	require.Equal(t, 0, view.PortfolioHealth.PositionCount)
	require.Equal(t, decision.AttentionHold, view.OpportunityAttention.Status)
	require.Equal(t, 0, view.CapitalEfficiencyAttention.LowEfficiencyCount)
	require.Contains(t, view.PortfolioHealth.Notes, "当前无持仓")
}

func TestBuild_NoOpportunity(t *testing.T) {
	view := decision.Build(decision.Inputs{
		AccountFound: true,
		Equity:       2_000_000,
		Cash:         300_000,
		Holdings: []decision.HoldingRow{
			{StockCode: "sz000002", CapitalUsed: 40_000, Weight: 0.02, InvestmentScore: ptr(85), PositionStatus: "HOLDING_PROFIT", RiskLevel: "LOW"},
		},
		Candidate: decision.CandidateRow{}, // no candidate
		PreTrade:  decision.PreTradeInput{Present: true, Level: "PASS", CashEnough: true, Concentration: "NORMAL"},
	})
	require.Equal(t, decision.AttentionHold, view.OpportunityAttention.Status)
	require.Empty(t, view.OpportunityAttention.CandidateCode)
	require.Equal(t, decision.AttentionHold, view.OverallAttention)
}

func TestBuild_LowCapitalEfficiency(t *testing.T) {
	// weight 0.15 → burden 150→100; eff = 50 - 50 = 0 → LOW
	view := decision.Build(decision.Inputs{
		AccountFound: true,
		Equity:       2_000_000,
		Cash:         200_000,
		Holdings: []decision.HoldingRow{
			{
				StockCode: "sz000002", CapitalUsed: 300_000, Weight: 0.15,
				InvestmentScore: ptr(50), PositionStatus: "HOLDING_LOSS", RiskLevel: "MEDIUM",
				AttentionReason: "亏损关注",
			},
		},
	})
	require.GreaterOrEqual(t, view.CapitalEfficiencyAttention.LowEfficiencyCount, 1)
	require.Equal(t, decision.EffLow, view.CapitalEfficiencyAttention.Items[0].EfficiencyLevel)
	require.True(t,
		view.CapitalEfficiencyAttention.Status == decision.AttentionWatch ||
			view.CapitalEfficiencyAttention.Status == decision.AttentionReview)
	require.NotEqual(t, decision.AttentionHold, view.OverallAttention)
}

func TestBuild_RiskElevated(t *testing.T) {
	view := decision.Build(decision.Inputs{
		AccountFound: true,
		Equity:       2_000_000,
		Cash:         100_000,
		Holdings: []decision.HoldingRow{
			{
				StockCode: "sz000003", CapitalUsed: 100_000, Weight: 0.05,
				InvestmentScore: ptr(55), PositionStatus: "NEED_REVIEW", RiskLevel: "HIGH",
				AttentionReason: "风险升高需复盘",
			},
		},
		PreTrade: decision.PreTradeInput{Present: true, Level: "BLOCKED", CashEnough: false, Concentration: "HIGH"},
	})
	require.Equal(t, decision.AttentionReview, view.RiskAttention.Status)
	require.Equal(t, decision.AttentionReview, view.OverallAttention)
	require.Equal(t, "BLOCKED", view.RiskAttention.PretradeLevel)
	require.NotEmpty(t, view.RiskAttention.IntelligenceItems)
}

func TestBuild_OpportunityMapsReplaceToReview(t *testing.T) {
	view := decision.Build(decision.Inputs{
		AccountFound: true,
		Equity:       2_000_000,
		Cash:         50_000,
		Holdings: []decision.HoldingRow{
			{StockCode: "sz000002", CapitalUsed: 200_000, Weight: 0.10, InvestmentScore: ptr(55)},
		},
		Candidate: decision.CandidateRow{Present: true, Code: "sz000001", Name: "平安银行", Score: ptr(90)},
	})
	require.Equal(t, "POSSIBLE_REPLACE", view.OpportunityAttention.SourceAction)
	require.Equal(t, "sz000001", view.OpportunityAttention.CandidateCode)
	require.Equal(t, "平安银行", view.OpportunityAttention.CandidateName)
	require.Equal(t, decision.AttentionReview, view.OpportunityAttention.UserAction)
	require.NotEqual(t, "BUY", view.OpportunityAttention.UserAction)
	require.NotEqual(t, "SELL", view.OverallAttention)
	require.NotEqual(t, "AUTO_REBALANCE", view.OverallAttention)
}

func TestBuild_ForbiddenActionsNeverEmitted(t *testing.T) {
	view := decision.Build(decision.Inputs{AccountFound: true, Equity: 1, Cash: 1})
	for _, s := range []string{
		view.OverallAttention,
		view.PortfolioHealth.Status,
		view.OpportunityAttention.Status,
		view.OpportunityAttention.UserAction,
		view.CapitalEfficiencyAttention.Status,
		view.RiskAttention.Status,
	} {
		require.Contains(t, []string{decision.AttentionHold, decision.AttentionReview, decision.AttentionWatch}, s)
	}
}
