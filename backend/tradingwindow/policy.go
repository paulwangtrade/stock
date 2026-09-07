package tradingwindow

import (
	"strings"
	"time"

	"go-stock/backend/models"
)

// Plan window constants (Session A open auto-pick window, fillMode=A).
const (
	OpenWindowStartHHMMSS  = "09:30:00"
	OpenWindowEndHHMMSS    = "11:30:00"
	OpenFreezeDeadlineHHMM = "09:29:30"
	// Afternoon Session A segment (manual / SessionPolicy allow; fillMode=A cron does not retry).
	AfternoonWindowStartHHMMSS = "13:00:00"
	AfternoonWindowEndHHMMSS   = "15:00:00"
)

// PlanWindowStatus is a derived open-window state; does not replace plan DB status.
type PlanWindowStatus string

const (
	StatusReadyForOpen     PlanWindowStatus = "READY_FOR_OPEN"
	StatusMissedOpenWindow PlanWindowStatus = "MISSED_OPEN_WINDOW"
	StatusOpenExecutable   PlanWindowStatus = "OPEN_EXECUTABLE"
	StatusExpired          PlanWindowStatus = "EXPIRED"
)

// PlanWindowInput drives derived window evaluation.
type PlanWindowInput struct {
	TradeDate   string
	CurrentTime time.Time
	PlanStatus  string
	IsFrozen    bool
	FrozenTime  *time.Time
}

// PlanWindowResult is the outcome of EvaluatePlanWindow.
type PlanWindowResult struct {
	Status                PlanWindowStatus
	Reason                string
	OpenWindowStart       string
	OpenWindowEnd         string
	FreezeDeadline        string
	BlocksAutoExecution   bool
	BlocksManualExecution bool
}

// EvaluatePlanWindow derives open-window status from trade date, clock, and freeze facts.
// Phase A: pure function; does not mutate plan state or block manual execution.
func EvaluatePlanWindow(in PlanWindowInput) PlanWindowResult {
	loc := in.CurrentTime.Location()
	tradeDay, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(in.TradeDate), loc)
	if err != nil {
		return PlanWindowResult{
			Status:                StatusExpired,
			Reason:                ReasonNoExecutionWindow,
			OpenWindowStart:       OpenWindowStartHHMMSS,
			OpenWindowEnd:         OpenWindowEndHHMMSS,
			FreezeDeadline:        OpenFreezeDeadlineHHMM,
			BlocksAutoExecution:   true,
			BlocksManualExecution: false,
		}
	}

	deadline := clockOnDay(tradeDay, OpenFreezeDeadlineHHMM, loc)
	morningStart := clockOnDay(tradeDay, OpenWindowStartHHMMSS, loc)
	morningEnd := clockOnDay(tradeDay, OpenWindowEndHHMMSS, loc)
	afternoonStart := clockOnDay(tradeDay, AfternoonWindowStartHHMMSS, loc)
	afternoonEnd := clockOnDay(tradeDay, AfternoonWindowEndHHMMSS, loc)

	base := PlanWindowResult{
		OpenWindowStart:       OpenWindowStartHHMMSS,
		OpenWindowEnd:         OpenWindowEndHHMMSS,
		FreezeDeadline:        OpenFreezeDeadlineHHMM,
		BlocksManualExecution: false,
	}

	now := in.CurrentTime
	if !sameCalendarDay(now, tradeDay) {
		if now.Before(tradeDay) {
			return applyNotFrozenBeforeDeadline(base, now, deadline, in.IsFrozen, in.FrozenTime)
		}
		out := base
		out.Status = StatusExpired
		out.Reason = ReasonNoExecutionWindow
		out.BlocksAutoExecution = true
		return out
	}

	if isTerminalPlanStatus(in.PlanStatus) {
		out := base
		out.Status = StatusExpired
		out.Reason = ReasonNoExecutionWindow
		out.BlocksAutoExecution = true
		return out
	}

	if in.IsFrozen && in.FrozenTime != nil && !in.FrozenTime.IsZero() {
		return evaluateFrozen(base, now, *in.FrozenTime, deadline, morningStart, morningEnd, afternoonStart, afternoonEnd)
	}
	return applyNotFrozenBeforeDeadline(base, now, deadline, in.IsFrozen, in.FrozenTime)
}

