package strategyexplain

import (
	"fmt"
	"strings"

	"go-stock/backend/strategysnapshot"
)

func renderSignal(sr strategysnapshot.SignalResult) SignalExplanation {
	if sr.Unavailable || (sr.SignalSnapshotID == 0 && strings.TrimSpace(sr.Tag) == "" && sr.Score == 0) {
		reason := sr.MissingReason
		if reason == "" {
			reason = "signal fields empty"
		}
		return SignalExplanation{Available: false, MissingReason: reason, Narrative: "信号结果不可用，无法解释扫描依据。"}
	}
	var b strings.Builder
	b.WriteString("信号")
	if strings.TrimSpace(sr.Tag) != "" {
		b.WriteString("标签「")
		b.WriteString(strings.TrimSpace(sr.Tag))
		b.WriteString("」")
	}
	if sr.Score != 0 {
		b.WriteString(fmt.Sprintf("，信号分 %.2f", sr.Score))
	}
	if sr.SignalSnapshotID > 0 {
		b.WriteString(fmt.Sprintf("，引用扫描 #%d", sr.SignalSnapshotID))
	}
	if strings.TrimSpace(sr.Session) != "" {
		b.WriteString("（时段 ")
		b.WriteString(sr.Session)
		b.WriteString("）")
	}
	b.WriteString("。未重新计算指标。")
	return SignalExplanation{
		Available:   true,
		Tag:         sr.Tag,
		Score:       sr.Score,
		SnapshotRef: sr.SignalSnapshotID,
		Narrative:   b.String(),
	}
}

func renderRisk(rd strategysnapshot.RiskDecision) RiskExplanation {
	if rd.Unavailable && rd.RiskCode == "" && rd.RiskMessage == "" && rd.RiskStatus == "" {
		reason := rd.MissingReason
		if reason == "" {
			reason = "risk fields empty"
		}
		return RiskExplanation{Available: false, MissingReason: reason, Narrative: "风控快照不可用。"}
	}
	out := RiskExplanation{
		Available:   true,
		Accepted:    rd.ItemAccepted,
		RiskCode:    rd.RiskCode,
		RiskMessage: rd.RiskMessage,
		PlanStatus:  rd.RiskStatus,
	}
	switch {
	case rd.ItemAccepted != nil && !*rd.ItemAccepted:
		msg := "条目未通过风控过滤"
		if rd.RiskCode != "" {
			msg += "（" + rd.RiskCode
			if rd.RiskMessage != "" {
				msg += "：" + rd.RiskMessage
			}
			msg += "）"
		} else if rd.RiskMessage != "" {
			msg += "：" + rd.RiskMessage
		}
		msg += "，因此未纳入可执行意图。"
		out.Narrative = msg
	case rd.ItemAccepted != nil && *rd.ItemAccepted:
		out.Narrative = "条目通过计划风控过滤"
		if rd.RiskStatus != "" {
			out.Narrative += "（计划状态 " + rd.RiskStatus + "）"
		}
		out.Narrative += "。"
	default:
		if rd.RiskSummary != "" {
			out.Narrative = "计划风控摘要：" + rd.RiskSummary
		} else if rd.RiskStatus != "" {
			out.Narrative = "计划风控状态：" + rd.RiskStatus + "。"
		} else {
			out.Narrative = "风控信息已记录，条目接受状态未知。"
		}
	}
	return out
}

func renderEntry(snap *strategysnapshot.StrategySnapshot, reasonHint, ruleHint string) EntryReason {
	sv := snap.StrategyVersion
	md := snap.MarketDataRef
	rule := strings.TrimSpace(ruleHint)
	if rule == "" && snap.Parameters.Values != nil {
		if v, ok := snap.Parameters.Values["default_entry_rule"].(string); ok {
			rule = v
		}
	}
	reason := strings.TrimSpace(reasonHint)
	er := EntryReason{
		StrategyName:     sv.StrategyName,
		StrategyVersion:  sv.Version,
		EntryReasonRaw:   reason,
		EntryRule:        rule,
		RefPrice:         md.RefPrice,
		RefSource:        md.RefSource,
		ThesisIntactNote: "命题以生成时刻 StrategySnapshot 为准，持有期不覆盖、不重算策略。",
	}
	var parts []string
	name := strings.TrimSpace(sv.StrategyName)
	if name == "" {
		name = "未命名策略"
	}
	ver := strings.TrimSpace(sv.Version)
	if ver != "" {
		parts = append(parts, fmt.Sprintf("策略 %s（%s）", name, ver))
	} else {
		parts = append(parts, "策略 "+name)
	}
	if reason != "" {
		parts = append(parts, "入选原因："+reason)
	}
	if rule != "" {
		parts = append(parts, "入场规则 "+rule)
	}
	if !md.Unavailable && md.RefPrice > 0 {
		src := md.RefSource
		if src == "" {
			src = "ref"
		}
		asof := md.RefAsOf
		if asof != "" {
			parts = append(parts, fmt.Sprintf("参考价 %.4f（%s @ %s）", md.RefPrice, src, asof))
		} else {
			parts = append(parts, fmt.Sprintf("参考价 %.4f（%s）", md.RefPrice, src))
		}
	} else if md.Unavailable {
		parts = append(parts, "价锚信息缺失")
	}
	er.Narrative = strings.Join(parts, "；") + "。"
	return er
}

func buildHeadline(signal SignalExplanation, risk RiskExplanation, entry EntryReason) string {
	accepted := risk.Accepted != nil && *risk.Accepted
	rejected := risk.Accepted != nil && !*risk.Accepted
	switch {
	case rejected:
		code := risk.RiskCode
		if code == "" {
			code = "风控"
		}
		return fmt.Sprintf("因%s未纳入可执行计划。", code)
	case signal.Available && accepted:
		tag := signal.Tag
		if tag == "" {
			tag = "信号"
		}
		return fmt.Sprintf("因信号「%s」且风控通过而纳入计划。", tag)
	case accepted:
		if entry.StrategyName != "" {
			return fmt.Sprintf("策略 %s 将标的纳入计划（风控通过）。", entry.StrategyName)
		}
		return "风控通过，标的纳入计划。"
	case signal.Available:
		return "存在信号依据；风控接受状态不完整。"
	default:
		return "基于 Snapshot 的策略解释（部分事实缺失）。"
	}
}

func defaultDisclaimers() []string {
	return []string{
		"本说明为只读策略解释，不构成投资建议或买卖指令。",
		"不修改 TradePlan / Strategy / Execution，不触发 Broker / Gateway / Fill。",
	}
}
