package data

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"gorm.io/gorm"
)

// AfterClose / Morning / ExecutionReady / Schema status strings for TradingDayStatusView.
const (
	AfterCloseStatusCompleted       = "COMPLETED"
	AfterCloseStatusRiskFailed      = "RISK_FAILED"
	AfterCloseStatusSkippedDisabled = "SKIPPED_DISABLED"
	AfterCloseStatusNotFound        = "NOT_FOUND"
	AfterCloseStatusUnknown         = "UNKNOWN"

	MorningModeAdoptFrozen  = "adopt_frozen"
	MorningModeBuildMorning = "build_morning"

	ExecutionGuardWouldPass           = "WOULD_PASS"
	ExecutionGuardWouldBlockNotFrozen = "WOULD_BLOCK_NOT_FROZEN"
	ExecutionGuardNA                  = "N/A"

	RiskStatusNA = "N/A"
)

// TradingDayStatusView is the Phase6.5.4.1 Production Readiness readonly snapshot.
type TradingDayStatusView struct {
	Date           string                       `json:"date"`
	AfterClose     TradingDayAfterCloseView     `json:"after_close"`
	Plan           TradingDayPlanView           `json:"plan"`
	Risk           TradingDayRiskView           `json:"risk"`
	Freeze         TradingDayFreezeView         `json:"freeze"`
	Morning        TradingDayMorningView        `json:"morning"`
	ExecutionReady TradingDayExecutionReadyView `json:"execution_ready"`
	Schema         TradingDaySchemaView         `json:"schema"`
	Cron           TradingDayCronView           `json:"cron"`
}

// TradingDayAfterCloseView inferred after-close outcome from persisted Pool/Plan (no workflow run).
type TradingDayAfterCloseView struct {
	Status  string `json:"status"`
	LastRun string `json:"last_run,omitempty"`
	PoolID  uint   `json:"pool_id,omitempty"`
	PlanID  uint   `json:"plan_id,omitempty"`
}

// TradingDayPlanView plan existence / identity (persisted fields only).
type TradingDayPlanView struct {
	Exists        bool   `json:"exists"`
	Status        string `json:"status,omitempty"`
	PlanVersion   int    `json:"plan_version,omitempty"`
	SourceSession string `json:"source_session,omitempty"`
}

