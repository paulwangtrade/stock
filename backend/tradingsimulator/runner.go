package tradingsimulator

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingautomation"
	"go-stock/backend/tradingwindow"
)

const (
	simActor = "sim:trading_workflow"
)

var simLoc = time.FixedZone("CST", 8*3600)

// RunTradingDaySimulation orchestrates one simulated trading day by calling
// existing production functions with an injected clock. It is not a trading engine.
func RunTradingDaySimulation(req SimulationRequest) (*SimulationResult, error) {
	date := strings.TrimSpace(req.Date)
	if date == "" {
		date = "2026-08-17"
	}
	scenario := strings.ToUpper(strings.TrimSpace(req.Scenario))
	if scenario == "" {
		scenario = ScenarioNormal
	}
	mode := strings.ToUpper(strings.TrimSpace(req.AutomationMode))
	switch scenario {
	case ScenarioManualMode:
		mode = tradingautomation.ModeManual
	case ScenarioNormal, ScenarioLateFreeze, ScenarioRiskBlock:
		if mode == "" {
			mode = tradingautomation.ModeAuto
		}
	case ScenarioNoPlan:
		if mode == "" {
			mode = tradingautomation.ModeAuto
		}
	default:
		mode = tradingautomation.ModeAuto
	}

	tradingautomation.SetConfigForTest(tradingautomation.Config{
		AutomationMode:  mode,
		MaterializeTime: "09:20",
		ApprovalTime:    "09:25",
		FreezeTime:      "09:29:00",
		FreezeDeadline:  "09:29:30",
	})
	defer tradingautomation.ResetConfigCache()
	applyScenarioRiskConfig(scenario)
	defer data.ResetPaperOpenBuyConfigCache()

	clock := NewClock(date, "09:00:00", simLoc)
	out := &SimulationResult{
		Date:     date,
		Scenario: scenario,
		Mode:     mode,
		Timeline: make([]SimulationEvent, 0, 12),
		Overall:  StatusPass,
	}

	static := papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.05, LimitUp: 11.0}},
	}

	switch scenario {
	case ScenarioNoPlan:
		// no seed
	default:
		if _, err := seedMaterializedDraft(date); err != nil {
			return nil, err
		}
	}

	clock.AdvanceTo("09:20:00")
	mat := tradingautomation.RunMaterializeAutomation(date, clock.Now())
	appendAutomationEvent(out, clock, mat, mapMaterializeEvent)
	out.Morning = scoreFromStep(mat, true)

	clock.AdvanceTo("09:25:00")
	appr := tradingautomation.RunApprovalAutomation(date, clock.Now())
	appendAutomationEvent(out, clock, appr, mapApproveEvent)
	out.Approve = scoreFromStep(appr, scenario != ScenarioRiskBlock)

	switch scenario {
	case ScenarioLateFreeze:
		runLateFreezeBranch(out, clock, date, static)
	case ScenarioManualMode, ScenarioRiskBlock, ScenarioNoPlan:
		clock.AdvanceTo("09:29:00")
		fr := tradingautomation.RunFreezeAutomation(date, clock.Now())
		appendAutomationEvent(out, clock, fr, mapFreezeEvent)
		out.Freeze = scoreFromStep(fr, false)
		runExecutionSettlement(out, clock, date, static, papertrading.TriggerCron, "cron")
	default: // NORMAL
		clock.AdvanceTo("09:29:00")
		fr := tradingautomation.RunFreezeAutomation(date, clock.Now())
		appendAutomationEvent(out, clock, fr, mapFreezeEvent)
		out.Freeze = scoreFromStep(fr, true)
		recordWindow(out, date, clock.Now())
		runExecutionSettlement(out, clock, date, static, papertrading.TriggerCron, "cron")
	}

	foldOverall(out, scenario)
	logger.SugaredLogger.Infof(
		"TradingDaySimulation date=%s scenario=%s mode=%s overall=%s orders=%d fills=%d",
		out.Date, out.Scenario, out.Mode, out.Overall, out.Orders, out.Fills,
	)
	return out, nil
}

