package summary

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/tradingdaymonitor"
)

const (
	dataSourceNote = "Daily Investment Summary v0 · Dashboard + Day Monitor + Position Intelligence; rule templates; no LLM; no trade writes"
	disclaimer     = "每日投资摘要（规则生成）。非投资建议，不生成买卖指令，不修改交易状态。"

	QualityOK       = "OK"
	QualityDegraded = "DEGRADED"
)

// Inputs holds already-loaded read models (tests inject; Service loads when nil).
type Inputs struct {
	TradeDate    string
	AsOf         time.Time
	Dashboard    *portfolio.PortfolioDashboardView
	Monitor      *tradingdaymonitor.TradingDayMonitorView
	Intelligence *intelligence.Bundle
	// IntelErr marks Position Intelligence failure → degrade risk/attention, do not fail summary.
	IntelErr error
}

// Build assembles DailyInvestmentSummaryView from read-only inputs (pure).
func Build(in Inputs) *DailyInvestmentSummaryView {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	td := strings.TrimSpace(in.TradeDate)
	if td == "" {
		td = asOf.Format("2006-01-02")
	}

	out := &DailyInvestmentSummaryView{
		TradeDate:         td,
		AsOf:              asOf,
		PositionAttention: []PositionAttentionItem{},
		TomorrowFocus:     []string{},
		Quality:           QualityOK,
		MissingInputs:     []string{},
		DataSourceNote:    dataSourceNote,
		Disclaimer:        disclaimer,
	}

	out.PortfolioSummary = buildPortfolio(in.Dashboard)
	if in.Dashboard == nil {
		out.MissingInputs = append(out.MissingInputs, "portfolio_dashboard")
		out.Quality = QualityDegraded
	}

	out.TradingSummary = buildTrading(in.Monitor)
	if in.Monitor == nil {
		out.MissingInputs = append(out.MissingInputs, "trading_day_monitor")
		out.Quality = QualityDegraded
	}

	risk, attn := buildRiskAndAttention(in.Intelligence, in.IntelErr)
	out.RiskSummary = risk
	out.PositionAttention = attn
	if risk.Degraded {
		out.MissingInputs = append(out.MissingInputs, "position_intelligence")
		out.Quality = QualityDegraded
	}

	out.TomorrowFocus = buildTomorrowFocus(out)
	return out
}

func buildPortfolio(dash *portfolio.PortfolioDashboardView) PortfolioSummaryBlock {
	b := PortfolioSummaryBlock{Narrative: "组合数据不可用"}
	if dash == nil {
		return b
	}
	s := dash.Summary
	b.Equity = s.Equity
	b.Cash = s.Cash
	b.MarketValue = s.MarketValue
	b.DailyPnL = s.DailyPnL
	b.PositionCount = s.PositionCount
	if !dash.Found {
		b.Narrative = "暂无 Paper 账户或持仓"
		return b
	}
	pnlPart := "今日盈亏不可比"
	if s.DailyPnL != nil {
		pct := 0.0
		if s.Equity > 0 && *s.DailyPnL != 0 {
			// approximate vs prior equity ≈ equity - daily_pnl
			prior := s.Equity - *s.DailyPnL
			if prior > 0 {
				pct = *s.DailyPnL / prior * 100
			}
		}
		pnlPart = fmt.Sprintf("今日盈亏 %+.2f（约 %+.2f%%）", *s.DailyPnL, pct)
	}
	b.Narrative = fmt.Sprintf("今日组合权益 %.0f，持仓 %d 只，%s", s.Equity, s.PositionCount, pnlPart)
	return b
}

