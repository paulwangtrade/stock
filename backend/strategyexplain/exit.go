package strategyexplain

import (
	"fmt"
	"strings"
)

// Aligns with papertrading Exit Evaluation reason codes (no import — avoid trading coupling).
const (
	exitCodeTimeReview = "TIME_REVIEW"
	exitCodeLossReview = "LOSS_REVIEW"
	exitCodePlanReview = "PLAN_REVIEW"
	exitStateReviewReq = "REVIEW_REQUIRED"
)

func renderExit(overlay *ExitOverlay, include bool) *ExitReason {
	if !include {
		return nil
	}
	if overlay == nil {
		return &ExitReason{
			Mode:            ExitModeUnavailable,
			Narrative:       "未提供持仓复评上下文，无法生成卖出复评解释。",
			ComparedToEntry: "相对买入命题：退出复评数据不可用。",
		}
	}
	codes := normalizeCodes(overlay.ExitReasonCodes)
	state := strings.TrimSpace(overlay.ExitState)
	hasReview := false
	for _, c := range codes {
		if c == exitCodeTimeReview || c == exitCodeLossReview || c == exitCodePlanReview {
			hasReview = true
			break
		}
	}
	if state == exitStateReviewReq {
		hasReview = true
	}

	out := &ExitReason{
		ExitState:   state,
		ReasonCodes: codes,
	}
	if !hasReview && len(codes) == 0 && state != exitStateReviewReq && state != "WATCH" {
		out.Mode = ExitModeNone
		out.Narrative = buildExitNoneNarrative(overlay)
		out.ComparedToEntry = "相对买入命题：暂无卖出复评信号。"
		return out
	}
	if !hasReview && state == "WATCH" {
		out.Mode = ExitModeNone
		out.Narrative = buildExitWatchNarrative(overlay)
		out.ComparedToEntry = "相对买入命题：进入关注区间，建议复核假设（非卖出指令）。"
		return out
	}

	out.Mode = ExitModeReview
	out.Narrative = buildExitReviewNarrative(overlay, codes, state)
	out.ComparedToEntry = buildComparedToEntry(codes)
	return out
}

func normalizeCodes(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func buildExitNoneNarrative(o *ExitOverlay) string {
	days := o.HoldingDays
	if o.UnrealizedReturn == nil {
		if days > 0 {
			return fmt.Sprintf("持有%d天，现价或收益未知，暂无复评信号。", days)
		}
		return "暂无卖出复评信号。"
	}
	ret := fmt.Sprintf("%+.1f%%", *o.UnrealizedReturn*100)
	if days > 0 {
		return fmt.Sprintf("持有%d天，当前收益%s，暂无复评信号。", days, ret)
	}
	return fmt.Sprintf("当前收益%s，暂无复评信号。", ret)
}

func buildExitWatchNarrative(o *ExitOverlay) string {
	days := o.HoldingDays
	retTxt := "收益未知"
	if o.UnrealizedReturn != nil {
		retTxt = fmt.Sprintf("当前收益%+.1f%%", *o.UnrealizedReturn*100)
	}
	if days > 0 {
		return fmt.Sprintf("持有%d天，%s，浮亏或状态进入关注区间，建议复核持仓假设。", days, retTxt)
	}
	return retTxt + "，进入关注区间，建议复核持仓假设。"
}

func buildExitReviewNarrative(o *ExitOverlay, codes []string, state string) string {
	has := map[string]bool{}
	for _, c := range codes {
		has[c] = true
	}
	retTxt := "收益未知"
	if o.UnrealizedReturn != nil {
		retTxt = fmt.Sprintf("当前收益%+.1f%%", *o.UnrealizedReturn*100)
	}
	days := o.HoldingDays
	switch {
	case has[exitCodePlanReview]:
		return "关联计划生命周期异常，需要重新评估持仓逻辑。"
	case has[exitCodeTimeReview]:
		return "超过策略最大观察周期，需要重新评估。"
	case has[exitCodeLossReview] && state == exitStateReviewReq:
		if days > 0 {
			return fmt.Sprintf("持有%d天，%s，浮亏触及复评阈值，需要重新评估。", days, retTxt)
		}
		return retTxt + "，浮亏触及复评阈值，需要重新评估。"
	case has[exitCodeLossReview]:
		if days > 0 {
			return fmt.Sprintf("持有%d天，%s，浮亏进入关注区间，建议复核持仓假设。", days, retTxt)
		}
		return retTxt + "，浮亏进入关注区间，建议复核持仓假设。"
	default:
		if days > 0 {
			return fmt.Sprintf("持有%d天，%s，需要重新评估持仓逻辑。", days, retTxt)
		}
		return "需要重新评估持仓逻辑。"
	}
}

func buildComparedToEntry(codes []string) string {
	parts := make([]string, 0, 3)
	for _, c := range codes {
		switch c {
		case exitCodeTimeReview:
			parts = append(parts, "持有超期")
		case exitCodeLossReview:
			parts = append(parts, "浮亏复评")
		case exitCodePlanReview:
			parts = append(parts, "计划异常")
		}
	}
	if len(parts) == 0 {
		return "相对买入命题：出现复评观察原因（非卖出指令）。"
	}
	return "相对买入命题：" + strings.Join(parts, " / ") + "（非卖出指令）。"
}
