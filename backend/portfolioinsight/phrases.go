package portfolioinsight

import (
	"fmt"
	"sort"
	"strings"

	"go-stock/backend/holdingdecision/rules"
)

var reasonPlainZH = map[string]string{
	rules.ReasonPnLTakeProfit:    "浮盈已达到系统观察的止盈关注线，标记为降低集中度观察",
	rules.ReasonPnLLargeProtect:  "浮盈较大，系统提示关注利润保护（观察）",
	rules.ReasonPnLMaxLoss:       "浮亏超过观察用最大容忍线",
	rules.ReasonPnLRiskWorse:     "收益风险状态相对恶化，进入减持观察",
	rules.ReasonTenureStale:      "持有较久且表现平淡，进入长期无变化观察",
	rules.ReasonTenureLongReview: "持有时间较长，进入定期复核观察",
	rules.ReasonTrendBreak:       "已提供的趋势事实显示结构破坏（观察）",
	rules.ReasonRiskNameOverCap:  "该票权重大于组合单票上限观察值",
	rules.ReasonRiskSectorHot:    "所属行业组合权重超过行业上限观察值",
	rules.ReasonRiskGrossHot:     "组合毛敞口余量不足，高权重票进入修剪观察",
	rules.ReasonPortfolioTighten: "风险收紧后的约束下，该票权重触发观察",
	rules.ReasonDataMissing:      "部分持仓事实缺失，仅作数据缺口说明",
	rules.ReasonDefaultHold:      "未触发减持类规则，维持持有观察",
}

var riskLevelLabelZH = map[string]string{
	RiskLow:         "较低",
	RiskModerate:    "中等",
	RiskElevated:    "偏高",
	RiskHigh:        "高",
	RiskUnavailable: "暂不可评",
}

func plainReason(code string) string {
	code = strings.TrimSpace(code)
	if v, ok := reasonPlainZH[code]; ok {
		return v
	}
	if code == "" {
		return "上游给出了减持/退出观察，但未附带稳定原因码"
	}
	return fmt.Sprintf("观察原因码 %s（详见规则命中）", code)
}

func reduceHeadline(action string) string {
	switch strings.ToUpper(strings.TrimSpace(action)) {
	case rules.ActionExit:
		return "观察：标记为退出关注（不是卖出指令）"
	case rules.ActionReduce:
		return "观察：建议关注减持（非交易指令）"
	default:
		return "观察：持仓动作说明"
	}
}

func sortedUnique(ss []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Forbidden user-facing phrases (must never appear in Insight copy).
var forbiddenPhrases = []string{
	"立即卖出", "强制平仓", "一键跟单", "保证收益", "请立即卖出", "强制清仓",
}
