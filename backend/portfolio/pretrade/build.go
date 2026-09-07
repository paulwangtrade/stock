package pretrade

import (
	"fmt"
	"strings"
	"time"
)

const (
	elevatedWeightThreshold = 0.10
	highWeightThreshold     = 0.15

	intentGapSkip    = "gap_skip"
	intentSizeSkip   = "size_skip"
	intentManualSkip = "manual_skip"
	itemSkipped      = "skipped"
)

// Build assembles PreTradeRiskResult (pure, no I/O).
func Build(in Inputs) *PreTradeRiskResult {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	out := &PreTradeRiskResult{
		PlanID:          in.PlanID,
		TradeDate:       strings.TrimSpace(in.TradeDate),
		AsOf:            asOf,
		PositionActions: []PositionAction{},
		RiskSnapshot:    in.RiskSnapshot,
		DataSourceNote:  dataSourceNote,
		Disclaimer:      disclaimer,
	}

	equity := in.Account.Equity
	if equity <= 0 {
		equity = in.Account.Cash
	}

	posByCode := map[string]PositionInput{}
	for _, p := range in.Positions {
		code := normalizeCode(p.StockCode)
		if code != "" {
			posByCode[code] = p
		}
	}

	var required float64
	var addCount, newCount int
	var conflictCodes []string
	var maxWeight float64
	actions := make([]PositionAction, 0)

	for _, line := range in.Lines {
		if !isExecutableBuy(line) {
			continue
		}
		req := itemRequiredCash(line)
		if req <= 0 {
			continue
		}
		required += req

		code := normalizeCode(line.StockCode)
		weight := 0.0
		if equity > 0 {
			weight = req / equity
		}
		if weight > maxWeight {
			maxWeight = weight
		}

		action := PositionAction{
			StockCode:     line.StockCode,
			StockName:     line.StockName,
			PlannedAmount: req,
			PlannedVolume: line.TargetVolume,
		}
		if pos, ok := posByCode[code]; ok && pos.TotalVolume > 0 {
			action.Action = ActionAdd
			action.ExistingVolume = pos.TotalVolume
			action.StockName = firstNonEmpty(line.StockName, pos.StockName)
			action.Reason = "已有持仓，计划买入视为加仓"
			addCount++
			conflictCodes = append(conflictCodes, line.StockCode)
		} else {
			action.Action = ActionNew
			action.Reason = "当前无持仓，计划买入视为新开"
			newCount++
		}
		actions = append(actions, action)
	}

	// HOLD: existing positions not touched by plan (observation for Allocation Assistant).
	planned := map[string]bool{}
	for _, a := range actions {
		planned[normalizeCode(a.StockCode)] = true
	}
	for _, p := range in.Positions {
		code := normalizeCode(p.StockCode)
		if code == "" || p.TotalVolume <= 0 || planned[code] {
			continue
		}
		actions = append(actions, PositionAction{
			StockCode:      p.StockCode,
			StockName:      p.StockName,
			Action:         ActionHold,
			ExistingVolume: p.TotalVolume,
			Reason:         "计划未覆盖该持仓，建议继续持有观察",
		})
	}
	out.PositionActions = actions

	// Cash
	cash := CashCheck{
		AvailableCash: in.Account.Cash,
		RequiredCash:  required,
		CashEnough:    in.Account.Cash >= required || required <= 0,
	}
	if !cash.CashEnough {
		cash.Status = StatusBlocked
		cash.Reason = fmt.Sprintf("需要 %.0f，可用 %.0f", required, in.Account.Cash)
	} else {
		cash.Status = StatusPass
		cash.Reason = "可用现金足以覆盖本计划预计占用"
	}
	out.CashCheck = cash

	// Existing position aggregate
	ep := ExistingPositionCheck{
		AddCount:      addCount,
		NewCount:      newCount,
		ConflictCodes: conflictCodes,
	}
	if addCount > 0 {
		ep.Status = StatusWarning
		ep.Reason = fmt.Sprintf("%d 只计划标的已有持仓（ADD）", addCount)
	} else {
		ep.Status = StatusPass
		ep.Reason = "计划标的均为新开（NEW）或无可执行买入行"
	}
	out.ExistingPositionCheck = ep

	// Concentration
	concLevel := ConcentrationNormal
	switch {
	case maxWeight >= highWeightThreshold:
		concLevel = ConcentrationHigh
	case maxWeight >= elevatedWeightThreshold:
		concLevel = ConcentrationElevated
	}
	cc := ConcentrationCheck{
		Level:        concLevel,
		MaxWeightPct: maxWeight * 100,
		IndustryNote: "行业集中度暂不可用（无行业主数据聚合）",
	}
	if concLevel == ConcentrationHigh {
		cc.Status = StatusWarning
		cc.Reason = "执行后单票权重偏高"
	} else if concLevel == ConcentrationElevated {
		cc.Status = StatusWarning
		cc.Reason = "执行后单票权重偏高（关注）"
	} else {
		cc.Status = StatusPass
		cc.Reason = "单票集中度正常"
	}
	out.ConcentrationCheck = cc

	out.Level, out.RequiresConfirm, out.Summary = aggregate(out)
	// F.1: allowed is informational mirror of "not BLOCKED"; never enforced by Gateway.
	out.Allowed = out.Level != LevelBlocked
	return out
}

func aggregate(r *PreTradeRiskResult) (level string, confirm bool, summary string) {
	if r.CashCheck.Status == StatusBlocked {
		return LevelBlocked, false, "可用现金不足以覆盖本计划预计占用"
	}
	warn := r.ExistingPositionCheck.Status == StatusWarning ||
		r.ConcentrationCheck.Status == StatusWarning
	if warn {
		parts := []string{}
		if r.ExistingPositionCheck.AddCount > 0 {
			parts = append(parts, r.ExistingPositionCheck.Reason)
		}
		if r.ConcentrationCheck.Status == StatusWarning {
			parts = append(parts, r.ConcentrationCheck.Reason)
		}
		summary = strings.Join(parts, "；")
		if summary == "" {
			summary = "执行前存在需确认事项"
		}
		return LevelWarning, true, summary
	}
	return LevelPass, false, "执行前风险检查通过（只读观察）"
}

func isExecutableBuy(it PlanLine) bool {
	side := strings.ToLower(strings.TrimSpace(it.Side))
	if side != "" && side != "buy" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(it.Status), itemSkipped) {
		return false
	}
	intent := strings.ToLower(strings.TrimSpace(it.IntentStatus))
	switch intent {
	case intentGapSkip, intentSizeSkip, intentManualSkip:
		return false
	}
	return true
}

func itemRequiredCash(it PlanLine) float64 {
	if it.TargetVolume > 0 && it.LimitPrice > 0 {
		return float64(it.TargetVolume) * it.LimitPrice
	}
	if it.TargetAmount > 0 {
		return it.TargetAmount
	}
	if it.TargetVolume > 0 {
		px := it.LimitPrice
		if px <= 0 {
			px = it.OpenRefPrice
		}
		if px <= 0 {
			px = it.RefPrice
		}
		if px > 0 {
			return float64(it.TargetVolume) * px
		}
	}
	return 0
}

func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