func runLateFreezeBranch(out *SimulationResult, clock *Clock, date string, price papertrading.PriceProvider) {
	clock.AdvanceTo("11:12:00")
	plan, _ := lookupTodayPlan(date)
	if plan == nil || !isApproved(plan) {
		out.Freeze = StepScore{Status: StatusFail, Reason: "NOT_APPROVED"}
		appendEvent(out, clock, EventAutoFreezeFailed, StatusFail, "NOT_APPROVED", 0)
		return
	}
	got, err := strategy.FreezeTradePlanAt(plan.ID, simActor, "sim late freeze", clock.Now())
	if err != nil {
		out.Freeze = StepScore{Status: StatusFail, Reason: err.Error()}
		appendEvent(out, clock, EventAutoFreezeFailed, StatusFail, err.Error(), plan.ID)
		return
	}
	out.Freeze = StepScore{Status: StatusPass, Reason: "late_freeze"}
	appendEvent(out, clock, EventAutoFreezeSuccess, StatusPass, "late_freeze", got.ID)

	window := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate:   date,
		CurrentTime: clock.Now(),
		PlanStatus:  got.Status,
		IsFrozen:    got.IsFrozen(),
		FrozenTime:  got.FreezeAt,
	})
	out.WindowStatus = string(window.Status)
	out.WindowReason = window.Reason
	if window.Status == tradingwindow.StatusMissedOpenWindow {
		appendEvent(out, clock, EventMissedOpenWindow, StatusPass, window.Reason, got.ID)
	} else {
		appendEvent(out, clock, EventMissedOpenWindow, StatusFail, string(window.Status), got.ID)
	}

	execRes, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        date,
		PlanID:           got.ID,
		Trigger:          papertrading.TriggerManual,
		Actor:            simActor,
		Price:            price,
		SkipWeekdayCheck: true,
		Now:              clock.Now(),
	})
	recordExecution(out, clock, execRes, err, true)
	clock.AdvanceTo("15:05:00")
	recordSettlement(out, date, price)
}

func runExecutionSettlement(out *SimulationResult, clock *Clock, date string, price papertrading.PriceProvider, trigger, actor string) {
	clock.AdvanceTo("09:31:00")
	execRes, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		TradeDate:        date,
		Trigger:          trigger,
		Actor:            actor,
		Price:            price,
		SkipWeekdayCheck: true,
		Now:              clock.Now(),
	})
	expectSuccess := out.Scenario == ScenarioNormal
	recordExecution(out, clock, execRes, err, expectSuccess)
	clock.AdvanceTo("15:05:00")
	recordSettlement(out, date, price)
}

func recordExecution(out *SimulationResult, clock *Clock, execRes *papertrading.ExecutionResult, err error, expectSuccess bool) {
	if err != nil {
		out.Execution = StepScore{Status: StatusFail, Reason: err.Error()}
		appendEvent(out, clock, EventExecutionSkipped, StatusFail, err.Error(), 0)
		return
	}
	if execRes == nil {
		out.Execution = StepScore{Status: StatusFail, Reason: "nil execution result"}
		appendEvent(out, clock, EventExecutionSkipped, StatusFail, "nil result", 0)
		return
	}
	out.Orders = execRes.OrdersTotal
	out.Fills = execRes.FilledCount
	reason := execRes.Status
	if execRes.Message != "" {
		reason = execRes.Status + ": " + execRes.Message
	}
	skipped := strings.HasPrefix(execRes.Status, "skipped_")
	if skipped {
		out.Execution = StepScore{Status: StatusSkip, Reason: reason}
		st := StatusPass
		if expectSuccess {
			st = StatusFail
		}
		appendEvent(out, clock, EventExecutionSkipped, st, reason, execRes.PlanID)
		if expectSuccess {
			out.Execution.Status = StatusFail
		}
		return
	}
	out.Execution = StepScore{Status: StatusPass, Reason: execRes.Status}
	appendEvent(out, clock, EventExecutionSuccess, StatusPass, execRes.Status, execRes.PlanID)
}

