package decision

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	scoreGapReplace = 15.0
	scoreGapReview  = 7.5
	minWeightForOpp = 0.05
	weightRef       = 0.10
)

// Build assembles PortfolioDecisionSummary (pure). Never emits BUY/SELL/AUTO_REBALANCE.
func Build(in Inputs) *PortfolioDecisionSummary {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	out := &PortfolioDecisionSummary{
		TradeDate:      strings.TrimSpace(in.TradeDate),
		AsOf:           asOf,
		MissingInputs:  []string{},
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
		Quality:        QualityOK,
	}
	if in.MissingOpportunityPackage {
		out.MissingInputs = append(out.MissingInputs, "opportunity_analysis")
		out.Quality = QualityDegraded
	}
	if in.MissingEfficiencyPackage {
		out.MissingInputs = append(out.MissingInputs, "capital_efficiency")
		out.Quality = QualityDegraded
	}

	out.PortfolioHealth = buildHealth(in)
	out.CapitalEfficiencyAttention = buildEfficiency(in)
	out.OpportunityAttention = buildOpportunity(in, out.CapitalEfficiencyAttention)
	out.RiskAttention = buildRisk(in)
	out.OverallAttention = aggregate(
		out.PortfolioHealth.Status,
		out.OpportunityAttention.Status,
		out.CapitalEfficiencyAttention.Status,
		out.RiskAttention.Status,
	)
	out.Explanation = buildExplanation(out)
	return out
}

func buildHealth(in Inputs) PortfolioHealth {
	h := PortfolioHealth{
		Equity:        in.Equity,
		Cash:          in.Cash,
		PositionCount: len(in.Holdings),
		Found:         in.AccountFound,
		Notes:         []string{},
		Status:        AttentionHold,
	}
	if !in.AccountFound {
		h.Status = AttentionReview
		h.Notes = append(h.Notes, "暂无 Paper 账户")
		return h
	}
	needReview := 0
	for _, p := range in.Holdings {
		if strings.EqualFold(p.PositionStatus, "NEED_REVIEW") {
			needReview++
		}
	}
	if in.PreTrade.Present && strings.EqualFold(in.PreTrade.Level, "BLOCKED") {
		h.Status = AttentionReview
		h.Notes = append(h.Notes, "执行前现金或约束阻断")
	}
	if needReview >= 2 {
		h.Status = AttentionReview
		h.Notes = append(h.Notes, fmt.Sprintf("%d 只持仓需复盘", needReview))
	} else if needReview == 1 && h.Status != AttentionReview {
		h.Status = AttentionWatch
		h.Notes = append(h.Notes, "存在需复盘持仓")
	}
	if in.Equity > 0 && in.Cash/in.Equity < 0.05 && h.Status == AttentionHold {
		h.Status = AttentionWatch
		h.Notes = append(h.Notes, "现金占比偏低")
	}
	if len(in.Holdings) == 0 && h.Status == AttentionHold {
		h.Notes = append(h.Notes, "当前无持仓")
	}
	return h
}

func buildEfficiency(in Inputs) CapitalEfficiencyAttention {
	out := CapitalEfficiencyAttention{
		Status: AttentionHold,
		Items:  []EfficiencyItem{},
	}
	var lowCount int
	var hasNeedReviewLow bool
	for _, h := range in.Holdings {
		item := EfficiencyItem{
			StockCode:       h.StockCode,
			CapitalUsed:     h.CapitalUsed,
			PortfolioWeight: h.Weight,
			InvestmentScore: h.InvestmentScore,
		}
		if h.InvestmentScore != nil {
			eff := efficiencyScore(*h.InvestmentScore, h.Weight)
			item.EfficiencyScore = &eff
			item.EfficiencyLevel = efficiencyLevel(eff)
			switch item.EfficiencyLevel {
			case EffLow:
				lowCount++
				if strings.EqualFold(h.PositionStatus, "NEED_REVIEW") {
					hasNeedReviewLow = true
				}
				out.Items = append(out.Items, item)
			}
		}
	}
	out.LowEfficiencyCount = lowCount
	switch {
	case lowCount > 0 && hasNeedReviewLow:
		out.Status = AttentionReview
	case lowCount > 0:
		out.Status = AttentionWatch
	default:
		// MEDIUM/HIGH without LOW → no capital-efficiency attention escalate
		out.Status = AttentionHold
	}
	return out
}

func efficiencyScore(invScore, weight float64) float64 {
	if weight < 0 {
		weight = 0
	}
	burden := weight / weightRef * 100
	if burden > 100 {
		burden = 100
	}
	v := invScore - 0.5*burden
	return clamp100(v)
}

func efficiencyLevel(eff float64) string {
	switch {
	case eff >= 70:
		return EffHigh
	case eff >= 45:
		return EffMedium
	default:
		return EffLow
	}
}

