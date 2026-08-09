package riskreport

import (
	"fmt"
	"math"
	"strings"
)

type dimResult struct {
	score   int
	factors []RiskFactor
	warns   []RiskWarning
	sugs    []RiskSuggestion
	ok      bool
}

func aggregate(src ReportSources) (RiskScore, []RiskFactor, []RiskWarning, []RiskSuggestion, DimensionsView, string) {
	port := buildPortfolio(src)
	pos := buildPosition(src)
	exec := buildExecution(src)
	mkt := buildMarket(src)

	factors := append([]RiskFactor{}, port.factors...)
	factors = append(factors, pos.factors...)
	factors = append(factors, exec.factors...)
	factors = append(factors, mkt.factors...)

	warns := append([]RiskWarning{}, port.warns...)
	warns = append(warns, pos.warns...)
	warns = append(warns, exec.warns...)
	warns = append(warns, mkt.warns...)

	sugs := append([]RiskSuggestion{}, port.sugs...)
	sugs = append(sugs, pos.sugs...)
	sugs = append(sugs, exec.sugs...)
	sugs = append(sugs, mkt.sugs...)

	by := map[string]int{
		DimPortfolio:  clamp(port.score),
		DimPosition:   clamp(pos.score),
		DimExecution:  clamp(exec.score),
		DimMarket:     clamp(mkt.score),
	}
	// Equal weight MVP among available dims; unavailable dims excluded from average.
	sum, n := 0, 0
	for _, d := range []dimResult{port, pos, exec, mkt} {
		if d.ok {
			sum += clamp(d.score)
			n++
		}
	}
	overall := 0
	band := BandUnknown
	quality := "UNKNOWN"
	if n > 0 {
		overall = clamp(int(math.Round(float64(sum) / float64(n))))
		band = bandOf(overall)
		quality = "OK"
		if !port.ok || !pos.ok || !exec.ok {
			quality = "PARTIAL"
		}
	}

	dims := DimensionsView{
		Portfolio: DimensionSlice{Score: by[DimPortfolio], FactorCount: len(port.factors), Available: port.ok},
		Position:  DimensionSlice{Score: by[DimPosition], FactorCount: len(pos.factors), Available: pos.ok},
		Execution: DimensionSlice{Score: by[DimExecution], FactorCount: len(exec.factors), Available: exec.ok},
		Market:    DimensionSlice{Score: by[DimMarket], FactorCount: len(mkt.factors), Available: mkt.ok},
	}
	score := RiskScore{Overall: overall, Band: band, ByDimension: by}
	return score, factors, warns, sugs, dims, quality
}

func buildPortfolio(src ReportSources) dimResult {
	if strings.ToUpper(src.PortfolioQuality) != "OK" {
		return dimResult{
			ok: false,
			warns: []RiskWarning{{
				Code:    "PORTFOLIO_DATA_UNKNOWN",
				Message: "组合风险数据不可用（账户未启用或权益未知），未编造资金。",
			}},
			sugs: []RiskSuggestion{{
				Code:    "PORTFOLIO_ENABLE_OBS",
				Message: "启用模拟盘观察账户后可生成组合风险因子。",
				Kind:    SuggestInform,
			}},
		}
	}
	out := dimResult{ok: true}
	score := 0
	if src.Concentration != nil {
		c := *src.Concentration
		if c >= 0.5 {
			score += 35
			f := factor("PORTFOLIO_CONCENTRATION", DimPortfolio, SeverityHigh,
				"单票集中度偏高", fmt.Sprintf("最大单票权重 %.1f%%", c*100), &c, "ratio", src.PositionSymbols)
			out.factors = append(out.factors, f)
			out.warns = append(out.warns, RiskWarning{
				Code: "WARN_CONCENTRATION", Message: "组合单票集中度偏高，建议复核仓位结构。", FactorCodes: []string{f.Code},
			})
			out.sugs = append(out.sugs, RiskSuggestion{
				Code: "SUG_REVIEW_CONCENTRATION", Message: "建议复核高权重持仓是否符合风险预算。", Kind: SuggestReview,
			})
		} else if c >= 0.35 {
			score += 20
			f := factor("PORTFOLIO_CONCENTRATION", DimPortfolio, SeverityWarn,
				"单票集中度偏高", fmt.Sprintf("最大单票权重 %.1f%%", c*100), &c, "ratio", nil)
			out.factors = append(out.factors, f)
		}
	}
	if src.PositionRatio != nil {
		r := *src.PositionRatio
		if r >= 0.85 {
			score += 25
			f := factor("PORTFOLIO_HIGH_POSITION_RATIO", DimPortfolio, SeverityHigh,
				"仓位偏满", fmt.Sprintf("仓位比 %.1f%%", r*100), &r, "ratio", nil)
			out.factors = append(out.factors, f)
			out.warns = append(out.warns, RiskWarning{
				Code: "WARN_HIGH_POSITION", Message: "仓位比较高，现金缓冲有限。", FactorCodes: []string{f.Code},
			})
		} else if r >= 0.7 {
			score += 12
			out.factors = append(out.factors, factor("PORTFOLIO_HIGH_POSITION_RATIO", DimPortfolio, SeverityWarn,
				"仓位偏高", fmt.Sprintf("仓位比 %.1f%%", r*100), &r, "ratio", nil))
		}
	}
	if src.Cash != nil && src.PositionRatio != nil && *src.PositionRatio > 0 {
		cash := *src.Cash
		if cash <= 0 {
			score += 20
			f := factor("PORTFOLIO_CASH_LOW", DimPortfolio, SeverityHigh,
				"现金耗尽", "可用现金≤0", &cash, "CNY", nil)
			out.factors = append(out.factors, f)
			out.sugs = append(out.sugs, RiskSuggestion{
				Code: "SUG_MONITOR_CASH", Message: "建议关注现金与后续买入能力（仅观察，不触发交易）。", Kind: SuggestMonitor,
			})
		}
	}
	out.score = score
	return out
}