func evaluateFrozen(
	base PlanWindowResult,
	now, frozen, deadline, morningStart, morningEnd, afternoonStart, afternoonEnd time.Time,
) PlanWindowResult {
	out := base
	if frozen.After(deadline) {
		out.Status = StatusMissedOpenWindow
		out.Reason = ReasonPlanFrozenAfterDeadline
		out.BlocksAutoExecution = true
		if inOpenSessionA(now, morningStart, morningEnd, afternoonStart, afternoonEnd) {
			return out
		}
		if now.Before(morningStart) {
			out.Status = StatusMissedOpenWindow
			return out
		}
		out.Status = StatusExpired
		out.Reason = ReasonNoExecutionWindow
		return out
	}

	if inOpenSessionA(now, morningStart, morningEnd, afternoonStart, afternoonEnd) {
		out.Status = StatusOpenExecutable
		out.Reason = ""
		out.BlocksAutoExecution = false
		return out
	}

	if now.Before(morningStart) {
		out.Status = StatusReadyForOpen
		out.Reason = ""
		out.BlocksAutoExecution = false
		return out
	}

	if !now.Before(afternoonEnd) {
		out.Status = StatusExpired
		out.Reason = ReasonNoExecutionWindow
		out.BlocksAutoExecution = true
		return out
	}

	// Lunch break or after morning window: frozen on time, waiting for afternoon segment.
	out.Status = StatusReadyForOpen
	out.Reason = ""
	out.BlocksAutoExecution = false
	return out
}

func applyNotFrozenBeforeDeadline(
	base PlanWindowResult,
	now, deadline time.Time,
	isFrozen bool,
	frozenTime *time.Time,
) PlanWindowResult {
	out := base
	if isFrozen && frozenTime != nil && !frozenTime.IsZero() {
		return evaluateFrozen(
			base, now, *frozenTime, deadline,
			clockOnDay(*frozenTime, OpenWindowStartHHMMSS, now.Location()),
			clockOnDay(*frozenTime, OpenWindowEndHHMMSS, now.Location()),
			clockOnDay(*frozenTime, AfternoonWindowStartHHMMSS, now.Location()),
			clockOnDay(*frozenTime, AfternoonWindowEndHHMMSS, now.Location()),
		)
	}
	if now.After(deadline) {
		out.Status = StatusMissedOpenWindow
		out.Reason = ReasonPlanNotFrozenBeforeOpen
		out.BlocksAutoExecution = true
		return out
	}
	out.Status = StatusReadyForOpen
	out.Reason = ReasonPlanNotFrozenBeforeOpen
	out.BlocksAutoExecution = false
	return out
}

func inOpenSessionA(now, morningStart, morningEnd, afternoonStart, afternoonEnd time.Time) bool {
	inMorning := !now.Before(morningStart) && now.Before(morningEnd)
	inAfternoon := !now.Before(afternoonStart) && now.Before(afternoonEnd)
	return inMorning || inAfternoon
}

func isTerminalPlanStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case models.TradePlanStatusDone, models.TradePlanStatusPartial, models.TradePlanStatusSkipped,
		models.TradePlanStatusFailed, models.TradePlanStatusSuperseded:
		return true
	default:
		return false
	}
}

func clockOnDay(day time.Time, hhmmss string, loc *time.Location) time.Time {
	t, err := time.ParseInLocation("15:04:05", hhmmss, loc)
	if err != nil {
		return day
	}
	y, m, d := day.Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, loc)
}

func sameCalendarDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// BlocksManualRunExecution reports whether manual RunExecution should be blocked.
// Phase A: always false — auto cron may be constrained; manual path is preserved.
func BlocksManualRunExecution(_ PlanWindowResult) bool {
	return false
}
