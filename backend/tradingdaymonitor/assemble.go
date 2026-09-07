package tradingdaymonitor

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/morningpreparation"
	"go-stock/backend/models"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/tradingevent"
)

const (
	dataSourceNote = "Trading Day Monitor · TradingEvent + readiness + PositionState (K-Beta); no DB writes for monitor itself"
	disclaimer     = "只读交易日观察。不修改 Gateway / Automation / Settlement。持仓状态来自 PositionState，不自判新仓。"
)

// Options selects the business day to project.
type Options struct {
	TradeDate string
	AsOf      time.Time
	// Events overrides buffer read (tests). nil → tradingevent.ListByTradeDate.
	Events []tradingevent.TradingEvent
	// Plan overrides plan lookup (tests). When unset and DisablePlanLookup is false, loads from TradePlanRepo.
	Plan *models.TradePlan
	// DisablePlanLookup skips TradePlanRepo when Plan is nil (tests: force PLAN_NOT_FOUND aux).
	DisablePlanLookup bool
	// SkipReadiness skips auxiliary plan/readiness fill (tests).
	SkipReadiness bool
	// SkipPositionStates skips PositionState attach (pure unit tests).
	SkipPositionStates bool
	// PositionStates injects prebuilt rows (tests).
	PositionStates []PositionStateRow
}

// Build assembles TradingDayMonitorView (read-only).
func Build(opts Options) *TradingDayMonitorView {
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	tradeDate := strings.TrimSpace(opts.TradeDate)
	if tradeDate == "" {
		tradeDate = asOf.Format("2006-01-02")
	}

	events := opts.Events
	if events == nil {
		events = tradingevent.ListByTradeDate(tradeDate)
	}

	out := &TradingDayMonitorView{
		TradeDate:      tradeDate,
		AsOf:           asOf,
		Morning: MorningSection{
			Materialize: pendingStep(),
			Approve:     pendingStep(),
			Freeze:      pendingStep(),
		},
		Execution: ExecutionSection{
			Status: StatusPending,
			Source: SourceTradingEvent,
		},
		Settlement: SettlementSection{
			Status: StatusPending,
			Source: SourceTradingEvent,
		},
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}

	applyEvents(out, events)

	if !opts.SkipReadiness {
		plan := opts.Plan
		if plan == nil && !opts.DisablePlanLookup {
			plan = loadPlan(tradeDate)
		}
		applyReadinessAux(out, tradeDate, asOf, plan)
	}

	if opts.PositionStates != nil {
		out.PositionStates = opts.PositionStates
	} else if !opts.SkipPositionStates {
		attachPositionStates(out, tradeDate, asOf)
	}
	return out
}

func attachPositionStates(out *TradingDayMonitorView, tradeDate string, asOf time.Time) {
	bundle := positionstate.NewService(nil).Evaluate(positionstate.Query{TradeDate: tradeDate, AsOf: asOf})
	if bundle == nil {
		return
	}
	out.PositionStates = make([]PositionStateRow, 0, len(bundle.Positions))
	for _, p := range bundle.Positions {
		out.PositionStates = append(out.PositionStates, PositionStateRow{
			Symbol:        p.Symbol,
			State:         p.State,
			TotalQty:      p.TotalQty,
			AvailableQty:  p.AvailableQty,
			LockedQty:     p.LockedQty,
			IsNewPosition: p.IsNewPosition,
			CanSell:       p.CanSell,
			HoldingDays:   p.HoldingDays,
			RiskTag:       p.RiskTag,
		})
	}
}

func pendingStep() StepStatus {
	return StepStatus{Status: StatusPending, Source: SourceTradingEvent}
}

func applyEvents(out *TradingDayMonitorView, events []tradingevent.TradingEvent) {
	for _, ev := range events {
		switch ev.EventType {
		case tradingevent.EventMaterializeSuccess, tradingevent.EventMaterializeFailed:
			out.Morning.Materialize = stepFromEvent(ev)
		case tradingevent.EventApproveSuccess, tradingevent.EventApproveBlocked:
			out.Morning.Approve = stepFromEvent(ev)
		case tradingevent.EventFreezeSuccess, tradingevent.EventFreezeBlocked:
			out.Morning.Freeze = stepFromEvent(ev)
		case tradingevent.EventExecutionStarted,
			tradingevent.EventExecutionSkipped,
			tradingevent.EventExecutionCompleted,
			tradingevent.EventExecutionFailed:
			// Keep STARTED only if nothing stronger yet; later events overwrite.
			if out.Execution.EventType == tradingevent.EventExecutionCompleted ||
				out.Execution.EventType == tradingevent.EventExecutionFailed ||
				out.Execution.EventType == tradingevent.EventExecutionSkipped {
				if ev.EventType == tradingevent.EventExecutionStarted {
					continue
				}
			}
			out.Execution = ExecutionSection{
				Session:   ev.Session,
				Status:    mapExecStatus(ev),
				PlanID:    ev.PlanID,
				Reason:    ev.Reason,
				EventType: ev.EventType,
				Source:    SourceTradingEvent,
			}
		case tradingevent.EventSettlementCompleted, tradingevent.EventSettlementFailed:
			out.Settlement = SettlementSection{
				Status:    mapSettleStatus(ev),
				Reason:    ev.Reason,
				EventType: ev.EventType,
				Source:    SourceTradingEvent,
			}
		}
	}
}