func buildPosition(src ReportSources) dimResult {
	if src.HoldingRiskStates == nil && src.ExitReasonCounts == nil && src.ExitReviewStates == nil {
		return dimResult{ok: false}
	}
	// Empty maps still mean "fetched but empty book" → available with score 0.
	out := dimResult{ok: true}
	score := 0
	danger := src.HoldingRiskStates["DANGER"]
	watch := src.HoldingRiskStates["WATCH"]
	if danger > 0 {
		score += 30 + min(20, danger*5)
		syms := src.HoldingSymbols["DANGER"]
		f := factor("POSITION_DANGER", DimPosition, SeverityHigh,
			"持仓风险标签 DANGER", fmt.Sprintf("%d 只持仓处于 DANGER", danger), floatPtr(float64(danger)), "count", syms)
		out.factors = append(out.factors, f)
		out.warns = append(out.warns, RiskWarning{
			Code: "WARN_POSITION_DANGER", Message: "存在浮亏深度进入 DANGER 的持仓，建议复核命题。", FactorCodes: []string{f.Code},
		})
		out.sugs = append(out.sugs, RiskSuggestion{
			Code: "SUG_REVIEW_DANGER", Message: "建议复核 DANGER 持仓的买入假设与持有逻辑（非卖出指令）。", Kind: SuggestReview,
		})
	}
	if watch > 0 {
		score += 15 + min(10, watch*3)
		syms := src.HoldingSymbols["WATCH"]
		out.factors = append(out.factors, factor("POSITION_WATCH", DimPosition, SeverityWarn,
			"持仓风险标签 WATCH", fmt.Sprintf("%d 只持仓处于 WATCH", watch), floatPtr(float64(watch)), "count", syms))
		out.sugs = append(out.sugs, RiskSuggestion{
			Code: "SUG_MONITOR_WATCH", Message: "建议关注 WATCH 持仓浮盈变化。", Kind: SuggestMonitor,
		})
	}
	for code, n := range src.ExitReasonCounts {
		if n <= 0 {
			continue
		}
		switch code {
		case "TIME_REVIEW":
			score += 10
			out.factors = append(out.factors, factor("POSITION_TIME_REVIEW", DimPosition, SeverityWarn,
				"持有期复评", fmt.Sprintf("%d 条 TIME_REVIEW", n), floatPtr(float64(n)), "count", nil))
		case "LOSS_REVIEW":
			score += 15
			out.factors = append(out.factors, factor("POSITION_LOSS_REVIEW", DimPosition, SeverityHigh,
				"浮亏复评", fmt.Sprintf("%d 条 LOSS_REVIEW", n), floatPtr(float64(n)), "count", nil))
		case "PLAN_REVIEW":
			score += 12
			out.factors = append(out.factors, factor("POSITION_PLAN_REVIEW", DimPosition, SeverityWarn,
				"计划生命周期复评", fmt.Sprintf("%d 条 PLAN_REVIEW", n), floatPtr(float64(n)), "count", nil))
		}
	}
	if src.ExitReviewStates["REVIEW_REQUIRED"] > 0 {
		out.warns = append(out.warns, RiskWarning{
			Code:    "WARN_EXIT_REVIEW",
			Message: "存在需要重新评估持仓逻辑的条目（复评≠卖出）。",
			FactorCodes: []string{"POSITION_TIME_REVIEW", "POSITION_LOSS_REVIEW", "POSITION_PLAN_REVIEW"},
		})
		out.sugs = append(out.sugs, RiskSuggestion{
			Code: "SUG_EXIT_REVIEW", Message: "建议结合 Exit Review 摘要复核持仓假设，不自动平仓。", Kind: SuggestReview,
		})
	}
	out.score = score
	return out
}

