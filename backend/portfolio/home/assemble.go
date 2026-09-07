package home

import (
	"strings"
	"time"

	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/portfolio/readmodel"
	"go-stock/backend/portfolio/summary"
	"go-stock/backend/tradingdaymonitor"
)

// Assemble builds InvestmentHomeView from already-loaded read models (no recompute).
func Assemble(in Inputs) *InvestmentHomeView {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	td := strings.TrimSpace(in.TradeDate)
	if td == "" {
		td = asOf.Format("2006-01-02")
	}
	out := &InvestmentHomeView{
		TradeDate:      td,
		AsOf:           asOf,
		AttentionItems: []AttentionItem{},
		MissingInputs:  []string{},
		Quality:        QualityOK,
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}

	out.PortfolioSummary = mapPortfolio(in.Snapshot, in.Dashboard, in.Daily)
	if in.Snapshot == nil && in.Dashboard == nil {
		out.MissingInputs = append(out.MissingInputs, "portfolio_snapshot")
		out.Quality = QualityDegraded
	}
	if in.Dashboard == nil {
		out.MissingInputs = append(out.MissingInputs, "portfolio_dashboard")
		out.Quality = QualityDegraded
	}

	out.DecisionSummary = in.Decision
	if in.Decision == nil {
		out.MissingInputs = append(out.MissingInputs, "decision_summary")
		out.Quality = QualityDegraded
	}

	out.DailySummary = in.Daily
	if in.Daily == nil {
		out.MissingInputs = append(out.MissingInputs, "daily_summary")
		out.Quality = QualityDegraded
	} else if in.Daily.Quality == summary.QualityDegraded {
		out.Quality = QualityDegraded
	}

	out.TradingStatus = mapTrading(in.Monitor, in.Daily, in.TradingNarrative)
	if in.Monitor == nil {
		out.MissingInputs = append(out.MissingInputs, "trading_day_monitor")
		out.Quality = QualityDegraded
	}

	out.DailyAttention = resolveDailyAttention(in, td, asOf)
	if out.DailyAttention != nil {
		if out.DailyAttention.Quality == attention.QualityDegraded {
			out.Quality = QualityDegraded
			for _, m := range out.DailyAttention.MissingInputs {
				out.MissingInputs = appendUnique(out.MissingInputs, "attention:"+m)
			}
		}
		out.AttentionItems = projectDeprecatedAttention(out.DailyAttention)
	}

	out.PositionStates = resolvePositionStates(in, td, asOf)

	return out
}

func resolvePositionStates(in Inputs, td string, asOf time.Time) *positionstate.Bundle {
	if in.PositionStates != nil {
		return in.PositionStates
	}
	if in.Snapshot != nil {
		return bundleFromReadModel(in.Snapshot, td, asOf)
	}
	return positionstate.NewService(nil).Evaluate(positionstate.Query{TradeDate: td, AsOf: asOf})
}

func bundleFromReadModel(snap *readmodel.View, td string, asOf time.Time) *positionstate.Bundle {
	out := &positionstate.Bundle{
		TradeDate:      td,
		AsOf:           asOf,
		Positions:      []positionstate.PositionStateView{},
		DataSourceNote: "Investment Home · PositionState nested from Portfolio Snapshot; qty from paper_sim_positions",
		Disclaimer:     disclaimer,
	}
	if snap == nil || !snap.Found {
		return out
	}
	for _, p := range snap.Positions {
		if p.TotalQty <= 0 {
			continue
		}
		ps := p.PositionState
		ps.Symbol = strings.TrimSpace(p.StockCode)
		// Ledger qty is authoritative; do not let calculator-normalized fields replace the row.
		ps.TotalQty = p.TotalQty
		ps.AvailableQty = p.AvailableQty
		ps.LockedQty = p.LockedQty
		out.Positions = append(out.Positions, ps)
	}
	return out
}

