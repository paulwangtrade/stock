package exitwatch

import (
	"math"
	"strings"
	"time"
)

// Project maps one holding's facts onto an observation alert.
// Missing price, cost, identity, or source becomes DATA_INSUFFICIENT
// and does not keep TIME/LOSS/SIGNAL/PLAN.
func Project(f Facts) Item {
	f = normalizeFacts(f)
	pol := f.Policy.normalized()
	item := Item{
		Source:                f.Source,
		PositionID:            f.PositionID,
		StockCode:             f.StockCode,
		StockName:             strings.TrimSpace(f.StockName),
		ReasonCodes:           []string{},
		Bar:                   f.Bar,
		PolicyID:              pol.ID,
		PolicyVersion:         pol.Version,
		PolicyRef:             pol.Ref(),
		AsOf:                  f.AsOf,
		RequiresManualConfirm: true,
		NotAnOrder:            true,
		Disclaimer:            Disclaimer,
	}

	if !sourceOK(f.Source) {
		return finishInsufficient(item, f, []string{ReasonNoSource}, ReasonNoSource, "来源缺失，退出观察失败关闭为数据不足")
	}
	if !identityOK(f) {
		return finishInsufficient(item, f, nil, RuleIncomplete, "持仓身份不完整，退出观察失败关闭为数据不足")
	}
	if f.PriceStale {
		return finishInsufficient(item, f, []string{ReasonStale}, ReasonStale, "行情过期，退出观察失败关闭为数据不足")
	}
	if !f.PriceKnown {
		return finishInsufficient(item, f, nil, RuleIncomplete, "现价缺失，退出观察失败关闭为数据不足")
	}
	if !f.CostKnown {
		return finishInsufficient(item, f, nil, RuleIncomplete, "成本缺失，退出观察失败关闭为数据不足")
	}
	if f.RequireHoldingDays && !f.HoldingDaysKnown {
		return finishInsufficient(item, f, nil, RuleIncomplete, "持有天数缺失，退出观察失败关闭为数据不足")
	}
	if !f.UseExitEvaluation && f.UnrealizedReturn != nil && finiteReturn(f.UnrealizedReturn) == nil {
		return finishInsufficient(item, f, nil, RuleIncomplete, "收益率无效，退出观察失败关闭为数据不足")
	}
	if f.UseExitEvaluation && !knownExitState(f.ExitState) {
		return finishInsufficient(item, f, nil, RuleIncomplete, "退出评估状态未知，失败关闭为数据不足")
	}

	class, reasons := classify(f, pol)
	if f.T1Locked {
		reasons = appendReason(reasons, ReasonT1Locked)
	}
	item.Class = class
	item.Label = classLabel(class)
	item.ReasonCodes = sortReasons(reasons)
	item.Rule = primaryRule(class, item.ReasonCodes)
	item.DedupKey = dedupKey(item.Source, item.PositionID, item.Rule, item.Bar)
	item.Summary = sufficientSummary(item.Label, item.ReasonCodes)
	if f.Source == SourcePaperSim && (class == ClassReduce || class == ClassFlatten) {
		item.SellIntentAllowed = true
		item.ManualSellDraftRef = strings.TrimSpace(f.ManualDraftRef)
	}
	return item
}

func normalizeFacts(f Facts) Facts {
	f.Source = strings.TrimSpace(strings.ToLower(f.Source))
	f.PositionID = strings.TrimSpace(f.PositionID)
	f.StockCode = strings.TrimSpace(f.StockCode)
	f.Bar = strings.TrimSpace(f.Bar)
	if f.AsOf.IsZero() {
		f.AsOf = time.Now()
	}
	if f.Bar == "" {
		f.Bar = f.AsOf.In(time.Local).Format("2006-01-02")
	}
	if f.Policy.ID == "" && f.Policy.Version == 0 {
		f.Policy = DefaultPolicy
	}
	return f
}

func sourceOK(src string) bool {
	return src == SourcePaperSim || src == SourceExternalMirror
}

func identityOK(f Facts) bool {
	if f.StockCode == "" || f.PositionID == "" {
		return false
	}
	if f.RequireQuantity && (!f.QuantityKnown || f.Quantity <= 0) {
		return false
	}
	return true
}

func knownExitState(state string) bool {
	switch strings.TrimSpace(state) {
	case "NORMAL", "WATCH", "REVIEW_REQUIRED":
		return true
	default:
		return false
	}
}