func recordSettlement(out *SimulationResult, date string, price papertrading.PriceProvider) {
	clock := NewClock(date, "15:05:00", simLoc)
	res, err := papertrading.SettlementJob(date, price, false)
	if err != nil {
		out.Settlement = StepScore{Status: StatusFail, Reason: err.Error()}
		appendEvent(out, clock, EventSettlementSkipped, StatusFail, err.Error(), 0)
		return
	}
	if res == nil {
		out.Settlement = StepScore{Status: StatusSkip, Reason: "nil settlement"}
		appendEvent(out, clock, EventSettlementSkipped, StatusSkip, "nil", 0)
		return
	}
	if !res.Enabled || strings.Contains(res.Message, "no paper_sim") || strings.Contains(res.Message, "disabled") {
		out.Settlement = StepScore{Status: StatusSkip, Reason: res.Message}
		appendEvent(out, clock, EventSettlementSkipped, StatusPass, res.Message, 0)
		return
	}
	out.Settlement = StepScore{Status: StatusPass, Reason: res.Message}
	appendEvent(out, clock, EventSettlementSuccess, StatusPass, res.Message, 0)
}

func recordWindow(out *SimulationResult, date string, now time.Time) {
	plan, _ := lookupTodayPlan(date)
	in := tradingwindow.PlanWindowInput{TradeDate: date, CurrentTime: now}
	if plan != nil {
		in.PlanStatus = plan.Status
		in.IsFrozen = plan.IsFrozen()
		in.FrozenTime = plan.FreezeAt
	}
	w := tradingwindow.EvaluatePlanWindow(in)
	out.WindowStatus = string(w.Status)
	out.WindowReason = w.Reason
}

func seedMaterializedDraft(tradeDate string) (*models.TradePlan, error) {
	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		PlanVersion:          1,
		PricingPolicyVersion: 1,
		PricingStage:         "morning_materialized",
		SourceSession:        models.TradePlanSourceAfterClose,
		Side:                 "buy",
		AmountPerStock:       100_000,
		MaxNames:             5,
		RiskStatus:           "passed",
	}
	items := []models.TradePlanItem{{
		TradeDate:    tradeDate,
		StockCode:    "sz000001",
		StockName:    "平安银行",
		Side:         "buy",
		Priority:     1,
		TargetAmount: 100_000,
		Status:       models.TradePlanItemPending,
		IntentStatus: "priced",
		LimitPrice:   10,
		TargetVolume: 1000,
		RefPrice:     10,
	}}
	if err := data.NewTradePlanRepo().CreatePlanWithItems(plan, items); err != nil {
		return nil, err
	}
	return data.NewTradePlanRepo().GetByID(plan.ID)
}

func lookupTodayPlan(tradeDate string) (*models.TradePlan, error) {
	repo := data.NewTradePlanRepo()
	if frozen, err := repo.GetFrozenByTradeDate(tradeDate); err == nil && frozen != nil {
		return frozen, nil
	}
	return repo.GetLatestByTradeDate(tradeDate)
}

func applyScenarioRiskConfig(scenario string) {
	cfg := data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:       false,
		OpenBuyAmountPerStock:    100_000,
		EnableRiskFilter:         false,
		PlanMarketLevel:          3,
		BlockNewEntriesOnDefense: false,
	}
	if scenario == ScenarioRiskBlock {
		cfg.EnableRiskFilter = true
		cfg.PlanMarketLevel = 2
		cfg.BlockNewEntriesOnDefense = true
	}
	_ = data.SavePaperOpenBuyConfig(cfg)
	data.ResetPaperOpenBuyConfigCache()
}

func isApproved(plan *models.TradePlan) bool {
	return plan != nil && plan.ApprovedAt != nil && !plan.ApprovedAt.IsZero()
}

func appendEvent(out *SimulationResult, clock *Clock, event, status, reason string, planID uint) {
	now := clock.Now()
	sess := string(papertrading.ResolveExecutionSession(now))
	out.Timeline = append(out.Timeline, SimulationEvent{
		Timestamp: now,
		Session:   sess,
		Event:     event,
		Status:    status,
		Reason:    reason,
		PlanID:    planID,
	})
}

func appendAutomationEvent(out *SimulationResult, clock *Clock, step tradingautomation.StepResult, mapFn func(tradingautomation.StepResult) (string, string)) {
	ev, st := mapFn(step)
	appendEvent(out, clock, ev, st, step.Reason, step.PlanID)
}