// TradingDayRiskView persisted Risk fields only (never recompute PlanFilter).
type TradingDayRiskView struct {
	Passed bool   `json:"passed"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// TradingDayFreezeView Freeze audit snapshot.
type TradingDayFreezeView struct {
	IsFrozen bool   `json:"is_frozen"`
	FreezeAt string `json:"freeze_at,omitempty"`
	FreezeBy string `json:"freeze_by,omitempty"`
}

// TradingDayMorningView expected morning mode from Frozen presence (readonly expectation).
type TradingDayMorningView struct {
	Mode string `json:"mode"` // adopt_frozen | build_morning
}

// TradingDayExecutionReadyView aligns with A6.2 Guard: ready AND freeze_at != nil.
// Must NOT reuse legacy PaperOpenBuyStatus.executionReady.
type TradingDayExecutionReadyView struct {
	Ready       bool   `json:"ready"`
	GuardStatus string `json:"guard_status"`
	Reason      string `json:"reason,omitempty"`
}

// TradingDaySchemaView schema registry observability.
type TradingDaySchemaView struct {
	RegistryVersion  int    `json:"registry_version"`
	ValidationStatus string `json:"validation_status"`
}

// TradingDayCronView in-process cron registration flags (readonly).
type TradingDayCronView struct {
	AfterCloseRegistered bool `json:"after_close_registered"`
	MorningRegistered    bool `json:"morning_registered"`
}

// SchemaStatusProvider supplies schema snapshot for TradingDayStatus (injectable for tests).
type SchemaStatusProvider func() TradingDaySchemaView

var tradingDaySchemaProvider SchemaStatusProvider = defaultTradingDaySchemaStatus

// SetTradingDaySchemaStatusProvider wires App/startup validation into the assembler.
// Passing nil restores the default lightweight probe.
func SetTradingDaySchemaStatusProvider(p SchemaStatusProvider) {
	if p == nil {
		tradingDaySchemaProvider = defaultTradingDaySchemaStatus
		return
	}
	tradingDaySchemaProvider = p
}

func defaultTradingDaySchemaStatus() TradingDaySchemaView {
	version := db.GetAppliedSchemaVersion()
	view := TradingDaySchemaView{
		RegistryVersion:  version,
		ValidationStatus: db.SchemaValidationBlocked,
	}
	if db.Dao == nil {
		return view
	}
	m := db.Dao.Migrator()
	if !m.HasTable(&models.TradePlan{}) {
		return view
	}
	// Lifecycle freeze_at is required for Guard-aligned readiness observability (registry v3).
	if !m.HasColumn(&models.TradePlan{}, "FreezeAt") {
		return view
	}
	view.ValidationStatus = db.SchemaValidationReady
	return view
}

type tradingDayStatusQuerier interface {
	GetFrozenByTradeDate(tradeDate string) (*models.TradePlan, error)
	GetLatestByTradeDate(tradeDate string) (*models.TradePlan, error)
}

type tradingDayPoolQuerier interface {
	GetLatestByTradeDate(tradeDate string) (*models.CandidatePool, error)
}

// BuildTradingDayStatus assembles a readonly TradingDayStatusView for tradeDate (YYYY-MM-DD).
//
// Data sources: TradePlan, CandidatePool, persisted Risk fields, schema provider, cron registry.
// Forbidden: Risk recompute, Workflow trigger, Execution calls.
func BuildTradingDayStatus(tradeDate string) (*TradingDayStatusView, error) {
	return buildTradingDayStatus(tradeDate, NewTradePlanRepo(), NewCandidatePoolRepo())
}

func buildTradingDayStatus(tradeDate string, plans tradingDayStatusQuerier, pools tradingDayPoolQuerier) (*TradingDayStatusView, error) {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return nil, fmt.Errorf("trade_date is required")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return nil, fmt.Errorf("invalid trade_date: %w", err)
	}
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	plan, err := loadTradingDayPlan(plans, tradeDate)
	if err != nil {
		return nil, err
	}

	var pool *models.CandidatePool
	if pools != nil {
		p, perr := pools.GetLatestByTradeDate(tradeDate)
		if perr != nil && !errorsIsRecordNotFound(perr) {
			return nil, perr
		}
		pool = p
	}

	view := &TradingDayStatusView{
		Date:   tradeDate,
		Schema: tradingDaySchemaProvider(),
		Cron:   GetTradingDayCronStatus(),
	}
	fillTradingDayPlanBlocks(view, plan, pool)
	return view, nil
}

func loadTradingDayPlan(plans tradingDayStatusQuerier, tradeDate string) (*models.TradePlan, error) {
	if plans == nil {
		return nil, nil
	}
	frozen, err := plans.GetFrozenByTradeDate(tradeDate)
	if err == nil {
		return frozen, nil
	}
	if !errorsIsRecordNotFound(err) {
		return nil, err
	}
	latest, err := plans.GetLatestByTradeDate(tradeDate)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return latest, nil
}

func fillTradingDayPlanBlocks(view *TradingDayStatusView, plan *models.TradePlan, pool *models.CandidatePool) {
	view.AfterClose = deriveAfterCloseView(plan, pool)
	view.Plan = derivePlanView(plan)
	view.Risk = deriveRiskView(plan)
	view.Freeze = deriveFreezeView(plan)
	view.Morning = deriveMorningView(plan)
	view.ExecutionReady = deriveExecutionReadyView(plan)
}

func deriveAfterCloseView(plan *models.TradePlan, pool *models.CandidatePool) TradingDayAfterCloseView {
	out := TradingDayAfterCloseView{Status: AfterCloseStatusUnknown}
	if plan != nil {
		out.PlanID = plan.ID
		if plan.PoolID != 0 {
			out.PoolID = plan.PoolID
		}
		if !plan.GeneratedAt.IsZero() {
			out.LastRun = plan.GeneratedAt.Format(time.RFC3339)
		} else if plan.CheckedAt != nil && !plan.CheckedAt.IsZero() {
			out.LastRun = plan.CheckedAt.Format(time.RFC3339)
		}
	}
	if pool != nil {
		if out.PoolID == 0 {
			out.PoolID = pool.ID
		}
		if out.LastRun == "" && !pool.GeneratedAt.IsZero() {
			out.LastRun = pool.GeneratedAt.Format(time.RFC3339)
		}
	}

	switch {
	case plan == nil && pool == nil:
		if !IsAfterClosePlanEnabled() {
			out.Status = AfterCloseStatusSkippedDisabled
		} else {
			out.Status = AfterCloseStatusNotFound
		}
	case plan != nil && strings.TrimSpace(plan.RiskStatus) == risk.PlanRiskStatusBlocked:
		out.Status = AfterCloseStatusRiskFailed
	case plan != nil || pool != nil:
		out.Status = AfterCloseStatusCompleted
	default:
		out.Status = AfterCloseStatusUnknown
	}
	return out
}

func derivePlanView(plan *models.TradePlan) TradingDayPlanView {
	if plan == nil {
		return TradingDayPlanView{Exists: false}
	}
	return TradingDayPlanView{
		Exists:        true,
		Status:        plan.Status,
		PlanVersion:   plan.PlanVersion,
		SourceSession: plan.SourceSession,
	}
}

func deriveRiskView(plan *models.TradePlan) TradingDayRiskView {
	if plan == nil {
		return TradingDayRiskView{Passed: false, Status: RiskStatusNA}
	}
	status := strings.TrimSpace(plan.RiskStatus)
	if status == "" {
		status = RiskStatusNA
	}
	passed := visibilityRiskPassed(status)
	reason := ""
	reasons := visibilityRiskReasons(plan)
	if len(reasons) > 0 {
		reason = reasons[0]
	} else if s := strings.TrimSpace(plan.RiskSummary); s != "" {
		reason = s
	}
	return TradingDayRiskView{Passed: passed, Status: status, Reason: reason}
}

func deriveFreezeView(plan *models.TradePlan) TradingDayFreezeView {
	if plan == nil {
		return TradingDayFreezeView{IsFrozen: false}
	}
	return TradingDayFreezeView{
		IsFrozen: plan.IsFrozen(),
		FreezeAt: formatTradingDayTimePtr(plan.FreezeAt),
		FreezeBy: plan.FreezeBy,
	}
}

func deriveMorningView(plan *models.TradePlan) TradingDayMorningView {
	if plan != nil && plan.IsFrozen() {
		return TradingDayMorningView{Mode: MorningModeAdoptFrozen}
	}
	return TradingDayMorningView{Mode: MorningModeBuildMorning}
}

// deriveExecutionReadyView mirrors models.RequireFrozenReadyTradePlan (A6.2 Guard).
// Condition: status=ready AND freeze_at != nil. Never uses legacy executionReady.
func deriveExecutionReadyView(plan *models.TradePlan) TradingDayExecutionReadyView {
	guard := models.RequireFrozenReadyTradePlan(plan)
	if guard.Allowed {
		return TradingDayExecutionReadyView{
			Ready:       true,
			GuardStatus: ExecutionGuardWouldPass,
		}
	}
	status := ExecutionGuardWouldBlockNotFrozen
	if plan == nil {
		status = ExecutionGuardNA
	}
	return TradingDayExecutionReadyView{
		Ready:       false,
		GuardStatus: status,
		Reason:      guard.Reason,
	}
}

func formatTradingDayTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
