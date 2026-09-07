package tradingautomation

import (
	"sync"
	"time"

	"go-stock/backend/models"
)

var testHookMu sync.Mutex

type approveRunnerFn func(planID uint, now time.Time) (bool, string)
type freezeRunnerFn func(planID uint, now time.Time) (bool, string)

var (
	testApproveRunner approveRunnerFn
	testFreezeRunner  freezeRunnerFn
)

// SetMaterializeRunnerForTest overrides materialize step (tests).
func SetMaterializeRunnerForTest(fn materializeRunner) func() {
	testHookMu.Lock()
	prev := runMaterializeStepFn
	runMaterializeStepFn = fn
	testHookMu.Unlock()
	return func() {
		testHookMu.Lock()
		runMaterializeStepFn = prev
		testHookMu.Unlock()
	}
}

// SetLookupPlanForTest overrides plan lookup (tests).
func SetLookupPlanForTest(fn planLookupFn) {
	testHookMu.Lock()
	lookupTodayPlanFn = fn
	testHookMu.Unlock()
}

// ResetLookupPlanForTest restores default plan lookup.
func ResetLookupPlanForTest() {
	testHookMu.Lock()
	lookupTodayPlanFn = defaultLookupTodayPlan
	testHookMu.Unlock()
}

// SetApproveRunnerForTest bypasses approvegate for unit tests.
func SetApproveRunnerForTest(fn approveRunnerFn) func() {
	testHookMu.Lock()
	testApproveRunner = fn
	testHookMu.Unlock()
	return func() {
		testHookMu.Lock()
		testApproveRunner = nil
		testHookMu.Unlock()
	}
}

// SetFreezeRunnerForTest bypasses strategy.FreezeTradePlan for unit tests.
func SetFreezeRunnerForTest(fn freezeRunnerFn) func() {
	testHookMu.Lock()
	testFreezeRunner = fn
	testHookMu.Unlock()
	return func() {
		testHookMu.Lock()
		testFreezeRunner = nil
		testHookMu.Unlock()
	}
}

func lookupDraftPlanForAutomation(tradeDate string) (*models.TradePlan, error) {
	testHookMu.Lock()
	fn := lookupTodayPlanFn
	testHookMu.Unlock()
	plan, err := fn(tradeDate)
	if err != nil || plan == nil {
		return plan, err
	}
	if plan.IsFrozen() || plan.Status == models.TradePlanStatusDraft {
		return plan, nil
	}
	return nil, nil
}
