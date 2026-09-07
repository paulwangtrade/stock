package attention

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/portfolio/decision"
	"go-stock/backend/portfolio/intelligence"
	"go-stock/backend/tradingdaymonitor"
)

// Build normalizes upstream read models into DailyAttentionView (pure).
func Build(in Inputs) *DailyAttentionView {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	td := strings.TrimSpace(in.TradeDate)
	if td == "" {
		td = asOf.Format("2006-01-02")
	}

	out := &DailyAttentionView{
		TradeDate:      td,
		AsOf:           asOf,
		Items:          []DailyAttention{},
		MissingInputs:  []string{},
		Quality:        QualityOK,
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
		OverallAction:  ActionHold,
	}

	if in.DecisionErr != nil || in.Decision == nil {
		out.MissingInputs = append(out.MissingInputs, "decision_summary")
		out.Quality = QualityDegraded
	}
	if in.IntelErr != nil || in.Intelligence == nil {
		out.MissingInputs = append(out.MissingInputs, "position_intelligence")
		out.Quality = QualityDegraded
	}
	if in.DailyErr != nil || in.Daily == nil {
		out.MissingInputs = append(out.MissingInputs, "daily_summary")
		out.Quality = QualityDegraded
	}
	if in.Monitor == nil {
		out.MissingInputs = append(out.MissingInputs, "trading_day_monitor")
		out.Quality = QualityDegraded
	}
	if in.Decision != nil && in.Decision.Quality == decision.QualityDegraded {
		out.Quality = QualityDegraded
		for _, m := range in.Decision.MissingInputs {
			out.MissingInputs = appendUnique(out.MissingInputs, "decision:"+m)
		}
	}
	if in.Daily != nil && strings.EqualFold(in.Daily.Quality, "DEGRADED") {
		out.Quality = QualityDegraded
		out.MissingInputs = appendUnique(out.MissingInputs, "daily_summary_degraded")
	}

	items := []DailyAttention{}
	items = append(items, fromIntelligence(in, td)...)
	items = append(items, fromDecisionOpportunity(in, td)...)
	items = append(items, fromDecisionEfficiency(in, td)...)
	items = append(items, fromDecisionPortfolioAndPretrade(in, td)...)
	items = append(items, fromDailyAndMonitor(in, td)...)

	items = dedupe(items)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].ID < items[j].ID
	})

	filtered := make([]DailyAttention, 0, len(items))
	for _, it := range items {
		it.SuggestedAction = clampAction(it.SuggestedAction)
		if it.SuggestedAction == ActionHold {
			continue
		}
		filtered = append(filtered, it)
	}
	out.Items = filtered
	out.Counts = countActions(filtered)
	out.OverallAction = aggregateOverall(filtered, in)
	out.Headline = buildHeadline(out, in)
	return out
}

func fromIntelligence(in Inputs, td string) []DailyAttention {
	if in.Intelligence == nil {
		return nil
	}
	out := []DailyAttention{}
	for _, p := range in.Intelligence.Positions {
		if strings.EqualFold(p.PositionStatus, intelligence.StatusNoPosition) {
			continue
		}
		action := ActionWatch
		sev := SeverityLow
		pri := 60
		title := "持仓关注"
		switch {
		case strings.EqualFold(p.PositionStatus, intelligence.StatusNeedReview),
			strings.EqualFold(p.RiskLevel, intelligence.RiskHigh):
			action = ActionReview
			sev = SeverityHigh
			pri = 12
			title = "持仓需复盘"
		case strings.EqualFold(p.RiskLevel, intelligence.RiskMedium),
			strings.EqualFold(p.PositionStatus, intelligence.StatusHoldingLoss):
			action = ActionWatch
			sev = SeverityMedium
			pri = 40
			title = "持仓风险留意"
		default:
			if strings.TrimSpace(p.AttentionReason) == "" {
				continue
			}
			if strings.EqualFold(p.PositionStatus, intelligence.StatusHoldingProfit) &&
				strings.EqualFold(p.RiskLevel, intelligence.RiskLow) {
				continue
			}
			action = ActionWatch
			sev = SeverityLow
			pri = 70
		}
		reason := strings.TrimSpace(p.AttentionReason)
		if reason == "" {
			reason = fmt.Sprintf("status=%s risk=%s", p.PositionStatus, p.RiskLevel)
		}
		out = append(out, DailyAttention{
			ID:              makeID(td, TypeRisk, p.StockCode, reason),
			ItemType:        TypeRisk,
			StockCode:       p.StockCode,
			StockName:       p.StockName,
			Priority:        pri,
			Title:           title,
			Reason:          reason,
			Source:          SourceIntelligence,
			Severity:        sev,
			SuggestedAction: action,
			AsOfTradeDate:   td,
		})
	}
	return out
}

