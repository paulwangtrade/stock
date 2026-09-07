package home_test

import (
	"testing"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/home"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/portfolio/readmodel"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"

	"github.com/stretchr/testify/require"
)

func TestAssemble_NormalTradingDay(t *testing.T) {
	pnl := 1000.0
	view := home.Assemble(home.Inputs{
		TradeDate: "2026-08-18",
		AsOf:      time.Now(),
		Dashboard: &portfolio.PortfolioDashboardView{
			Found: true,
			Summary: portfolio.PortfolioSummary{
				Equity: 2_000_000, Cash: 400_000, MarketValue: 1_600_000,
				DailyPnL: &pnl, PositionCount: 3,
			},
		},
		Decision: &decision.PortfolioDecisionSummary{
			OverallAttention: decision.AttentionHold,
			Explanation:      "组合暂无强制关注事项。系统仅提示关注，不自动买卖或调仓。",
			Quality:          decision.QualityOK,
		},
		Daily: &summary.DailyInvestmentSummaryView{
			Quality: summary.QualityOK,
			PortfolioSummary: summary.PortfolioSummaryBlock{
				Narrative: "今日组合权益 2000000，持仓 3 只",
			},
			TradingSummary: summary.TradingSummaryBlock{
				Narrative: "今日执行完成（session=A，plan_id=1）；结算已完成",
			},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Morning: tradingdaymonitor.MorningSection{
				Materialize: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
				Approve:     tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
				Freeze:      tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
			},
			Execution:  tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPass, PlanID: 1, Session: "A"},
			Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPass},
		},
		Intelligence: &intelligence.Bundle{Found: true},
	})
	require.Equal(t, home.QualityOK, view.Quality)
	require.True(t, view.PortfolioSummary.Found)
	require.Equal(t, 3, view.PortfolioSummary.PositionCount)
	require.NotNil(t, view.DecisionSummary)
	require.NotNil(t, view.DailySummary)
	require.NotNil(t, view.DailyAttention)
	require.Equal(t, attention.ActionHold, view.DailyAttention.OverallAction)
	require.Equal(t, tradingdaymonitor.StatusPass, view.TradingStatus.ExecutionStatus)
	require.Contains(t, view.TradingStatus.Narrative, "执行完成")
}

func TestAssemble_NoPositions(t *testing.T) {
	view := home.Assemble(home.Inputs{
		Dashboard: &portfolio.PortfolioDashboardView{
			Found:   true,
			Summary: portfolio.PortfolioSummary{Equity: 1_000_000, Cash: 1_000_000, PositionCount: 0},
		},
		Decision: &decision.PortfolioDecisionSummary{OverallAttention: decision.AttentionHold, Quality: decision.QualityOK},
		Daily: &summary.DailyInvestmentSummaryView{
			Quality:          summary.QualityOK,
			PortfolioSummary: summary.PortfolioSummaryBlock{Narrative: "今日组合权益 1000000，持仓 0 只，今日盈亏不可比"},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPending},
		},
		Intelligence: &intelligence.Bundle{Found: true, Positions: nil},
	})
	require.Equal(t, 0, view.PortfolioSummary.PositionCount)
	require.NotNil(t, view.DailyAttention)
	require.Equal(t, attention.ActionHold, view.DailyAttention.OverallAction)
	require.Empty(t, view.DailyAttention.Items)
	require.Empty(t, view.AttentionItems)
}

func TestAssemble_NoTrading(t *testing.T) {
	view := home.Assemble(home.Inputs{
		Dashboard: &portfolio.PortfolioDashboardView{Found: true, Summary: portfolio.PortfolioSummary{Equity: 1e6, Cash: 1e6}},
		Decision:  &decision.PortfolioDecisionSummary{OverallAttention: decision.AttentionHold, Quality: decision.QualityOK},
		Daily: &summary.DailyInvestmentSummaryView{
			Quality: summary.QualityOK,
			TradingSummary: summary.TradingSummaryBlock{
				ExecutionStatus: tradingdaymonitor.StatusPending,
				Narrative:       "今日未执行，原因：NO_FROZEN_PLAN",
			},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Morning:    tradingdaymonitor.MorningSection{Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusUnknown}},
			Execution:  tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPending},
			Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPending},
		},
		Intelligence: &intelligence.Bundle{Found: true},
	})
	require.Equal(t, tradingdaymonitor.StatusPending, view.TradingStatus.ExecutionStatus)
	require.Contains(t, view.TradingStatus.Narrative, "未执行")
}