func buildTrading(mon *tradingdaymonitor.TradingDayMonitorView) TradingSummaryBlock {
	b := TradingSummaryBlock{
		MaterializeStatus: tradingdaymonitor.StatusPending,
		ApproveStatus:     tradingdaymonitor.StatusPending,
		FreezeStatus:      tradingdaymonitor.StatusPending,
		ExecutionStatus:   tradingdaymonitor.StatusPending,
		SettlementStatus:  tradingdaymonitor.StatusPending,
		Narrative:         "交易日链路数据不可用",
	}
	if mon == nil {
		return b
	}
	b.MaterializeStatus = mon.Morning.Materialize.Status
	b.ApproveStatus = mon.Morning.Approve.Status
	b.FreezeStatus = mon.Morning.Freeze.Status
	b.ExecutionStatus = mon.Execution.Status
	b.SettlementStatus = mon.Settlement.Status
	b.Session = mon.Execution.Session
	b.PlanID = mon.Execution.PlanID
	b.ExecutionReason = mon.Execution.Reason

	switch mon.Execution.Status {
	case tradingdaymonitor.StatusPass:
		b.Narrative = fmt.Sprintf("今日执行完成（session=%s，plan_id=%d）", emptyDash(mon.Execution.Session), mon.Execution.PlanID)
		if mon.Settlement.Status == tradingdaymonitor.StatusPass {
			b.Narrative += "；结算已完成"
		}
	case tradingdaymonitor.StatusFail:
		reason := mon.Execution.Reason
		if reason == "" {
			reason = "EXECUTION_FAILED"
		}
		b.Narrative = fmt.Sprintf("今日执行失败，原因：%s", reason)
	case tradingdaymonitor.StatusSkip:
		reason := mon.Execution.Reason
		if reason == "" {
			reason = "SKIPPED"
		}
		b.Narrative = fmt.Sprintf("今日未执行（跳过），原因：%s", reason)
	case tradingdaymonitor.StatusPending:
		if mon.Morning.Freeze.Status != tradingdaymonitor.StatusPass {
			reason := mon.Morning.Freeze.Reason
			if reason == "" {
				reason = "NO_FROZEN_PLAN"
			}
			b.Narrative = fmt.Sprintf("今日未执行，原因：%s", reason)
		} else {
			b.Narrative = "今日冻结已就绪，执行尚未发生或未观测到事件"
		}
	default:
		b.Narrative = fmt.Sprintf("执行状态 %s", mon.Execution.Status)
	}
	return b
}

func buildRiskAndAttention(bundle *intelligence.Bundle, intelErr error) (RiskSummaryBlock, []PositionAttentionItem) {
	attn := []PositionAttentionItem{}
	risk := RiskSummaryBlock{
		RiskLevel: intelligence.RiskUnknown,
		Narrative: "暂无持仓风险关注",
	}
	if intelErr != nil {
		risk.Degraded = true
		risk.Narrative = "持仓智能层暂不可用，风险摘要已降级"
		return risk, attn
	}
	if bundle == nil {
		risk.Degraded = true
		risk.Narrative = "持仓智能数据缺失，风险摘要已降级"
		return risk, attn
	}

	worst := intelligence.RiskLow
	var topReason string
	for _, p := range bundle.Positions {
		if p.PositionStatus == intelligence.StatusNoPosition {
			continue
		}
		worst = worseRisk(worst, p.RiskLevel)
		if isAttention(p) {
			risk.AttentionCount++
			item := PositionAttentionItem{
				StockCode: p.StockCode,
				StockName: p.StockName,
				Reason:    p.AttentionReason,
				Status:    p.PositionStatus,
			}
			attn = append(attn, item)
			if topReason == "" {
				topReason = p.AttentionReason
			}
		}
	}
	if risk.AttentionCount == 0 && len(bundle.Positions) == 0 {
		worst = intelligence.RiskUnknown
	}
	risk.RiskLevel = worst
	risk.HighestAttentionReason = topReason
	if risk.AttentionCount == 0 {
		risk.Narrative = "暂无持仓风险关注"
	} else {
		risk.Narrative = fmt.Sprintf("%d 只持仓需要关注：%s", risk.AttentionCount, topReason)
	}
	return risk, attn
}

func isAttention(p intelligence.PositionIntelligenceView) bool {
	switch p.PositionStatus {
	case intelligence.StatusNeedReview, intelligence.StatusHoldingLoss, intelligence.StatusNewPosition:
		return true
	default:
		return p.RiskLevel == intelligence.RiskHigh || p.RiskLevel == intelligence.RiskMedium
	}
}

func worseRisk(a, b string) string {
	rank := map[string]int{
		intelligence.RiskUnknown: 0,
		intelligence.RiskLow:     1,
		intelligence.RiskMedium:  2,
		intelligence.RiskHigh:    3,
	}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func buildTomorrowFocus(sum *DailyInvestmentSummaryView) []string {
	focus := []string{}
	ts := sum.TradingSummary
	planOpen := ts.FreezeStatus == tradingdaymonitor.StatusPending ||
		ts.FreezeStatus == tradingdaymonitor.StatusUnknown ||
		ts.FreezeStatus == tradingdaymonitor.StatusFail ||
		ts.ExecutionStatus == tradingdaymonitor.StatusPending
	if planOpen {
		focus = append(focus, "存在未完成计划")
	}
	if sum.RiskSummary.AttentionCount > 0 {
		focus = append(focus, "存在持仓风险关注")
	}
	if len(focus) == 0 {
		focus = append(focus, "暂无关注事项")
	}
	return focus
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
