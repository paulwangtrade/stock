package score_test

import (
	"testing"
	"time"

	"go-stock/backend/portfolio/score"

	"github.com/stretchr/testify/require"
)

func TestBuild_CandidateOnly(t *testing.T) {
	view := score.Build(score.Inputs{
		StockCode: "sz000001",
		TradeDate: "2026-08-18",
		AsOf:      time.Now(),
		Candidate: score.CandidateInput{
			Present:     true,
			StockName:   "平安",
			Score:       0.90,
			SignalScore: 0.80,
			HasSignal:   true,
			Rank:        1,
		},
	})
	require.True(t, view.Found)
	require.Equal(t, score.SubjectCandidate, view.SubjectType)
	require.NotNil(t, view.StrategyScore)
	require.InDelta(t, 90, *view.StrategyScore, 1e-6)
	require.NotNil(t, view.MomentumScore)
	require.InDelta(t, 80, *view.MomentumScore, 1e-6)
	require.Nil(t, view.RiskScore, "no fake neutral risk")
	require.Nil(t, view.PortfolioFitScore, "no fake fit without holding")
	require.Equal(t, score.QualityDegraded, view.Quality)
	require.Contains(t, view.MissingFactors, "risk_score")
	require.Contains(t, view.MissingFactors, "portfolio_fit_score")
	require.NotNil(t, view.TotalScore)
}

func TestBuild_HoldingOnly(t *testing.T) {
	ret := -0.06
	view := score.Build(score.Inputs{
		StockCode: "sz000002",
		Holding: score.HoldingInput{
			Present:          true,
			StockName:        "万科",
			Weight:           0.12,
			UnrealizedReturn: &ret,
			RiskLevel:        "MEDIUM",
			PositionStatus:   "HOLDING_LOSS",
			StrategyStatus:   "UNKNOWN",
		},
	})
	require.Equal(t, score.SubjectHolding, view.SubjectType)
	require.Nil(t, view.StrategyScore, "UNKNOWN strategy must not invent score")
	require.NotNil(t, view.RiskScore)
	require.NotNil(t, view.MomentumScore)
	require.NotNil(t, view.PortfolioFitScore)
	require.Contains(t, view.MissingFactors, "strategy_score")
	require.Equal(t, score.QualityDegraded, view.Quality)
	require.NotNil(t, view.TotalScore)
}

func TestBuild_Both(t *testing.T) {
	ret := 0.02
	view := score.Build(score.Inputs{
		StockCode: "sz000001",
		Candidate: score.CandidateInput{
			Present: true, Score: 0.85, SignalScore: 0.7, HasSignal: true,
		},
		Holding: score.HoldingInput{
			Present: true, Weight: 0.04, UnrealizedReturn: &ret,
			RiskLevel: "LOW", PositionStatus: "HOLDING_PROFIT", StrategyStatus: "UNKNOWN",
		},
	})
	require.Equal(t, score.SubjectBoth, view.SubjectType)
	require.NotNil(t, view.StrategyScore)
	require.InDelta(t, 85, *view.StrategyScore, 1e-6)
	require.NotNil(t, view.MomentumScore)
	require.NotNil(t, view.RiskScore)
	require.NotNil(t, view.PortfolioFitScore)
	require.Equal(t, score.QualityOK, view.Quality)
	require.Empty(t, view.MissingFactors)
	require.NotNil(t, view.TotalScore)
}

func TestBuild_MissingComponent_NoFakeData(t *testing.T) {
	view := score.Build(score.Inputs{
		StockCode: "sz000003",
		Candidate: score.CandidateInput{
			Present: true, Score: 0.5, HasSignal: false, // no signal → momentum null
		},
	})
	require.Nil(t, view.MomentumScore)
	require.Nil(t, view.RiskScore)
	require.Nil(t, view.PortfolioFitScore)
	require.Contains(t, view.MissingFactors, "momentum_score")
}

func TestBuild_NotFound(t *testing.T) {
	view := score.Build(score.Inputs{StockCode: "sz999999"})
	require.False(t, view.Found)
	require.Equal(t, score.QualityNotFound, view.Quality)
	require.Nil(t, view.TotalScore)
}

func TestBuild_CandidateRiskCodeOnlyWhenPresent(t *testing.T) {
	view := score.Build(score.Inputs{
		StockCode: "sz000001",
		Candidate: score.CandidateInput{
			Present: true, Score: 0.8, HasSignal: true, SignalScore: 0.5,
			TradePlanRiskCode: "CASH_INSUFFICIENT",
		},
	})
	require.NotNil(t, view.RiskScore)
	require.InDelta(t, 40, *view.RiskScore, 1e-6)
}