func mapMaterializeEvent(step tradingautomation.StepResult) (string, string) {
	switch step.Outcome {
	case tradingautomation.OutcomeMaterializationAutoSuccess:
		return EventAutoMaterializeSuccess, StatusPass
	case tradingautomation.OutcomeMaterializationSkipped:
		return EventMaterializationSkipped, StatusPass
	default:
		return EventMaterializationFailed, StatusFail
	}
}

func mapApproveEvent(step tradingautomation.StepResult) (string, string) {
	switch step.Outcome {
	case tradingautomation.OutcomeAutoApprovalSuccess:
		return EventAutoApproveSuccess, StatusPass
	case tradingautomation.OutcomeAutoApprovalSkipped:
		return EventAutoApprovalSkipped, StatusPass
	default:
		return EventAutoApprovalBlocked, StatusPass
	}
}

func mapFreezeEvent(step tradingautomation.StepResult) (string, string) {
	switch step.Outcome {
	case tradingautomation.OutcomeAutoFreezeSuccess:
		return EventAutoFreezeSuccess, StatusPass
	case tradingautomation.OutcomeAutoFreezeSkipped:
		return EventAutoFreezeSuccess, StatusPass
	case tradingautomation.OutcomeAutoFreezeAfterDeadline:
		return EventAutoFreezeAfterDeadline, StatusPass
	default:
		return EventAutoFreezeFailed, StatusPass
	}
}

func scoreFromStep(step tradingautomation.StepResult, requireOK bool) StepScore {
	if requireOK {
		if step.OK {
			return StepScore{Status: StatusPass, Reason: step.Outcome}
		}
		return StepScore{Status: StatusFail, Reason: step.Outcome + " " + step.Reason}
	}
	if step.OK {
		return StepScore{Status: StatusPass, Reason: step.Outcome}
	}
	return StepScore{Status: StatusSkip, Reason: step.Outcome + " " + step.Reason}
}

func foldOverall(out *SimulationResult, scenario string) {
	switch scenario {
	case ScenarioLateFreeze:
		if out.WindowStatus != string(tradingwindow.StatusMissedOpenWindow) {
			out.Overall = StatusFail
			out.FailureReason = "expected MISSED_OPEN_WINDOW, got " + out.WindowStatus
			return
		}
		if out.Execution.Status == StatusFail {
			out.Overall = StatusFail
			out.FailureReason = out.Execution.Reason
			return
		}
		out.Overall = StatusPass
	case ScenarioNoPlan:
		if !hasEvent(out, EventExecutionSkipped) {
			out.Overall = StatusFail
			out.FailureReason = "expected EXECUTION_SKIPPED"
			return
		}
		out.Overall = StatusPass
	case ScenarioRiskBlock:
		if !hasEvent(out, EventAutoApprovalBlocked) {
			out.Overall = StatusFail
			out.FailureReason = "expected AUTO_APPROVAL_BLOCKED"
			return
		}
		out.Overall = StatusPass
	case ScenarioManualMode:
		if !hasEvent(out, EventMaterializationSkipped) && !hasEvent(out, EventAutoApprovalSkipped) {
			out.Overall = StatusFail
			out.FailureReason = "expected MANUAL skip outcomes"
			return
		}
		out.Overall = StatusPass
	default:
		if out.Morning.Status != StatusPass || out.Approve.Status != StatusPass || out.Freeze.Status != StatusPass {
			out.Overall = StatusFail
			out.FailureReason = firstFail(out)
			return
		}
		if out.Execution.Status == StatusFail {
			out.Overall = StatusFail
			out.FailureReason = out.Execution.Reason
			return
		}
		out.Overall = StatusPass
	}
}

func hasEvent(out *SimulationResult, name string) bool {
	for _, e := range out.Timeline {
		if e.Event == name {
			return true
		}
	}
	return false
}

func firstFail(out *SimulationResult) string {
	for _, s := range []StepScore{out.Morning, out.Approve, out.Freeze, out.Execution} {
		if s.Status == StatusFail {
			return s.Reason
		}
	}
	return ""
}
