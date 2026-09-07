package execution

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/broker"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestExecutePlanItem_SafetyGate_FrozenPlanPASS(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "1", Status: "pending"}}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	})
	now := time.Now()
	plan := &models.TradePlan{ID: 9, Status: models.TradePlanStatusReady, FreezeAt: &now, ApprovedAt: &now}
	_, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, Plan: plan, AutoFill: false})
	require.NoError(t, err)
	require.Equal(t, 1, port.calls)
}

func TestExecutePlanItem_SafetyGate_UnfrozenPlanBlocksPort(t *testing.T) {
	port := &stubPort{order: &broker.TradeOrder{ID: "1"}}
	svc := NewExecutionService(port).withSnapshotLoader(func(uint) (*preTradeAccountSnapshot, error) {
		return &preTradeAccountSnapshot{Cash: 1_000_000, Positions: map[string]preTradePosition{}}, nil
	})
	plan := &models.TradePlan{ID: 9, Status: models.TradePlanStatusDraft}
	_, err := svc.ExecutePlanItem(context.Background(), models.TradePlanItem{
		StockCode: "sz000001", Side: "buy", LimitPrice: 10, TargetVolume: 100,
	}, ExecutePlanItemOpts{Price: 10, Volume: 100, Plan: plan})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PLAN_NOT_FROZEN")
	require.Equal(t, 0, port.calls)
}