func buildExecution(src ReportSources) dimResult {
	if !src.ExecEnabled && src.ExecTotalOrders == 0 && src.ExecDataNote == "" {
		return dimResult{ok: false}
	}
	out := dimResult{ok: true}
	score := 0
	note := strings.ToLower(src.ExecDataNote + " " + src.ExecDataSource)
	if strings.Contains(note, "fallback") || strings.Contains(note, "legacy") {
		score += 15
		out.factors = append(out.factors, factor("EXEC_DATA_DEGRADED", DimExecution, SeverityWarn,
			"执行数据降级", "Execution Summary 使用 legacy fallback 或混合数据源", nil, "", nil))
		out.warns = append(out.warns, RiskWarning{
			Code: "WARN_EXEC_DEGRADED", Message: "执行观察数据源降级，统计可能不完整。", FactorCodes: []string{"EXEC_DATA_DEGRADED"},
		})
	}
	if src.ExecTotalOrders == 0 {
		out.factors = append(out.factors, factor("EXEC_NO_ACTIVITY", DimExecution, SeverityInfo,
			"无执行活动", "当日无订单记录", nil, "", nil))
		out.sugs = append(out.sugs, RiskSuggestion{
			Code: "SUG_EXEC_NONE", Message: "当日暂无执行记录，报告仅反映持仓/组合侧。", Kind: SuggestInform,
		})
	} else {
		rejectRate := 0.0
		if src.ExecTotalOrders > 0 {
			rejectRate = float64(src.ExecFailed) / float64(src.ExecTotalOrders)
		}
		if rejectRate >= 0.3 {
			score += 30
			f := factor("EXEC_REJECT_RATE", DimExecution, SeverityHigh,
				"拒单率偏高", fmt.Sprintf("失败 %d / 总 %d（%.0f%%）", src.ExecFailed, src.ExecTotalOrders, rejectRate*100),
				&rejectRate, "ratio", nil)
			out.factors = append(out.factors, f)
			out.warns = append(out.warns, RiskWarning{
				Code: "WARN_EXEC_REJECT", Message: "执行失败比例偏高，建议复核计划就绪与成交条件。", FactorCodes: []string{f.Code},
			})
			out.sugs = append(out.sugs, RiskSuggestion{
				Code: "SUG_REVIEW_EXEC", Message: "建议复核未成交原因（只读，不重试下单）。", Kind: SuggestReview,
			})
		} else if rejectRate > 0 {
			score += 10
			out.factors = append(out.factors, factor("EXEC_REJECT_RATE", DimExecution, SeverityWarn,
				"存在拒单", fmt.Sprintf("失败 %d / 总 %d", src.ExecFailed, src.ExecTotalOrders), &rejectRate, "ratio", nil))
		}
		if src.ExecFilled > 0 && src.ExecFilled < src.ExecTotalOrders && src.ExecFailed == 0 {
			score += 8
			out.factors = append(out.factors, factor("EXEC_PARTIAL_FILL", DimExecution, SeverityInfo,
				"部分成交", fmt.Sprintf("成交 %d / 总 %d，填充率 %.0f%%", src.ExecFilled, src.ExecTotalOrders, src.ExecFillRate*100),
				floatPtr(src.ExecFillRate), "ratio", nil))
		}
	}
	out.score = score
	return out
}

func buildMarket(src ReportSources) dimResult {
	out := dimResult{ok: true}
	score := 0
	st := strings.ToUpper(strings.TrimSpace(src.MarketState))
	if st == "CLOSED" || (!src.MarketTrading && st != "") {
		score += 5
		out.factors = append(out.factors, factor("MARKET_SESSION_CLOSED", DimMarket, SeverityInfo,
			"非交易时段/休市", "当前市场会话为 "+st, nil, "", nil))
	}
	if src.MarketLevel >= 3 {
		score += 25
		lv := float64(src.MarketLevel)
		out.factors = append(out.factors, factor("MARKET_LEVEL_ELEVATED", DimMarket, SeverityHigh,
			"市场风险等级偏高", fmt.Sprintf("MarketLevel=%d", src.MarketLevel), &lv, "level", nil))
		out.warns = append(out.warns, RiskWarning{
			Code: "WARN_MARKET_LEVEL", Message: "市场风险等级偏高（复述既有状态，不改开仓开关）。", FactorCodes: []string{"MARKET_LEVEL_ELEVATED"},
		})
	} else if src.MarketLevel >= 2 {
		score += 12
		lv := float64(src.MarketLevel)
		out.factors = append(out.factors, factor("MARKET_LEVEL_ELEVATED", DimMarket, SeverityWarn,
			"市场风险等级升高", fmt.Sprintf("MarketLevel=%d", src.MarketLevel), &lv, "level", nil))
	}
	out.score = score
	return out
}

func factor(code, dim, sev, title, detail string, metric *float64, unit string, syms []string) RiskFactor {
	return RiskFactor{
		Code: code, Dimension: dim, Severity: sev, Title: title, Detail: detail,
		MetricValue: metric, MetricUnit: unit, RelatedSymbols: syms,
	}
}

func floatPtr(v float64) *float64 { return &v }

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func bandOf(score int) string {
	switch {
	case score >= 60:
		return BandHigh
	case score >= 30:
		return BandMedium
	default:
		return BandLow
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func defaultDisclaimers() []string {
	return []string{
		"本报告为只读风险观察，不构成投资建议或交易指令，不会修改或触发任何成交。",
		"Suggestions 仅为复核/关注提示，不含买入/卖出/撤单指令。",
	}
}