func finishInsufficient(item Item, f Facts, reasons []string, rule, summary string) Item {
	if f.T1Locked && rule != ReasonNoSource {
		reasons = appendReason(reasons, ReasonT1Locked)
	}
	item.Class = ClassInsufficient
	item.Label = classLabel(ClassInsufficient)
	item.ReasonCodes = sortReasons(reasons)
	item.Rule = rule
	item.DedupKey = dedupKey(item.Source, item.PositionID, item.Rule, item.Bar)
	item.Summary = summary
	item.SellIntentAllowed = false
	item.ManualSellDraftRef = ""
	return item
}

func classify(f Facts, pol Policy) (string, []string) {
	class := ClassHold
	var reasons []string
	if f.UseExitEvaluation {
		switch strings.TrimSpace(f.ExitState) {
		case "REVIEW_REQUIRED":
			class = ClassFlatten
		case "WATCH":
			class = ClassReduce
		default:
			class = ClassHold
		}
		for _, code := range f.ExitReasonCodes {
			switch strings.TrimSpace(code) {
			case "TIME_REVIEW":
				reasons = appendReason(reasons, ReasonTime)
			case "LOSS_REVIEW":
				reasons = appendReason(reasons, ReasonLoss)
			case "PLAN_REVIEW":
				reasons = appendReason(reasons, ReasonPlan)
			}
		}
	} else {
		if f.HoldingDaysKnown && f.HoldingDays > pol.MaxHoldingDays {
			reasons = appendReason(reasons, ReasonTime)
			class = ClassReduce
		}
		if ret := finiteReturn(f.UnrealizedReturn); ret != nil {
			r := *ret
			if r <= pol.LossReviewThreshold {
				reasons = appendReason(reasons, ReasonLoss)
				class = ClassFlatten
			} else if r <= pol.LossWatchThreshold {
				reasons = appendReason(reasons, ReasonLoss)
				if class == ClassHold {
					class = ClassReduce
				}
			}
		}
	}
	if f.SignalReview {
		reasons = appendReason(reasons, ReasonSignal)
		if class == ClassHold {
			class = ClassReduce
		}
	}
	return class, reasons
}

func finiteReturn(r *float64) *float64 {
	if r == nil || math.IsNaN(*r) || math.IsInf(*r, 0) {
		return nil
	}
	return r
}

func appendReason(reasons []string, code string) []string {
	for _, c := range reasons {
		if c == code {
			return reasons
		}
	}
	return append(reasons, code)
}

func sortReasons(reasons []string) []string {
	if len(reasons) == 0 {
		return []string{}
	}
	order := []string{ReasonTime, ReasonLoss, ReasonSignal, ReasonPlan, ReasonStale, ReasonNoSource, ReasonT1Locked}
	out := make([]string, 0, len(reasons))
	seen := map[string]bool{}
	for _, c := range reasons {
		seen[c] = true
	}
	for _, c := range order {
		if seen[c] {
			out = append(out, c)
		}
	}
	return out
}

func primaryRule(class string, reasons []string) string {
	if class == ClassInsufficient {
		return RuleIncomplete
	}
	rank := []string{ReasonLoss, ReasonPlan, ReasonSignal, ReasonTime, ReasonT1Locked}
	has := map[string]bool{}
	for _, c := range reasons {
		has[c] = true
	}
	for _, c := range rank {
		if has[c] {
			return c
		}
	}
	return RuleHold
}

func dedupKey(source, positionID, rule, bar string) string {
	return source + "|" + positionID + "|" + rule + "|" + bar
}

func classLabel(class string) string {
	switch class {
	case ClassHold:
		return "持有观察"
	case ClassReduce:
		return "减仓观察"
	case ClassFlatten:
		return "清仓观察"
	default:
		return "数据不足"
	}
}

func reasonLabel(code string) string {
	switch code {
	case ReasonTime:
		return "持有时间"
	case ReasonLoss:
		return "浮亏"
	case ReasonSignal:
		return "信号变化"
	case ReasonPlan:
		return "计划异常"
	case ReasonStale:
		return "行情过期"
	case ReasonNoSource:
		return "来源缺失"
	case ReasonT1Locked:
		return "当日买入锁定"
	default:
		return ""
	}
}

func sufficientSummary(label string, reasons []string) string {
	parts := make([]string, 0, len(reasons))
	for _, c := range reasons {
		if s := reasonLabel(c); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return label + "。暂无退出观察信号。" + Disclaimer
	}
	return label + "：" + strings.Join(parts, "、") + "。" + Disclaimer
}
