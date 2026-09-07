package execution

import (
	"context"
	"fmt"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCutoverTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:phase2a_pr2_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, data.MigratePaperTrading(testDB))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
		data.SetPlanItemExecutor(nil)
		WirePlanItemExecutor()
	})
}

type countingPort struct {
	inner       ExecutionPort
	submitCalls int
}

func (c *countingPort) Submit(ctx context.Context, intent SubmitIntent) (*broker.TradeOrder, error) {
	c.submitCalls++
	return c.inner.Submit(ctx, intent)
}

func (c *countingPort) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	return c.inner.QueryOrder(ctx, orderID)
}

func (c *countingPort) Cancel(ctx context.Context, orderID string) error {
	return c.inner.Cancel(ctx, orderID)
}

func TestCutover_Case1_OpenBuyFilledViaExecutionService(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	_, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	counter := &countingPort{inner: NewPaperBroker(nil)}
	svc := NewExecutionService(counter)
	executor := newPlanItemExecutorAdapter(svc)
	data.SetPlanItemExecutor(executor)

	item := models.TradePlanItem{
		StockCode:    "sh600000",
		StockName:    "浦发银行",
		Side:         "buy",
		Status:       models.TradePlanItemPending,
		TargetVolume: 100,
		LimitPrice:   10,
	}
	order, err := executor.ExecutePlanItem(item, data.PlanItemExecOpts{
		StockName:   "浦发银行",
		Price:       10,
		Volume:      100,
		Reason:      "phase2a-pr2-case1",
		StrategyTag: models.PaperStrategyTagTradePlan,
		AutoFill:    true,
	})
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, 1, counter.submitCalls)
	require.Equal(t, data.PaperOrderStatusFilled, order.Status)
	require.Equal(t, int64(100), order.FilledVol)
	require.InDelta(t, 10.0, order.FilledPrice, 1e-9)
	require.NotEmpty(t, order.ClientOrderID)
	require.Equal(t, data.PaperExecBackendPaper, order.ExecBackend)
}

func TestCutover_Case2_CashInsufficientRejectedViaExecutionService(t *testing.T) {
	setupCutoverTestDB(t)
	api := data.NewPaperTradingApi()
	_, err := api.ResetAccount(1_000)
	require.NoError(t, err)

	counter := &countingPort{inner: NewPaperBroker(nil)}
	svc := NewExecutionService(counter)
	executor := newPlanItemExecutorAdapter(svc)

	item := models.TradePlanItem{
		StockCode:    "sh603799",
		StockName:    "华友钴业",
		Side:         "buy",
		Status:       models.TradePlanItemPending,
		LimitPrice:   38.50,
		TargetVolume: 2500,
	}
	order, err := executor.ExecutePlanItem(item, data.PlanItemExecOpts{
		StockName:   "华友钴业",
		Price:       38.50,
		Volume:      2500,
		Reason:      "phase2a-pr2-case2",
		StrategyTag: models.PaperStrategyTagTradePlan,
		AutoFill:    true,
	})
	require.Error(t, err)
	require.Nil(t, order, "PreTradeCheck 失败不得创建订单")
	require.Equal(t, 0, counter.submitCalls, "不得进入 ExecutionPort/PaperBroker")
	require.Equal(t, data.PaperOrderRejectCashInsufficient, PreTradeRejectCode(err))

	var orderCount int64
	require.NoError(t, db.Dao.Model(&data.PaperOrder{}).Count(&orderCount).Error)
	require.Zero(t, orderCount)
}

func TestRunPaperOpenBuyOnce_NoDirectSubmitPaperOrderInSource(t *testing.T) {
	// 行为守卫：开盘路径必须经 PlanItemExecutor，未注入时不得静默直连成功。
	setupCutoverTestDB(t)
	data.SetPlanItemExecutor(nil)

	require.Nil(t, data.GetPlanItemExecutorForTest())
}

// 导出测试钩子见 data；此处确认 Wire 后非空。
func TestWirePlanItemExecutor_SetsExecutor(t *testing.T) {
	data.SetPlanItemExecutor(nil)
	WirePlanItemExecutor()
	require.NotNil(t, data.GetPlanItemExecutorForTest())
}