func stepFromEvent(ev tradingevent.TradingEvent) StepStatus {
	st := ev.Status
	if st == "" {
		if strings.Contains(ev.EventType, "FAIL") || strings.Contains(ev.EventType, "BLOCK") {
			st = StatusFail
		} else {
			st = StatusPass
		}
	}
	return StepStatus{
		Status:    st,
		Reason:    ev.Reason,
		PlanID:    ev.PlanID,
		EventType: ev.EventType,
		Source:    SourceTradingEvent,
	}
}

func mapExecStatus(ev tradingevent.TradingEvent) string {
	switch ev.EventType {
	case tradingevent.EventExecutionFailed:
		return StatusFail
	case tradingevent.EventExecutionSkipped:
		return StatusSkip
	case tradingevent.EventExecutionCompleted:
		return StatusPass
	case tradingevent.EventExecutionStarted:
		return StatusPending
	default:
		if ev.Status != "" {
			return ev.Status
		}
		return StatusUnknown
	}
}

func mapSettleStatus(ev tradingevent.TradingEvent) string {
	switch ev.EventType {
	case tradingevent.EventSettlementFailed:
		return StatusFail
	case tradingevent.EventSettlementCompleted:
		return StatusPass
	default:
		return StatusUnknown
	}
}

// applyReadinessAux fills PENDING morning steps from persisted plan / morning readiness.
// Does NOT call tradingautomation.LastSteps.
func applyReadinessAux(out *TradingDayMonitorView, tradeDate string, now time.Time, plan *models.TradePlan) {
	if plan == nil {
		if out.Morning.Materialize.Status == StatusPending {
			out.Morning.Materialize = StepStatus{
				Status: StatusUnknown, Reason: "PLAN_NOT_FOUND", Source: SourceReadiness,
			}
		}
		if out.Morning.Approve.Status == StatusPending {
			out.Morning.Approve = StepStatus{
				Status: StatusUnknown, Reason: "PLAN_NOT_FOUND", Source: SourceReadiness,
			}
		}
		if out.Morning.Freeze.Status == StatusPending {
			out.Morning.Freeze = StepStatus{
				Status: StatusUnknown, Reason: "PLAN_NOT_FOUND", Source: SourceReadiness,
			}
		}
		return
	}

	obs := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: tradeDate,
		Now:       now,
		Plan:      plan,
	})

	if out.Morning.Materialize.Status == StatusPending {
		out.Morning.Materialize = StepStatus{
			Status: mapCheckStatus(obs.MaterializationStatus),
			Reason: obs.Reason,
			PlanID: plan.ID,
			Source: SourceReadiness,
		}
	}
	if out.Morning.Approve.Status == StatusPending {
		st := StatusPending
		reason := ""
		if plan.ApprovedAt != nil && !plan.ApprovedAt.IsZero() {
			st = StatusPass
			reason = "approved_at_persisted"
		}
		out.Morning.Approve = StepStatus{
			Status: st, Reason: reason, PlanID: plan.ID, Source: SourceReadiness,
		}
	}
	if out.Morning.Freeze.Status == StatusPending {
		st := mapCheckStatus(obs.FreezeStatus)
		reason := obs.Reason
		if plan.IsFrozen() {
			st = StatusPass
			reason = "frozen_persisted"
		}
		out.Morning.Freeze = StepStatus{
			Status: st, Reason: reason, PlanID: plan.ID, Source: SourceReadiness,
		}
	}
	if out.Execution.PlanID == 0 {
		out.Execution.PlanID = plan.ID
	}
}

func mapCheckStatus(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case morningpreparation.CheckPass:
		return StatusPass
	case morningpreparation.CheckFail:
		return StatusFail
	case "SKIP":
		return StatusSkip
	case morningpreparation.CheckNotApplicable:
		return StatusPass
	default:
		return StatusPending
	}
}

func loadPlan(tradeDate string) *models.TradePlan {
	repo := data.NewTradePlanRepo()
	if frozen, err := repo.GetFrozenByTradeDate(tradeDate); err == nil && frozen != nil {
		return frozen
	}
	plan, err := repo.GetLatestByTradeDate(tradeDate)
	if err != nil {
		return nil
	}
	return plan
}