func mapPortfolio(snap *readmodel.View, dash *portfolio.PortfolioDashboardView, daily *summary.DailyInvestmentSummaryView) HomePortfolioSummary {
	h := HomePortfolioSummary{}
	if snap != nil {
		h.Found = snap.Found
		h.Equity = snap.Equity
		h.Cash = snap.Cash
		h.MarketValue = snap.MarketValue
		h.PositionCount = snap.PositionCount
	} else if dash != nil {
		h.Found = dash.Found
		h.Equity = dash.Summary.Equity
		h.Cash = dash.Summary.Cash
		h.MarketValue = dash.Summary.MarketValue
		h.PositionCount = dash.Summary.PositionCount
	}
	// 今日收益: Snapshot 无日报字段。只用 Dashboard daily_pnl，禁止用浮盈代替。
	if dash != nil {
		h.DailyPnL = dash.Summary.DailyPnL
	}
	if daily != nil {
		h.Narrative = daily.PortfolioSummary.Narrative
		if snap == nil && dash == nil {
			h.Equity = daily.PortfolioSummary.Equity
			h.Cash = daily.PortfolioSummary.Cash
			h.MarketValue = daily.PortfolioSummary.MarketValue
			h.DailyPnL = daily.PortfolioSummary.DailyPnL
			h.PositionCount = daily.PortfolioSummary.PositionCount
			h.Found = daily.PortfolioSummary.PositionCount > 0 || daily.PortfolioSummary.Equity > 0
		}
	}
	if h.Narrative == "" && !h.Found {
		h.Narrative = "暂无组合数据"
	}
	return h
}

func resolveDailyAttention(in Inputs, td string, asOf time.Time) *attention.DailyAttentionView {
	if in.DailyAttention != nil {
		return in.DailyAttention
	}
	return attention.Build(attention.Inputs{
		TradeDate:    td,
		AsOf:         asOf,
		Decision:     in.Decision,
		Intelligence: in.Intelligence,
		IntelErr:     in.IntelErr,
		Daily:        in.Daily,
		Monitor:      in.Monitor,
	})
}

// projectDeprecatedAttention maps J.4 items → legacy AttentionItem chips.
func projectDeprecatedAttention(v *attention.DailyAttentionView) []AttentionItem {
	if v == nil {
		return []AttentionItem{}
	}
	out := make([]AttentionItem, 0, len(v.Items))
	for _, it := range v.Items {
		level := it.SuggestedAction
		if it.ItemType == attention.TypeTrading && level == attention.ActionReview {
			level = "FAIL" // legacy chip used FAIL for execution failures
		}
		out = append(out, AttentionItem{
			Kind:      it.ItemType,
			StockCode: it.StockCode,
			Title:     it.Title,
			Detail:    it.Reason,
			Level:     level,
		})
	}
	return out
}

func mapTrading(mon *tradingdaymonitor.TradingDayMonitorView, daily *summary.DailyInvestmentSummaryView, overrideNarrative string) HomeTradingStatus {
	t := HomeTradingStatus{
		MaterializeStatus: tradingdaymonitor.StatusPending,
		ApproveStatus:     tradingdaymonitor.StatusPending,
		FreezeStatus:      tradingdaymonitor.StatusPending,
		ExecutionStatus:   tradingdaymonitor.StatusPending,
		SettlementStatus:  tradingdaymonitor.StatusPending,
	}
	if mon != nil {
		t.MaterializeStatus = mon.Morning.Materialize.Status
		t.ApproveStatus = mon.Morning.Approve.Status
		t.FreezeStatus = mon.Morning.Freeze.Status
		t.ExecutionStatus = mon.Execution.Status
		t.SettlementStatus = mon.Settlement.Status
		t.Session = mon.Execution.Session
		t.PlanID = mon.Execution.PlanID
		t.ExecutionReason = mon.Execution.Reason
	}
	if daily != nil {
		t.Narrative = daily.TradingSummary.Narrative
		if mon == nil {
			ts := daily.TradingSummary
			t.MaterializeStatus = ts.MaterializeStatus
			t.ApproveStatus = ts.ApproveStatus
			t.FreezeStatus = ts.FreezeStatus
			t.ExecutionStatus = ts.ExecutionStatus
			t.SettlementStatus = ts.SettlementStatus
			t.Session = ts.Session
			t.PlanID = ts.PlanID
			t.ExecutionReason = ts.ExecutionReason
		}
	}
	if overrideNarrative != "" {
		t.Narrative = overrideNarrative
	}
	if t.Narrative == "" && mon == nil && daily == nil {
		t.Narrative = "交易日状态不可用"
	}
	return t
}

func appendUnique(ss []string, v string) []string {
	for _, s := range ss {
		if s == v {
			return ss
		}
	}
	return append(ss, v)
}
