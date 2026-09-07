package readiness

import (
	"strings"
)

const (
	elevatedWeightThreshold = 0.10
	highWeightThreshold     = 0.15

	severityWarn   = "WARN"
	severityBlock  = "BLOCK"

	intentGapSkip    = "gap_skip"
	intentSizeSkip   = "size_skip"
	intentManualSkip = "manual_skip"
	itemSkipped      = "skipped"
)

// Evaluate computes execution readiness from plan items, account, and positions.
// Pure function — no DB or side effects.
func Evaluate(planID int, items []PlanItem, account AccountSnapshot, positions []PositionSnapshot) ExecutionReadiness {
	out := ExecutionReadiness{
		PlanID:        planID,
		AvailableCash: account.Cash,
		TotalEquity:   account.Equity,
		Concentration: ConcentrationNormal,
		Status:        StatusReady,
	}
	if out.TotalEquity <= 0 {
		out.TotalEquity = account.Cash
	}

	posByCode := map[string]PositionSnapshot{}
	for _, p := range positions {
		code := normalizeCode(p.StockCode)
		if code != "" {
			posByCode[code] = p
		}
	}

	var requiredTotal float64
	for _, it := range items {
		if !isExecutableBuy(it) {
			continue
		}
		req := itemRequiredCash(it)
		if req <= 0 {
			continue
		}
		requiredTotal += req

		code := normalizeCode(it.StockCode)
		if pos, ok := posByCode[code]; ok && pos.TotalVolume > 0 {
			conflict := PositionConflict{
				StockCode: it.StockCode,
				StockName: firstNonEmpty(it.StockName, pos.StockName),
				Side:      "buy",
				Quantity:  pos.TotalVolume,
				Message:   "当前已有持仓，是否继续加仓需策略确认",
			}
			out.ExistingPositions = append(out.ExistingPositions, conflict)
			out.Issues = append(out.Issues, ReadinessIssue{
				Code:     IssueAlreadyHolding,
				Severity: severityWarn,
				Message:  conflict.Message,
			})
		}

		weight := 0.0
		if out.TotalEquity > 0 {
			weight = req / out.TotalEquity
		}
		out.AfterExecution = append(out.AfterExecution, PositionWeightProjection{
			StockCode:       it.StockCode,
			StockName:       it.StockName,
			RequiredCash:    req,
			ProjectedWeight: weight,
			WeightPct:       weight * 100,
		})
	}

	out.RequiredCash = requiredTotal
	out.CashEnough = out.AvailableCash >= out.RequiredCash
	if !out.CashEnough && out.RequiredCash > 0 {
		out.Issues = append([]ReadinessIssue{{
			Code:     IssueCashInsufficient,
			Severity: severityBlock,
			Message:  "可用现金不足以覆盖本计划预计占用",
		}}, out.Issues...)
	}

	out.Concentration = classifyConcentration(out.AfterExecution)
	if out.Concentration == ConcentrationHigh {
		out.Issues = append(out.Issues, ReadinessIssue{
			Code:     IssueHighConcentration,
			Severity: severityWarn,
			Message:  "执行后单项集中度偏高",
		})
	}

	out.Status = aggregateStatus(out)
	return out
}

func aggregateStatus(r ExecutionReadiness) string {
	if !r.CashEnough && r.RequiredCash > 0 {
		return StatusBlocked
	}
	if len(r.ExistingPositions) > 0 || r.Concentration == ConcentrationElevated || r.Concentration == ConcentrationHigh {
		return StatusWarning
	}
	return StatusReady
}

func classifyConcentration(projections []PositionWeightProjection) string {
	maxW := 0.0
	for _, p := range projections {
		if p.ProjectedWeight > maxW {
			maxW = p.ProjectedWeight
		}
	}
	switch {
	case maxW >= highWeightThreshold:
		return ConcentrationHigh
	case maxW >= elevatedWeightThreshold:
		return ConcentrationElevated
	default:
		return ConcentrationNormal
	}
}

func isExecutableBuy(it PlanItem) bool {
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

func itemRequiredCash(it PlanItem) float64 {
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
