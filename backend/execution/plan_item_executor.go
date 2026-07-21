package execution

import (
	"context"
	"fmt"
	"strconv"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
)

// planItemExecutorAdapter 将 data.PlanItemExecutor 接到 ExecutionService → PaperBroker。
type planItemExecutorAdapter struct {
	svc *ExecutionService
}

func newPlanItemExecutorAdapter(svc *ExecutionService) data.PlanItemExecutor {
	if svc == nil {
		svc = NewExecutionService(NewPaperBroker(nil))
	}
	return &planItemExecutorAdapter{svc: svc}
}

// WirePlanItemExecutor 将开盘路径接到 ExecutionService / PaperBroker（可重复调用）。
func WirePlanItemExecutor() {
	data.SetPlanItemExecutor(newPlanItemExecutorAdapter(nil))
}

func init() {
	WirePlanItemExecutor()
}

func (a *planItemExecutorAdapter) ExecutePlanItem(item models.TradePlanItem, opts data.PlanItemExecOpts) (*data.PaperOrder, error) {
	tradeOrder, err := a.svc.ExecutePlanItem(context.Background(), item, ExecutePlanItemOpts{
		AccountID:   opts.AccountID,
		StockName:   opts.StockName,
		Price:       opts.Price,
		Volume:      opts.Volume,
		Reason:      opts.Reason,
		StrategyTag: opts.StrategyTag,
		AutoFill:    opts.AutoFill,
	})
	if tradeOrder == nil {
		return nil, err
	}
	id, parseErr := strconv.ParseUint(tradeOrder.ID, 10, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("execution: invalid order id %q: %w", tradeOrder.ID, parseErr)
	}
	var order data.PaperOrder
	if dbErr := db.Dao.First(&order, uint(id)).Error; dbErr != nil {
		if err != nil {
			return nil, fmt.Errorf("%w (reload order: %v)", err, dbErr)
		}
		return nil, dbErr
	}
	return &order, err
}