func buildOpportunity(in Inputs, eff CapitalEfficiencyAttention) OpportunityAttention {
	out := OpportunityAttention{
		Status:     AttentionHold,
		Highlights: []OpportunityHighlight{},
		UserAction: AttentionHold,
		SourceAction: AttentionHold,
	}
	if !in.Candidate.Present || in.Candidate.Score == nil {
		out.Status = AttentionHold
		out.UserAction = AttentionHold
		return out
	}
	out.CandidateCode = in.Candidate.Code
	out.CandidateName = strings.TrimSpace(in.Candidate.Name)
	out.CandidateScore = in.Candidate.Score
	c := *in.Candidate.Score

	bestGap := 0.0
	source := AttentionHold
	for _, h := range in.Holdings {
		if h.InvestmentScore == nil {
			continue
		}
		gap := c - *h.InvestmentScore
		if gap < scoreGapReview {
			continue
		}
		if h.Weight < minWeightForOpp && h.CapitalUsed <= 0 {
			continue
		}
		reason := fmt.Sprintf("候选分 %.0f 高于持仓分 %.0f（分差 %.0f）", c, *h.InvestmentScore, gap)
		out.Highlights = append(out.Highlights, OpportunityHighlight{
			HoldingCode:  h.StockCode,
			HoldingScore: h.InvestmentScore,
			Reason:       reason,
		})
		if gap > bestGap {
			bestGap = gap
		}
		if gap >= scoreGapReplace && (h.Weight >= minWeightForOpp || h.CapitalUsed > 0) {
			source = "POSSIBLE_REPLACE"
		} else if source != "POSSIBLE_REPLACE" {
			source = AttentionReview
		}
	}
	out.SourceAction = source
	switch source {
	case "POSSIBLE_REPLACE":
		out.UserAction = AttentionReview
		out.Status = AttentionReview
		// Elevate if also low efficiency overlap
		if eff.LowEfficiencyCount > 0 {
			out.Status = AttentionReview
		}
	case AttentionReview:
		out.UserAction = AttentionReview
		out.Status = AttentionReview
		if bestGap < scoreGapReplace {
			out.UserAction = AttentionWatch
			out.Status = AttentionWatch
		}
	default:
		out.UserAction = AttentionHold
		out.Status = AttentionHold
	}
	return out
}

func buildRisk(in Inputs) RiskAttention {
	out := RiskAttention{
		Status:            AttentionHold,
		IntelligenceItems: []RiskIntelItem{},
	}
	if in.PreTrade.Present {
		out.PretradeLevel = in.PreTrade.Level
		ce := in.PreTrade.CashEnough
		out.CashEnough = &ce
		out.Concentration = in.PreTrade.Concentration
	}
	for _, h := range in.Holdings {
		attn := false
		if strings.EqualFold(h.PositionStatus, "NEED_REVIEW") ||
			strings.EqualFold(h.RiskLevel, "HIGH") ||
			strings.EqualFold(h.PositionStatus, "HOLDING_LOSS") ||
			strings.EqualFold(h.RiskLevel, "MEDIUM") {
			attn = true
		}
		if !attn && strings.TrimSpace(h.AttentionReason) == "" {
			continue
		}
		if strings.EqualFold(h.PositionStatus, "HOLDING_PROFIT") && strings.EqualFold(h.RiskLevel, "LOW") {
			continue
		}
		out.IntelligenceItems = append(out.IntelligenceItems, RiskIntelItem{
			StockCode:       h.StockCode,
			PositionStatus:  h.PositionStatus,
			RiskLevel:       h.RiskLevel,
			AttentionReason: h.AttentionReason,
		})
	}

	pre := strings.ToUpper(strings.TrimSpace(out.PretradeLevel))
	conc := strings.ToUpper(strings.TrimSpace(out.Concentration))
	hasNeedReview, hasHigh, hasLossOrMed := false, false, false
	for _, it := range out.IntelligenceItems {
		if strings.EqualFold(it.PositionStatus, "NEED_REVIEW") {
			hasNeedReview = true
		}
		if strings.EqualFold(it.RiskLevel, "HIGH") {
			hasHigh = true
		}
		if strings.EqualFold(it.PositionStatus, "HOLDING_LOSS") || strings.EqualFold(it.RiskLevel, "MEDIUM") {
			hasLossOrMed = true
		}
	}
	switch {
	case pre == "BLOCKED" || hasNeedReview || hasHigh || conc == "HIGH":
		out.Status = AttentionReview
	case pre == "WARNING" || hasLossOrMed || conc == "ELEVATED":
		out.Status = AttentionWatch
	default:
		out.Status = AttentionHold
	}
	return out
}

func aggregate(statuses ...string) string {
	overall := AttentionHold
	for _, s := range statuses {
		switch s {
		case AttentionReview:
			return AttentionReview
		case AttentionWatch:
			overall = AttentionWatch
		}
	}
	return overall
}

func buildExplanation(sum *PortfolioDecisionSummary) string {
	parts := []string{}
	switch sum.OverallAttention {
	case AttentionReview:
		parts = append(parts, "组合需复盘")
	case AttentionWatch:
		parts = append(parts, "组合有关注信号")
	default:
		parts = append(parts, "组合暂无强制关注事项")
	}
	if sum.PortfolioHealth.Status != AttentionHold && len(sum.PortfolioHealth.Notes) > 0 {
		parts = append(parts, strings.Join(sum.PortfolioHealth.Notes, "；"))
	}
	if sum.RiskAttention.Status != AttentionHold {
		if sum.RiskAttention.PretradeLevel != "" {
			parts = append(parts, fmt.Sprintf("PreTrade=%s", sum.RiskAttention.PretradeLevel))
		}
		if n := len(sum.RiskAttention.IntelligenceItems); n > 0 {
			parts = append(parts, fmt.Sprintf("%d 只持仓风险关注", n))
		}
	}
	if sum.CapitalEfficiencyAttention.LowEfficiencyCount > 0 {
		parts = append(parts, fmt.Sprintf("%d 只低资金效率持仓", sum.CapitalEfficiencyAttention.LowEfficiencyCount))
	}
	if sum.OpportunityAttention.Status != AttentionHold && sum.OpportunityAttention.CandidateCode != "" {
		parts = append(parts, fmt.Sprintf("新机会 %s 与现仓存在质量分差", sum.OpportunityAttention.CandidateCode))
	}
	parts = append(parts, "系统仅提示关注，不自动买卖或调仓。")
	return strings.Join(parts, "。")
}

func clamp100(v float64) float64 {
	return math.Max(0, math.Min(100, v))
}