func fromDecisionOpportunity(in Inputs, td string) []DailyAttention {
	if in.Decision == nil {
		return nil
	}
	opp := in.Decision.OpportunityAttention
	out := []DailyAttention{}
	action := clampAction(firstNonEmpty(opp.UserAction, opp.Status))
	if action == ActionHold && len(opp.Highlights) == 0 {
		return nil
	}
	if len(opp.Highlights) == 0 && opp.CandidateCode != "" && action != ActionHold {
		reason := "存在值得留意的候选机会（对比持仓）"
		if opp.CandidateScore != nil {
			reason = fmt.Sprintf("候选 %s 评分 %.0f，建议关注相对持仓落差", opp.CandidateCode, *opp.CandidateScore)
		}
		out = append(out, DailyAttention{
			ID:              makeID(td, TypeOpportunity, opp.CandidateCode, reason),
			ItemType:        TypeOpportunity,
			StockCode:       opp.CandidateCode,
			Priority:        priorityForAction(action, 35),
			Title:           "机会关注",
			Reason:          reason,
			Source:          SourceDecision,
			Severity:        severityForAction(action),
			SuggestedAction: action,
			AsOfTradeDate:   td,
		})
		return out
	}
	for _, h := range opp.Highlights {
		reason := strings.TrimSpace(h.Reason)
		if reason == "" {
			reason = "候选相对持仓存在评分落差"
		}
		rowAction := action
		if rowAction == ActionHold {
			rowAction = ActionWatch
		}
		out = append(out, DailyAttention{
			ID:              makeID(td, TypeOpportunity, h.HoldingCode, reason),
			ItemType:        TypeOpportunity,
			StockCode:       h.HoldingCode,
			Priority:        priorityForAction(rowAction, 32),
			Title:           "机会对比",
			Reason:          reason,
			Source:          SourceDecision,
			Severity:        severityForAction(rowAction),
			SuggestedAction: rowAction,
			AsOfTradeDate:   td,
		})
	}
	return out
}

func fromDecisionEfficiency(in Inputs, td string) []DailyAttention {
	if in.Decision == nil {
		return nil
	}
	eff := in.Decision.CapitalEfficiencyAttention
	out := []DailyAttention{}
	blockAction := clampAction(eff.Status)
	for _, e := range eff.Items {
		action := ActionWatch
		sev := SeverityMedium
		pri := 45
		if strings.EqualFold(e.EfficiencyLevel, decision.EffLow) {
			if blockAction == ActionReview {
				action = ActionReview
				sev = SeverityHigh
				pri = 28
			} else {
				action = ActionWatch
				sev = SeverityMedium
				pri = 45
			}
		}
		reason := "资金效率偏低"
		if e.EfficiencyLevel != "" {
			reason = fmt.Sprintf("资金效率 %s", e.EfficiencyLevel)
		}
		out = append(out, DailyAttention{
			ID:              makeID(td, TypeEfficiency, e.StockCode, reason),
			ItemType:        TypeEfficiency,
			StockCode:       e.StockCode,
			Priority:        pri,
			Title:           "资金效率",
			Reason:          reason,
			Source:          SourceDecision,
			Severity:        sev,
			SuggestedAction: action,
			AsOfTradeDate:   td,
		})
	}
	return out
}

