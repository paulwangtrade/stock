// TradePlan Lifecycle View — read-only display projection (Phase10-F.1).
package papertrading

import (
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Lifecycle display statuses (UI only; not persisted on trade_plans.status).
const (
	LifecycleDisplayDraft     = "DRAFT"
	LifecycleDisplayApproved  = "APPROVED"
	LifecycleDisplayFrozen    = "FROZEN"
	LifecycleDisplayExecuting = "EXECUTING"
	LifecycleDisplayCompleted = "COMPLETED"
)

// TradePlanLifecycleView is the read-only timeline for one plan.
type TradePlanLifecycleView struct {
	PlanID               uint       `json:"plan_id"`
	TradeDate            string     `json:"trade_date"`
	DBStatus             string     `json:"db_status"`
	DisplayStatus        string     `json:"display_status"`
	PlanCreatedAt        *time.Time `json:"plan_created_at"`
	ApprovedAt           *time.Time `json:"approved_at"`
	FreezeAt             *time.Time `json:"freeze_at"`
	ExecutionStartedAt   *time.Time `json:"execution_started_at"`
	FirstFillAt          *time.Time `json:"first_fill_at"`
	DataSourceNote       string     `json:"data_source_note"`
}

// BuildTradePlanLifecycle loads plan + earliest run/fill timestamps (read-only).
func BuildTradePlanLifecycle(planID uint) (*TradePlanLifecycleView, error) {
	out := &TradePlanLifecycleView{
		PlanID:         planID,
		DisplayStatus:  LifecycleDisplayDraft,
		DataSourceNote: "TradePlan Lifecycle · read-only display; does not mutate plan status machine",
	}
	if planID == 0 {
		return out, nil
	}
	if db.Dao == nil {
		return out, nil
	}
	var plan models.TradePlan
	if err := db.Dao.First(&plan, planID).Error; err != nil {
		return nil, err
	}
	out.TradeDate = plan.TradeDate
	out.DBStatus = plan.Status
	out.ApprovedAt = cloneTimePtr(plan.ApprovedAt)
	out.FreezeAt = cloneTimePtr(plan.FreezeAt)

	created := plan.CreatedAt
	if created.IsZero() {
		created = plan.GeneratedAt
	}
	if !created.IsZero() {
		c := created
		out.PlanCreatedAt = &c
	}

	// Earliest run start / ExecutedAt
	var run PaperSimRun
	err := db.Dao.Where("plan_id = ?", planID).Order("started_at asc, id asc").First(&run).Error
	if err == nil && !run.StartedAt.IsZero() {
		t := run.StartedAt
		out.ExecutionStartedAt = &t
	} else if plan.ExecutedAt != nil && !plan.ExecutedAt.IsZero() {
		out.ExecutionStartedAt = cloneTimePtr(plan.ExecutedAt)
	}

	var fill PaperSimFill
	err = db.Dao.Where("plan_id = ?", planID).Order("filled_at asc, id asc").First(&fill).Error
	if err == nil && !fill.FilledAt.IsZero() {
		t := fill.FilledAt
		out.FirstFillAt = &t
	}

	runStatus := ""
	if run.ID > 0 {
		runStatus = run.Status
	}
	out.DisplayStatus = DeriveLifecycleDisplayStatus(plan, runStatus)
	return out, nil
}

// DeriveLifecycleDisplayStatus maps DB plan + run status → display label.
func DeriveLifecycleDisplayStatus(plan models.TradePlan, runStatus string) string {
	st := strings.TrimSpace(plan.Status)
	switch st {
	case models.TradePlanStatusDone, models.TradePlanStatusPartial, models.TradePlanStatusSkipped,
		models.TradePlanStatusFailed, models.TradePlanStatusSuperseded:
		return LifecycleDisplayCompleted
	case models.TradePlanStatusExecuting:
		return LifecycleDisplayExecuting
	}
	if strings.EqualFold(strings.TrimSpace(runStatus), RunStatusRunning) {
		return LifecycleDisplayExecuting
	}
	if plan.IsFrozen() {
		return LifecycleDisplayFrozen
	}
	if plan.ApprovedAt != nil && !plan.ApprovedAt.IsZero() {
		return LifecycleDisplayApproved
	}
	return LifecycleDisplayDraft
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	c := *t
	return &c
}
