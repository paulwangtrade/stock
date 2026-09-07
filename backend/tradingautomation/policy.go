package tradingautomation

import (
	"strings"
	"time"

	"go-stock/backend/approvegate"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/morningpreparation"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingevent"
	"go-stock/backend/tradingwindow"
)

const (
	actorAutomation  = "cron:trading_automation"
	sourceAutomation = "trading_automation"
)

// StepResult is one automation step outcome (observation only in DB).
type StepResult struct {
	Step    string `json:"step"`
	OK      bool   `json:"ok"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
	PlanID  uint   `json:"plan_id,omitempty"`
}

// MorningAutomationObservation derived UI snapshot for today's morning automation.
type MorningAutomationObservation struct {
	Mode                       string `json:"mode"`
	Materialization            string `json:"materialization"`
	Approval                   string `json:"approval"`
	Freeze                     string `json:"freeze"`
	MaterializationReason      string `json:"materializationReason,omitempty"`
	ApprovalReason             string `json:"approvalReason,omitempty"`
	FreezeReason               string `json:"freezeReason,omitempty"`
	LastMaterializationOutcome string `json:"lastMaterializationOutcome,omitempty"`
	LastApprovalOutcome        string `json:"lastApprovalOutcome,omitempty"`
	LastFreezeOutcome          string `json:"lastFreezeOutcome,omitempty"`
}

type planLookupFn func(tradeDate string) (*models.TradePlan, error)
type materializeRunner func(tradeDate string, now time.Time) (bool, string)

var (
	lookupTodayPlanFn                      = defaultLookupTodayPlan
	runMaterializeStepFn materializeRunner = defaultRunMaterialize
)

// RunMaterializeAutomation runs T1 materialize when mode is ASSISTED or AUTO.
func RunMaterializeAutomation(tradeDate string, now time.Time) StepResult {
	cfg := GetConfig()
	if !ShouldAutoMaterialize() {
		return StepResult{Step: "materialize", OK: true, Outcome: OutcomeMaterializationSkipped, Reason: "mode=" + cfg.AutomationMode}
	}
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = now.Format("2006-01-02")
	}
	ok, reason := runMaterializeStepFn(tradeDate, now)
	outcome := OutcomeMaterializationAutoSuccess
	if !ok {
		outcome = OutcomeMaterializationFailed
	}
	plan, _ := lookupTodayPlanFn(tradeDate)
	res := StepResult{Step: "materialize", OK: ok, Outcome: outcome, Reason: reason}
	if plan != nil {
		res.PlanID = plan.ID
	}
	logger.SugaredLogger.Infof("TradingAutomation materialize mode=%s ok=%t outcome=%s reason=%s plan_id=%d",
		cfg.AutomationMode, ok, outcome, reason, res.PlanID)
	RecordStep(res)
	emitAutomation(tradeDate, now, res)
	return res
}

// RunApprovalAutomation auto-approves draft when mode is AUTO and gates pass.
func RunApprovalAutomation(tradeDate string, now time.Time) StepResult {
	if !ShouldAutoApprove() {
		return StepResult{Step: "approve", OK: true, Outcome: OutcomeAutoApprovalSkipped, Reason: "mode=" + Mode()}
	}
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = now.Format("2006-01-02")
	}
	plan, err := lookupDraftPlanForAutomation(tradeDate)
	if err != nil || plan == nil {
		res := StepResult{Step: "approve", OK: false, Outcome: OutcomeAutoApprovalBlocked, Reason: "PLAN_NOT_FOUND"}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if plan.IsFrozen() || isApproved(plan) {
		res := StepResult{Step: "approve", OK: true, Outcome: OutcomeAutoApprovalSuccess, Reason: "already_approved_or_frozen", PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if fn := getApproveTestRunner(); fn != nil {
		ok, reason := fn(plan.ID, now)
		res := StepResult{Step: "approve", OK: ok, PlanID: plan.ID}
		if ok {
			res.Outcome = OutcomeAutoApprovalSuccess
		} else {
			res.Outcome = OutcomeAutoApprovalBlocked
			res.Reason = reason
		}
		RecordStep(res)
		emitAutomation(tradeDate, now, res)
		return res
	}
	elig, err := approvegate.CheckApproveEligibility(plan.ID, nil)
	if err != nil {
		res := StepResult{Step: "approve", OK: false, Outcome: OutcomeAutoApprovalBlocked, Reason: err.Error(), PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if !elig.Eligible || len(elig.Blockers) > 0 {
		reason := firstBlockerReason(elig.Blockers)
		if reason == "" {
			reason = "RISK_OR_READINESS_BLOCKED"
		}
		res := StepResult{Step: "approve", OK: false, Outcome: OutcomeAutoApprovalBlocked, Reason: reason, PlanID: plan.ID}
		RecordStep(res)
		emitAutomation(tradeDate, now, res)
		return res
	}
	wr, err := approvegate.ApproveTradePlanByID(plan.ID, actorAutomation, sourceAutomation, &approvegate.ApproveOptions{Now: func() time.Time { return now }})
	if err != nil {
		res := StepResult{Step: "approve", OK: false, Outcome: OutcomeAutoApprovalBlocked, Reason: err.Error(), PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if wr == nil || !wr.OK {
		reason := "approve_write_failed"
		if wr != nil && wr.Message != "" {
			reason = wr.Message
		}
		res := StepResult{Step: "approve", OK: false, Outcome: OutcomeAutoApprovalBlocked, Reason: reason, PlanID: plan.ID}
		RecordStep(res)
		emitAutomation(tradeDate, now, res)
		return res
	}
	logger.SugaredLogger.Infof("TradingAutomation approve plan_id=%d outcome=%s", plan.ID, OutcomeAutoApprovalSuccess)
	res := StepResult{Step: "approve", OK: true, Outcome: OutcomeAutoApprovalSuccess, PlanID: plan.ID}
	RecordStep(res)
	emitAutomation(tradeDate, now, res)
	return res
}

// RunFreezeAutomation auto-freezes approved draft when mode is AUTO and before deadline.
func RunFreezeAutomation(tradeDate string, now time.Time) StepResult {
	if !ShouldAutoFreeze() {
		return StepResult{Step: "freeze", OK: true, Outcome: OutcomeAutoFreezeSkipped, Reason: "mode=" + Mode()}
	}
	if now.IsZero() {
		now = time.Now()
	}
	if strings.TrimSpace(tradeDate) == "" {
		tradeDate = now.Format("2006-01-02")
	}
	plan, err := lookupDraftPlanForAutomation(tradeDate)
	if err != nil || plan == nil {
		res := StepResult{Step: "freeze", OK: false, Outcome: OutcomeAutoFreezeFailed, Reason: "PLAN_NOT_FOUND"}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if plan.IsFrozen() {
		res := StepResult{Step: "freeze", OK: true, Outcome: OutcomeAutoFreezeSuccess, Reason: "already_frozen", PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if isAfterFreezeDeadline(tradeDate, now) {
		res := StepResult{
			Step: "freeze", OK: false, Outcome: OutcomeAutoFreezeAfterDeadline,
			Reason: tradingwindow.ReasonPlanNotFrozenBeforeOpen, PlanID: plan.ID,
		}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if fn := getFreezeTestRunner(); fn != nil {
		ok, reason := fn(plan.ID, now)
		res := StepResult{Step: "freeze", OK: ok, PlanID: plan.ID, Reason: reason}
		if ok {
			res.Outcome = OutcomeAutoFreezeSuccess
		} else {
			res.Outcome = OutcomeAutoFreezeFailed
		}
		RecordStep(res)
		emitAutomation(tradeDate, now, res)
		return res
	}
	if !isApproved(plan) {
		res := StepResult{Step: "freeze", OK: false, Outcome: OutcomeAutoFreezeFailed, Reason: "NOT_APPROVED", PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	elig, err := approvegate.CheckApproveEligibility(plan.ID, nil)
	if err != nil {
		res := StepResult{Step: "freeze", OK: false, Outcome: OutcomeAutoFreezeFailed, Reason: err.Error(), PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	if len(elig.Blockers) > 0 {
		res := StepResult{Step: "freeze", OK: false, Outcome: OutcomeAutoFreezeFailed, Reason: firstBlockerReason(elig.Blockers), PlanID: plan.ID}
		RecordStep(res)
		emitAutomation(tradeDate, now, res)
		return res
	}
	got, err := strategy.FreezeTradePlanAt(plan.ID, actorAutomation, "auto freeze", now)
	if err != nil {
		res := StepResult{Step: "freeze", OK: false, Outcome: OutcomeAutoFreezeFailed, Reason: err.Error(), PlanID: plan.ID}
		emitAutomation(tradeDate, now, res)
		return res
	}
	post := tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
		TradeDate: tradeDate, CurrentTime: now, PlanStatus: got.Status,
		IsFrozen: got.IsFrozen(), FrozenTime: got.FreezeAt,
	})
	reason := ""
	if post.Status == tradingwindow.StatusMissedOpenWindow {
		reason = post.Reason
		logger.SugaredLogger.Warnf("TradingAutomation freeze plan_id=%d MISSED_OPEN_WINDOW reason=%s (manual still allowed)",
			got.ID, post.Reason)
	}
	logger.SugaredLogger.Infof("TradingAutomation freeze plan_id=%d outcome=%s window=%s", got.ID, OutcomeAutoFreezeSuccess, post.Status)
	res := StepResult{Step: "freeze", OK: true, Outcome: OutcomeAutoFreezeSuccess, Reason: reason, PlanID: got.ID}
	RecordStep(res)
	emitAutomation(tradeDate, now, res)
	return res
}

// emitAutomation is observation-only; never changes StepResult or trading decisions.
func emitAutomation(tradeDate string, now time.Time, res StepResult) {
	tradingevent.EmitAutomationStep(tradeDate, now, res.Step, res.Outcome, res.Reason, res.PlanID, res.OK)
}

// EvaluateMorningAutomation derives UI observation from config + plan state + optional last steps.
func EvaluateMorningAutomation(tradeDate string, plan *models.TradePlan, lastMat, lastAppr, lastFreeze *StepResult) MorningAutomationObservation {
	cfg := GetConfig()
	obs := MorningAutomationObservation{Mode: cfg.AutomationMode}
	if lastMat != nil {
		obs.LastMaterializationOutcome = lastMat.Outcome
	}
	if lastAppr != nil {
		obs.LastApprovalOutcome = lastAppr.Outcome
	}
	if lastFreeze != nil {
		obs.LastFreezeOutcome = lastFreeze.Outcome
	}
	if plan == nil {
		obs.Materialization = StepPending
		obs.Approval = StepPending
		obs.Freeze = StepPending
		if cfg.AutomationMode == ModeManual {
			obs.Materialization = StepSkip
			obs.Approval = StepSkip
			obs.Freeze = StepSkip
		}
		return obs
	}
	obs.Materialization, obs.MaterializationReason = stepMaterialization(plan, lastMat)
	obs.Approval, obs.ApprovalReason = stepApproval(plan, lastAppr)
	obs.Freeze, obs.FreezeReason = stepFreeze(plan, lastFreeze)
	return obs
}

func stepMaterialization(plan *models.TradePlan, last *StepResult) (string, string) {
	if plan.IsFrozen() {
		return StepPass, ""
	}
	if isMorningMaterialized(plan) {
		return StepPass, ""
	}
	if last != nil {
		if last.OK {
			return StepPass, ""
		}
		return StepFail, OutcomeLabel(last.Outcome) + ": " + last.Reason
	}
	if Mode() == ModeManual {
		return StepPending, ""
	}
	return StepPending, ReasonMaterializationPending()
}

func stepApproval(plan *models.TradePlan, last *StepResult) (string, string) {
	if plan.IsFrozen() || isApproved(plan) {
		return StepPass, ""
	}
	if last != nil {
		if last.OK {
			return StepPass, ""
		}
		return StepFail, OutcomeLabel(last.Outcome) + ": " + last.Reason
	}
	if Mode() != ModeAuto {
		return StepPending, ""
	}
	return StepPending, ""
}

func stepFreeze(plan *models.TradePlan, last *StepResult) (string, string) {
	if plan.IsFrozen() {
		return StepPass, ""
	}
	if last != nil {
		if last.OK {
			return StepPass, last.Reason
		}
		return StepFail, OutcomeLabel(last.Outcome) + ": " + last.Reason
	}
	if Mode() != ModeAuto {
		return StepPending, ""
	}
	return StepPending, ""
}

func ReasonMaterializationPending() string { return "Morning materialization not completed" }

func defaultRunMaterialize(tradeDate string, now time.Time) (bool, string) {
	res := morningpreparation.RunMorningPlanPreparationJob(morningpreparation.JobOptions{
		TradeDate:          tradeDate,
		Now:                now,
		AttemptMaterialize: true,
	})
	if res.MaterializeAttempted && res.MaterializeSuccess {
		return true, ""
	}
	if !res.MaterializeAttempted {
		if res.Observation.MaterializationStatus == morningpreparation.CheckPass {
			return true, "already_materialized"
		}
		return false, "materialize_not_applicable"
	}
	return false, res.Message
}

func defaultLookupTodayPlan(tradeDate string) (*models.TradePlan, error) {
	return morningpreparation.LookupMorningMaterializePlan(tradeDate)
}

func lookupDraftPlan(tradeDate string) (*models.TradePlan, error) {
	return lookupDraftPlanForAutomation(tradeDate)
}

func isApproved(plan *models.TradePlan) bool {
	return plan != nil && plan.ApprovedAt != nil && !plan.ApprovedAt.IsZero()
}

func isMorningMaterialized(plan *models.TradePlan) bool {
	if plan == nil {
		return false
	}
	if strings.TrimSpace(plan.PricingStage) == "morning_materialized" {
		return true
	}
	for _, it := range plan.Items {
		if it.LimitPrice > 0 && it.TargetVolume > 0 {
			return true
		}
	}
	return false
}

func firstBlockerReason(blockers []readiness.Finding) string {
	for _, b := range blockers {
		msg := strings.TrimSpace(b.Message)
		if msg != "" {
			return msg
		}
		code := strings.TrimSpace(b.Code)
		if code != "" {
			return code
		}
	}
	return ""
}

func isAfterFreezeDeadline(tradeDate string, now time.Time) bool {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(tradeDate), now.Location())
	if err != nil {
		return false
	}
	deadlineStr := GetConfig().FreezeDeadline
	if strings.TrimSpace(deadlineStr) == "" {
		deadlineStr = tradingwindow.OpenFreezeDeadlineHHMM
	}
	t, err := time.ParseInLocation("15:04:05", deadlineStr, now.Location())
	if err != nil {
		t, _ = time.ParseInLocation("15:04:05", tradingwindow.OpenFreezeDeadlineHHMM, now.Location())
	}
	deadline := time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
	return now.After(deadline)
}

func getApproveTestRunner() approveRunnerFn {
	testHookMu.Lock()
	defer testHookMu.Unlock()
	return testApproveRunner
}

func getFreezeTestRunner() freezeRunnerFn {
	testHookMu.Lock()
	defer testHookMu.Unlock()
	return testFreezeRunner
}
