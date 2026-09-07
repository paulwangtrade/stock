package data

import (
	"sync"

	"go-stock/backend/models"
)

// PlanItemExecOpts 开盘编排传入的运行时参数（价量/名称等）。
type PlanItemExecOpts struct {
	AccountID   uint
	StockName   string
	Price       float64
	Volume      int64
	Reason      string
	StrategyTag string
	AutoFill    bool
	// Plan enables Safety Gate freeze check (Phase6.5.7.4.1).
	Plan *models.TradePlan
}

// PlanItemExecutor 开盘买入唯一下单入口（由 execution 注入 ExecutionService 实现）。
type PlanItemExecutor interface {
	ExecutePlanItem(item models.TradePlanItem, opts PlanItemExecOpts) (*PaperOrder, error)
}

var (
	planItemExecutorMu sync.RWMutex
	planItemExecutor   PlanItemExecutor
)

// SetPlanItemExecutor 注入执行端口实现；传 nil 清除（测试用）。
func SetPlanItemExecutor(e PlanItemExecutor) {
	planItemExecutorMu.Lock()
	planItemExecutor = e
	planItemExecutorMu.Unlock()
}

func getPlanItemExecutor() PlanItemExecutor {
	planItemExecutorMu.RLock()
	defer planItemExecutorMu.RUnlock()
	return planItemExecutor
}

// GetPlanItemExecutorForTest 仅供测试断言注入状态。
func GetPlanItemExecutorForTest() PlanItemExecutor {
	return getPlanItemExecutor()
}