func fromDecisionPortfolioAndPretrade(in Inputs, td string) []DailyAttention {
	if in.Decision == nil {
		return nil
	}
	out := []DailyAttention{}
	d := in.Decision
	overall := clampAction(d.OverallAttention)
	if overall != ActionHold && strings.TrimSpace(d.Explanation) != "" {
		out = append(out, DailyAttention{
			ID:              makeID(td, TypePortfolio, "", d.Explanation),
			ItemType:        TypePortfolio,
			Priority:        priorityForAction(overall, 25),
			Title:           "组合关注",
			Reason:          truncate(d.Explanation, 180),
			Source:          SourceDecision,
			Severity:        severityForAction(overall),
			SuggestedAction: overall,
			AsOfTradeDate:   td,
		})
	}
	risk := d.RiskAttention
	if risk.PretradeLevel != "" {
		lvl := strings.ToUpper(strings.TrimSpace(risk.PretradeLevel))
		if lvl == "BLOCKED" || lvl == "WARNING" {
			action := ActionWatch
			sev := SeverityMedium
			pri := 30
			title := "执行前风险"
			reason := "执行前检查存在警告"
			if lvl == "BLOCKED" {
				action = ActionReview
				sev = SeverityHigh
				pri = 8
				title = "执行前阻断"
				reason = "执行前检查阻断（如现金不足），建议复盘计划"
			}
			if risk.CashEnough != nil && !*risk.CashEnough {
				reason = "可用现金不足以覆盖计划所需，建议复盘"
			}
			out = append(out, DailyAttention{
				ID:              makeID(td, TypeRisk, "", reason),
				ItemType:        TypeRisk,
				Priority:        pri,
				Title:           title,
				Reason:          reason,
				Source:          SourcePretrade,
				Severity:        sev,
				SuggestedAction: action,
				AsOfTradeDate:   td,
			})
		}
	}
	return out
}

func fromDailyAndMonitor(in Inputs, td string) []DailyAttention {
	out := []DailyAttention{}

	exec := ""
	reason := ""
	planID := uint(0)
	if in.Monitor != nil {
		exec = in.Monitor.Execution.Status
		reason = in.Monitor.Execution.Reason
		planID = in.Monitor.Execution.PlanID
	} else if in.Daily != nil {
		exec = in.Daily.TradingSummary.ExecutionStatus
		reason = in.Daily.TradingSummary.ExecutionReason
		planID = in.Daily.TradingSummary.PlanID
	}
	if strings.EqualFold(exec, tradingdaymonitor.StatusFail) {
		r := strings.TrimSpace(reason)
		if r == "" {
			r = "今日模拟执行失败"
		}
		src := SourceMonitor
		if in.Monitor == nil {
			src = SourceDailySummary
		}
		out = append(out, DailyAttention{
			ID:              makeID(td, TypeTrading, "", r),
			ItemType:        TypeTrading,
			Priority:        5,
			Title:           "今日执行失败",
			Reason:          r,
			Source:          src,
			Severity:        SeverityHigh,
			SuggestedAction: ActionReview,
			RelatedPlanID:   planID,
			AsOfTradeDate:   td,
		})
	}

	if in.Daily != nil {
		for _, p := range in.Daily.PositionAttention {
			r := strings.TrimSpace(p.Reason)
			if r == "" {
				continue
			}
			out = append(out, DailyAttention{
				ID:              makeID(td, TypePosition, p.StockCode, r),
				ItemType:        TypePosition,
				StockCode:       p.StockCode,
				StockName:       p.StockName,
				Priority:        55,
				Title:           firstNonEmpty(p.StockName, p.StockCode, "持仓关注"),
				Reason:          r,
				Source:          SourceDailySummary,
				Severity:        SeverityLow,
				SuggestedAction: ActionWatch,
				AsOfTradeDate:   td,
			})
		}
		for _, focus := range in.Daily.TomorrowFocus {
			f := strings.TrimSpace(focus)
			if f == "" {
				continue
			}
			out = append(out, DailyAttention{
				ID:              makeID(td, TypeTomorrow, "", f),
				ItemType:        TypeTomorrow,
				Priority:        90,
				Title:           "明日关注",
				Reason:          f,
				Source:          SourceDailySummary,
				Severity:        SeverityInfo,
				SuggestedAction: ActionWatch,
				AsOfTradeDate:   td,
			})
		}
	}
	return out
}

