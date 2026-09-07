package job

// Well-known trading job names (match existing App cron keys where possible).
const (
	JobDailyPlan          = "paper_daily_plan"            // 09:20 morning preparation
	JobOpenPrepare        = "paper_open_prepare"          // 09:25
	JobOpenBuy            = "paper_open_buy"              // 09:30
	JobAfterCloseWorkflow = "after_close_plan_workflow"   // 15:30
	JobPaperTradingOpen     = "paper_trading_open"        // 09:31 fill A
	JobPaperTradingSettle   = "paper_trading_settle"      // 15:05
	JobPaperTradingSessionB = "paper_trading_session_b"   // 15:10 fill B
	JobPaperTradingT1Unlock = "paper_trading_t1_unlock"   // 09:20 T+1 unlock (Phase11-K)
	JobMorningPrepDeadline  = "morning_plan_prep_deadline"
	JobMorningPrepMaterialize = "morning_plan_prep_materialize"
	JobTradingAutomationApprove = "trading_automation_approve"
	JobTradingAutomationFreeze  = "trading_automation_freeze"
)

// TradingCatalog returns the Phase11-A monitored job definitions (detect-only catch-up).
func TradingCatalog() []Definition {
	return []Definition{
		{
			Name: JobDailyPlan, ExpectedSchedule: "0 20 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:20 morning plan preparation (RunMorningPlanPreparation)",
		},
		{
			Name: JobOpenPrepare, ExpectedSchedule: "0 25 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:25 paper open prepare",
		},
		{
			Name: JobOpenBuy, ExpectedSchedule: "0 30 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:30 paper open buy cron",
		},
		{
			Name: JobAfterCloseWorkflow, ExpectedSchedule: "0 30 15 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "15:30 after-close Candidate→Draft→Risk",
		},
		{
			Name: JobPaperTradingOpen, ExpectedSchedule: "0 31 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:31 paper trading fill mode A",
		},
		{
			Name: JobMorningPrepDeadline, ExpectedSchedule: "0 25 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:25 morning readiness deadline checkpoint",
		},
		{
			Name: JobMorningPrepMaterialize, ExpectedSchedule: "0 26 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:26 morning plan materialize attempt",
		},
		{
			Name: JobTradingAutomationApprove, ExpectedSchedule: "0 25 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:25 trading automation auto approve (AUTO mode)",
		},
		{
			Name: JobTradingAutomationFreeze, ExpectedSchedule: "0 29 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:29 trading automation auto freeze (AUTO mode)",
		},
		{
			Name: JobPaperTradingSettle, ExpectedSchedule: "0 5 15 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "15:05 paper settlement",
		},
		{
			Name: JobPaperTradingSessionB, ExpectedSchedule: "0 10 15 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "15:10 paper trading fill mode B",
		},
		{
			Name: JobPaperTradingT1Unlock, ExpectedSchedule: "0 20 9 * * 1-5",
			CatchUpPolicy: CatchUpDetectOnly,
			Description:   "09:20 T+1 unlock (PositionUnlockJob via morning settlement)",
		},
	}
}
