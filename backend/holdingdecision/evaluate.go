package holdingdecision

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/papertrading"
)

const actionNone = "none"

func (engine) Evaluate(eval any, policy Policy) *View {
	v, _ := eval.(*papertrading.HoldingEvalObservationView)
	return EvaluateObservation(v, policy)
}

// EvaluateObservation projects decisions from an evaluation view. Does not mutate eval.
func EvaluateObservation(eval *papertrading.HoldingEvalObservationView, policy Policy) *View {
	out := emptyView(policy)
	if eval == nil {
		return out
	}
	if !eval.AsOf.IsZero() {
		out.AsOf = eval.AsOf.Format(time.RFC3339)
	}
	for _, row := range eval.Holdings {
		out.Holdings = append(out.Holdings, decideStock(row, policy))
	}
	sort.Slice(out.Holdings, func(i, j int) bool {
		return out.Holdings[i].Symbol < out.Holdings[j].Symbol
	})
	for _, h := range out.Holdings {
		out.ByState[h.State]++
	}
	return out
}

func emptyView(policy Policy) *View {
	return &View{
		ExitCandidateEnabled: policy.ExitCandidateEnabled,
		ByState: map[string]int{
			StateHoldNormal:    0,
			StateHoldWatch:     0,
			StateHoldReview:    0,
			StateExitCandidate: 0,
		},
		Holdings:       []StockDecision{},
		DataSourceNote: dataSourceNote,
	}
}

func decideStock(row papertrading.HoldingEvalStockRow, policy Policy) StockDecision {
	symbol := strings.TrimSpace(row.StockCode)
	lots := make([]HoldingDecision, 0, len(row.Lots))
	for _, lot := range row.Lots {
		lots = append(lots, decideLot(symbol, lot, row, policy))
	}
	var top HoldingDecision
	if len(lots) == 0 {
		top = decideFacts(symbol, 0, 0, factsFromStock(row), policy)
	} else {
		top = lots[0]
		for i := 1; i < len(lots); i++ {
			if rankState(lots[i].State) > rankState(top.State) {
				top = lots[i]
			}
		}
	}
	return StockDecision{
		Symbol:      symbol,
		State:       top.State,
		Reason:      top.Reason,
		ReasonCodes: top.ReasonCodes,
		NextHint:    top.NextHint,
		Summary:     top.Summary,
		Action:      actionNone,
		Lots:        lots,
	}
}

func decideLot(symbol string, lot papertrading.HoldingEvalLotRow, stock papertrading.HoldingEvalStockRow, policy Policy) HoldingDecision {
	ev := Evidence{
		CurrentPrice: lot.CurrentPrice,
		ReturnRate:   lot.ReturnRate,
		HoldingDays:  lot.HoldingDays,
		ProfitState:  lot.ProfitState,
		PeriodState:  lot.HoldingPeriodState,
		QuoteSource:  lot.QuoteSource,
		RiskState:    stock.RiskState,
	}
	if ev.ProfitState == "" {
		ev.ProfitState = papertrading.ClassifyProfitState(lot.ReturnRate)
	}
	if lot.ReturnRate != nil {
		ev.RiskState = papertrading.ClassifyRiskState(lot.ReturnRate, papertrading.DefaultRiskStateThresholds)
	}
	return decideFacts(symbol, lot.FillID, lot.PlanID, ev, policy)
}

func factsFromStock(row papertrading.HoldingEvalStockRow) Evidence {
	return Evidence{
		CurrentPrice: row.CurrentPrice,
		ReturnRate:   row.UnrealizedReturn,
		HoldingDays:  row.HoldingDays,
		RiskState:    row.RiskState,
		ProfitState:  row.ProfitState,
		PeriodState:  row.HoldingPeriodState,
		QuoteSource:  row.QuoteSource,
	}
}

func decideFacts(symbol string, fillID, planID uint, ev Evidence, policy Policy) HoldingDecision {
	d := HoldingDecision{
		Symbol:      symbol,
		FillID:      fillID,
		PlanID:      planID,
		State:       StateHoldNormal,
		Reason:      ReasonNone,
		ReasonCodes: []string{},
		NextHint:    HintContinueHold,
		Action:      actionNone,
		Evidence:    ev,
	}

	missing := ev.CurrentPrice == nil && ev.ReturnRate == nil
	if missing {
		d.Reason = ReasonDataMissing
		d.ReasonCodes = []string{ReasonDataMissing}
		d.Summary = "现价与收益率缺失，维持常规持有观察，不提高决策等级"
		return d
	}

	risk := strings.ToUpper(strings.TrimSpace(ev.RiskState))
	profit := strings.ToUpper(strings.TrimSpace(ev.ProfitState))
	if profit == "" {
		profit = papertrading.ClassifyProfitState(ev.ReturnRate)
	}
	if risk == "" && ev.ReturnRate != nil {
		risk = papertrading.ClassifyRiskState(ev.ReturnRate, papertrading.DefaultRiskStateThresholds)
	}

	if risk == papertrading.RiskStateDanger {
		d.State = StateHoldReview
		d.Reason = ReasonRiskMaterial
		d.ReasonCodes = append(d.ReasonCodes, ReasonRiskMaterial)
		d.NextHint = HintReassessThesis
		d.Summary = "浮亏触及明显风险区间，需要重新评估持仓逻辑（非卖出指令）"
	} else if risk == papertrading.RiskStateWatch {
		d.State = StateHoldWatch
		d.Reason = ReasonRiskIncrease
		d.ReasonCodes = append(d.ReasonCodes, ReasonRiskIncrease)
		d.NextHint = HintKeepWatching
		d.Summary = "风险标签上升，继续观察（非卖出指令）"
	} else if profit == papertrading.ProfitStateLoss {
		d.State = StateHoldWatch
		d.Reason = ReasonProfitWeakness
		d.ReasonCodes = append(d.ReasonCodes, ReasonProfitWeakness)
		d.NextHint = HintKeepWatching
		d.Summary = "出现浮亏弱势，继续观察（非卖出指令）"
	} else {
		d.Summary = "持有表现在常规区间，维持持有观察"
	}

	if policy.ExitCandidateEnabled && d.State == StateHoldReview &&
		strings.ToUpper(ev.PeriodState) == papertrading.HoldingPeriodLong &&
		risk == papertrading.RiskStateDanger {
		d.State = StateExitCandidate
		d.NextHint = HintConsiderExitEval
		d.Summary = "达到退出评估候选门槛，仍不是卖出指令"
	}

	if len(d.ReasonCodes) == 0 {
		d.ReasonCodes = []string{ReasonNone}
	}
	return d
}

func rankState(s string) int {
	switch s {
	case StateExitCandidate:
		return 3
	case StateHoldReview:
		return 2
	case StateHoldWatch:
		return 1
	default:
		return 0
	}
}