// clampAction forces suggested_action into HOLD|WATCH|REVIEW.
func clampAction(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	switch s {
	case ActionHold, ActionWatch, ActionReview:
		return s
	case "BUY", "SELL", "AUTO_ACTION", "AUTO_REBALANCE", "POSSIBLE_REPLACE", "FAIL", "BLOCKED", "NEED_REVIEW":
		return ActionReview
	case "WARNING", "INFO", "PENDING":
		return ActionWatch
	case "":
		return ActionHold
	default:
		return ActionWatch
	}
}

func severityForAction(action string) string {
	switch clampAction(action) {
	case ActionReview:
		return SeverityHigh
	case ActionWatch:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}

func priorityForAction(action string, base int) int {
	switch clampAction(action) {
	case ActionReview:
		if base > 25 {
			return 22
		}
		return base
	case ActionWatch:
		if base < 30 {
			return 40
		}
		return base
	default:
		return 95
	}
}

func aggregateOverall(items []DailyAttention, in Inputs) string {
	hasReview, hasWatch := false, false
	for _, it := range items {
		switch it.SuggestedAction {
		case ActionReview:
			hasReview = true
		case ActionWatch:
			hasWatch = true
		}
	}
	if hasReview {
		return ActionReview
	}
	if hasWatch {
		return ActionWatch
	}
	if in.Decision != nil {
		return clampAction(in.Decision.OverallAttention)
	}
	return ActionHold
}

func buildHeadline(out *DailyAttentionView, in Inputs) string {
	switch out.OverallAction {
	case ActionReview:
		if len(out.Items) > 0 && out.Items[0].Title != "" {
			return "今日建议复盘：" + out.Items[0].Title
		}
		return "今日建议复盘"
	case ActionWatch:
		return "今日建议留意若干事项"
	default:
		if in.Decision != nil && strings.TrimSpace(in.Decision.Explanation) != "" {
			return truncate(in.Decision.Explanation, 80)
		}
		return "今日暂无强制关注事项"
	}
}

func countActions(items []DailyAttention) AttentionCounts {
	c := AttentionCounts{Total: len(items)}
	for _, it := range items {
		switch it.SuggestedAction {
		case ActionReview:
			c.Review++
		case ActionWatch:
			c.Watch++
		case ActionHold:
			c.Hold++
		}
	}
	return c
}

func dedupe(items []DailyAttention) []DailyAttention {
	seen := map[string]int{}
	out := []DailyAttention{}
	for _, it := range items {
		key := it.ItemType + "|" + strings.ToLower(it.StockCode) + "|" + it.Reason
		if idx, ok := seen[key]; ok {
			prev := out[idx]
			if actionRank(it.SuggestedAction) > actionRank(prev.SuggestedAction) ||
				(actionRank(it.SuggestedAction) == actionRank(prev.SuggestedAction) && it.Priority < prev.Priority) {
				out[idx] = it
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, it)
	}
	return out
}

func actionRank(a string) int {
	switch clampAction(a) {
	case ActionReview:
		return 3
	case ActionWatch:
		return 2
	default:
		return 1
	}
}

func makeID(td, typ, code, reason string) string {
	r := reason
	if len(r) > 48 {
		r = r[:48]
	}
	r = strings.ReplaceAll(r, "|", "/")
	return td + "|" + typ + "|" + strings.ToLower(strings.TrimSpace(code)) + "|" + r
}

func appendUnique(ss []string, v string) []string {
	for _, s := range ss {
		if s == v {
			return ss
		}
	}
	return append(ss, v)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
