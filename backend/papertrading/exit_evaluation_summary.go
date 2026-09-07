// Exit Review Summary — read-only explanation text (Phase10-D.2.7).
// Explains why re-check may be needed; never instructs sell / close / stop-loss execution.

package papertrading

import (
	"fmt"
	"strings"
)

// BuildExitReviewSummary builds a human-readable observation summary (Chinese).
func BuildExitReviewSummary(holdingDays int, unrealizedReturn *float64, state string, reasonCodes []string) string {
	has := map[string]bool{}
	for _, c := range reasonCodes {
		has[strings.TrimSpace(c)] = true
	}
	retTxt := "收益未知"
	if unrealizedReturn != nil {
		retTxt = fmt.Sprintf("当前收益%+.1f%%", *unrealizedReturn*100)
	}

	switch {
	case has[ExitReasonPlanReview]:
		return "关联计划生命周期异常，需要重新评估持仓逻辑"
	case has[ExitReasonTimeReview]:
		return "超过策略最大观察周期，需要重新评估"
	case has[ExitReasonLossReview] && state == ExitEvalStateReviewRequired:
		return fmt.Sprintf("持有%d天，%s，浮亏触及复评阈值，需要重新评估", holdingDays, retTxt)
	case has[ExitReasonLossReview]:
		return fmt.Sprintf("持有%d天，%s，浮亏进入关注区间，建议复核持仓假设", holdingDays, retTxt)
	default:
		if unrealizedReturn == nil {
			return fmt.Sprintf("持有%d天，现价缺失，暂无收益复评", holdingDays)
		}
		return fmt.Sprintf("持有%d天，%s，暂无复评信号", holdingDays, retTxt)
	}
}
