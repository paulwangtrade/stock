package summary_test

import (
	"errors"
	"testing"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"
	"go-stock/backend/tradingevent"

	"github.com/stretchr/testify/require"
)

func TestBuild_NormalTradingDay(t *testing.T) {
	pnl := 16000.0
	dash := &portfolio.PortfolioDashboardView{
		Found: true,
		Summary: portfolio.PortfolioSummary{
			Equity: 2_060_000, Cash: 500_000, MarketValue: 1_560_000,
			DailyPnL: &pnl, PositionCount: 5,
		},
	}
	mon := &tradingdaymonitor.TradingDayMonitorView{
		Morning: tradingdaymonitor.MorningSection{
			Materialize: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
			Approve:     tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
			Freeze:      tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
		},
		Execution: tradingdaymonitor.ExecutionSection{
			Session: "A", Status: tradingdaymonitor.StatusPass, PlanID: 12, Reason: "ok",
		},
		Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPass},
	}
	intel := &intelligence.Bundle{
		Found: true,
		Positions: []intelligence.PositionIntelligenceView{
			{StockCode: "sz000001", StockName: "平安", PositionStatus: intelligence.StatusHoldingProfit, RiskLevel: intelligence.RiskLow},
		},
	}
	view := summary.Build(summary.Inputs{
		TradeDate: "2026-08-17", AsOf: time.Now(),
		Dashboard: dash, Monitor: mon, Intelligence: intel,
	})
	require.Equal(t, summary.QualityOK, view.Quality)
	require.Contains(t, view.PortfolioSummary.Narrative, "持仓 5")
	require.Contains(t, view.TradingSummary.Narrative, "执行完成")
	require.Equal(t, tradingdaymonitor.StatusPass, view.TradingSummary.SettlementStatus)
	require.Contains(t, view.TomorrowFocus, "暂无关注事项")
}

func TestBuild_NoExecution(t *testing.T) {
	mon := &tradingdaymonitor.TradingDayMonitorView{
		Morning: tradingdaymonitor.MorningSection{
			Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusUnknown, Reason: "PLAN_NOT_FOUND"},
		},
		Execution:  tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPending},
		Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPending},
	}
	view := summary.Build(summary.Inputs{
		TradeDate: "2026-08-18",
		Dashboard: &portfolio.PortfolioDashboardView{Found: false},
		Monitor:   mon,
		Intelligence: &intelligence.Bundle{Found: false, Positions: nil},
	})
	require.Contains(t, view.TradingSummary.Narrative, "未执行")
	require.Contains(t, view.TomorrowFocus, "存在未完成计划")
}

func TestBuild_ExecutionFailed(t *testing.T) {
	mon := &tradingdaymonitor.TradingDayMonitorView{
		Morning: tradingdaymonitor.MorningSection{
			Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
		},
		Execution: tradingdaymonitor.ExecutionSection{
			Status: tradingdaymonitor.StatusFail, Reason: "broker_error", PlanID: 9, Session: "A",
		},
		Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPending},
	}
	view := summary.Build(summary.Inputs{Monitor: mon, Dashboard: &portfolio.PortfolioDashboardView{Found: true}})
	require.Contains(t, view.TradingSummary.Narrative, "执行失败")
	require.Contains(t, view.TradingSummary.Narrative, "broker_error")
	require.Equal(t, "broker_error", view.TradingSummary.ExecutionReason)
}

func TestBuild_NoPositions(t *testing.T) {
	view := summary.Build(summary.Inputs{
		Dashboard: &portfolio.PortfolioDashboardView{
			Found: true,
			Summary: portfolio.PortfolioSummary{Equity: 1_000_000, Cash: 1_000_000, PositionCount: 0},
		},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusSkip, Reason: "skipped_no_frozen_plan"},
			Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPass},
			Morning: tradingdaymonitor.MorningSection{
				Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
			},
		},
		Intelligence: &intelligence.Bundle{Found: true, Positions: []intelligence.PositionIntelligenceView{}},
	})
	require.Equal(t, 0, view.PortfolioSummary.PositionCount)
	require.Empty(t, view.PositionAttention)
	require.Contains(t, view.RiskSummary.Narrative, "暂无")
}

func TestBuild_IntelligenceDegraded(t *testing.T) {
	view := summary.Build(summary.Inputs{
		Dashboard: &portfolio.PortfolioDashboardView{Found: true},
		Monitor: &tradingdaymonitor.TradingDayMonitorView{
			Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPass},
			Morning: tradingdaymonitor.MorningSection{
				Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass},
			},
			Settlement: tradingdaymonitor.SettlementSection{Status: tradingdaymonitor.StatusPass},
		},
		Intelligence: nil,
		IntelErr:     errors.New("boom"),
	})
	require.Equal(t, summary.QualityDegraded, view.Quality)
	require.True(t, view.RiskSummary.Degraded)
	require.Contains(t, view.MissingInputs, "position_intelligence")
	require.Contains(t, view.RiskSummary.Narrative, "降级")
}

func TestBuild_AttentionList(t *testing.T) {
	intel := &intelligence.Bundle{
		Found: true,
		Positions: []intelligence.PositionIntelligenceView{
			{
				StockCode: "sz000002", StockName: "万科", PositionStatus: intelligence.StatusNeedReview,
				RiskLevel: intelligence.RiskHigh, AttentionReason: "当前持仓亏损，风险升高，建议复盘（未触发自动退出）",
			},
		},
	}
	view := summary.Build(summary.Inputs{
		Dashboard:    &portfolio.PortfolioDashboardView{Found: true},
		Monitor:      &tradingdaymonitor.TradingDayMonitorView{Execution: tradingdaymonitor.ExecutionSection{Status: tradingdaymonitor.StatusPass}, Morning: tradingdaymonitor.MorningSection{Freeze: tradingdaymonitor.StepStatus{Status: tradingdaymonitor.StatusPass}}},
		Intelligence: intel,
	})
	require.Equal(t, 1, view.RiskSummary.AttentionCount)
	require.Len(t, view.PositionAttention, 1)
	require.Equal(t, "sz000002", view.PositionAttention[0].StockCode)
	require.Contains(t, view.TomorrowFocus, "存在持仓风险关注")
}

func TestService_UsesTradingEventViaMonitor(t *testing.T) {
	tradingevent.ResetBufferForTest()
	td := "2026-08-19"
	now := time.Date(2026, 8, 19, 15, 10, 0, 0, time.Local)
	tradingevent.EmitExecution(tradingevent.EventExecutionFailed, td, "A", tradingevent.StatusFail, "NO_FROZEN_PLAN", 0, now, "")
	mon := tradingdaymonitor.Build(tradingdaymonitor.Options{TradeDate: td, AsOf: now, SkipReadiness: true})
	view := summary.Build(summary.Inputs{
		TradeDate: td,
		Monitor:   mon,
		Dashboard: &portfolio.PortfolioDashboardView{Found: false},
		Intelligence: &intelligence.Bundle{},
	})
	require.Equal(t, tradingdaymonitor.StatusFail, view.TradingSummary.ExecutionStatus)
	require.Contains(t, view.TradingSummary.Narrative, "NO_FROZEN_PLAN")
}