func TestAssemble_ExecutionFailed(t *testing.T) {
	view := home.Assemble(home.Inputs{
		Dashboard: &portfolio.PortfolioDashboardView{Found: true},
		Decision:  &decision.PortfolioDecisionSummary{OverallAttention: decision.AttentionWatch, Quality: decision.QualityOK},
		Daily: &summary.DailyInvestmentSummaryView{
			Quality: summary.QualityOK,
			TradingSummary: summary.TradingSummaryBlock{
				ExecutionStatus: tradingdaymonitor.StatusFail,
				ExecutionReason: "broker_error",
				Narrative:       "今日执行失败，原因：broker_error",
			},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Execution: tradingdaymonitor.ExecutionSection{
				Status: tradingdaymonitor.StatusFail, Reason: "broker_error",
			},
		},
		Intelligence: &intelligence.Bundle{Found: true},
	})
	require.Equal(t, tradingdaymonitor.StatusFail, view.TradingStatus.ExecutionStatus)
	require.NotNil(t, view.DailyAttention)
	require.Equal(t, attention.ActionReview, view.DailyAttention.OverallAction)
	var hasTrading bool
	for _, a := range view.DailyAttention.Items {
		if a.ItemType == attention.TypeTrading {
			hasTrading = true
			require.Equal(t, attention.ActionReview, a.SuggestedAction)
			require.Contains(t, a.Reason, "broker_error")
		}
		require.NotEqual(t, "BUY", a.SuggestedAction)
		require.NotEqual(t, "SELL", a.SuggestedAction)
		require.NotEqual(t, "AUTO_ACTION", a.SuggestedAction)
	}
	require.True(t, hasTrading)
	// deprecated projection still exposes TRADING chip
	var hasLegacy bool
	for _, a := range view.AttentionItems {
		if a.Kind == attention.TypeTrading {
			hasLegacy = true
		}
	}
	require.True(t, hasLegacy)
}

func TestAssemble_Degraded(t *testing.T) {
	view := home.Assemble(home.Inputs{
		TradeDate: "2026-08-18",
		Dashboard: nil,
		Decision:  nil,
		Daily:     nil,
		Monitor:   nil,
	})
	require.Equal(t, home.QualityDegraded, view.Quality)
	require.Contains(t, view.MissingInputs, "portfolio_snapshot")
	require.Contains(t, view.MissingInputs, "portfolio_dashboard")
	require.Contains(t, view.MissingInputs, "decision_summary")
	require.Contains(t, view.MissingInputs, "daily_summary")
	require.Contains(t, view.MissingInputs, "trading_day_monitor")
	require.Contains(t, view.PortfolioSummary.Narrative, "暂无")
}

func TestAssemble_SnapshotAssetsKeepTradingFromMonitor(t *testing.T) {
	dailyPnL := 12000.0
	view := home.Assemble(home.Inputs{
		TradeDate: "2026-08-19",
		Snapshot: &readmodel.View{
			Found: true, Cash: 700_000, Equity: 712_000, MarketValue: 12_000, PositionCount: 1,
			Positions: []readmodel.PositionView{{
				StockCode: "sh600000", StockName: "浦发",
				TotalQty: 1500, AvailableQty: 1000, LockedQty: 400,
				AvgCost: 10, MarkPrice: 12, MarketValue: 18_000, PnL: 3_000,
				PositionState: positionstate.PositionStateView{
					Symbol: "sh600000", State: positionstate.S3PartialLocked,
					TotalQty: 999, AvailableQty: 1000, LockedQty: 500,
				},
			}},
		},
		Dashboard: &portfolio.PortfolioDashboardView{
			Found: true,
			Summary: portfolio.PortfolioSummary{
				Equity: 1, Cash: 1, MarketValue: 1, DailyPnL: &dailyPnL, PositionCount: 99,
			},
		},
		Decision: &decision.PortfolioDecisionSummary{OverallAttention: decision.AttentionHold, Quality: decision.QualityOK},
		Daily: &summary.DailyInvestmentSummaryView{
			Quality:          summary.QualityOK,
			PortfolioSummary: summary.PortfolioSummaryBlock{Narrative: "叙事"},
			TradingSummary:   summary.TradingSummaryBlock{Narrative: "今日执行完成"},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Morning: tradingdaymonitor.MorningSection{
				Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
			},
			Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPass, PlanID: 7},
		},
		Intelligence: &intelligence.Bundle{Found: true},
	})
	require.InDelta(t, 700_000, view.PortfolioSummary.Cash, 1e-9)
	require.InDelta(t, 712_000, view.PortfolioSummary.Equity, 1e-9)
	require.InDelta(t, 12_000, view.PortfolioSummary.MarketValue, 1e-9)
	require.Equal(t, 1, view.PortfolioSummary.PositionCount)
	require.NotNil(t, view.PortfolioSummary.DailyPnL)
	require.InDelta(t, 12000.0, *view.PortfolioSummary.DailyPnL, 1e-9)
	require.NotEqual(t, 3000.0, *view.PortfolioSummary.DailyPnL)
	require.Len(t, view.PositionStates.Positions, 1)
	ps := view.PositionStates.Positions[0]
	require.Equal(t, int64(1500), ps.TotalQty)
	require.Equal(t, int64(1000), ps.AvailableQty)
	require.Equal(t, int64(400), ps.LockedQty)
	require.NotEqual(t, ps.AvailableQty+ps.LockedQty, ps.TotalQty)
	require.Equal(t, tradingdaymonitor.StatusPass, view.TradingStatus.FreezeStatus)
	require.Equal(t, tradingdaymonitor.StatusPass, view.TradingStatus.ExecutionStatus)
	require.Equal(t, uint(7), view.TradingStatus.PlanID)
}
