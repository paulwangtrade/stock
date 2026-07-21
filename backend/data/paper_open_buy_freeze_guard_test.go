package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func a62SeedReadyPlan(t *testing.T, freeze bool) *models.TradePlan {
	t.Helper()
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())

	tradeDate := todayTradeDateLocal()
	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		Status:         models.TradePlanStatusReady,
		Side:           "buy",
		AmountPerStock: 10_000,
		EnableExecute:  true,
	}
	if freeze {
		now := time.Now()
		plan.FreezeAt = &now
		plan.FreezeBy = "a62-test"
	}
	require.NoError(t, NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: tradeDate,
		StockCode: "sz000001",
		StockName: "平安银行",
		Side:      "buy",
		Status:    models.TradePlanItemPending,
		Priority:  1,
	}}))
	got, err := NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	if freeze {
		require.True(t, got.IsFrozen())
	} else {
		require.False(t, got.IsFrozen())
	}
	return got
}

// G1: Frozen ready → Prepare PASS (MarkChecked).
func TestA62_G1_FrozenReady_PreparePass(t *testing.T) {
	plan := a62SeedReadyPlan(t, true)
	res := RunPaperOpenPrepare()
	require.Equal(t, plan.ID, res.PlanID)
	require.NotContains(t, res.Message, models.ReasonPlanNotFrozen)
	require.Contains(t, res.Message, "prepare: planId=")
	require.NotEmpty(t, res.Codes)

	var checked models.TradePlan
	require.NoError(t, db.Dao.First(&checked, plan.ID).Error)
	require.NotNil(t, checked.CheckedAt)
}

// G2: Frozen ready → Buy PASS (Guard allows; CAS runs — executor may be unset).
func TestA62_G2_FrozenReady_BuyPass(t *testing.T) {
	plan := a62SeedReadyPlan(t, true)
	SetPlanItemExecutor(nil)
	t.Cleanup(func() { SetPlanItemExecutor(nil) })
	SetOpenBuyQuoteFetcherForTest(func(codes ...string) (*[]StockInfo, error) {
		out := []StockInfo{{Code: codes[0], Name: "x", Price: "10.00"}}
		return &out, nil
	})
	t.Cleanup(func() { SetOpenBuyQuoteFetcherForTest(nil) })

	res := RunPaperOpenBuyOnce(false)
	require.Equal(t, plan.ID, res.PlanID)
	require.NotContains(t, res.Message, models.ReasonPlanNotFrozen)
	// CAS must have left ready (executor missing → failed path after Guard).
	var after models.TradePlan
	require.NoError(t, db.Dao.First(&after, plan.ID).Error)
	require.NotEqual(t, models.TradePlanStatusReady, after.Status)
}

// G3: naked ready → Prepare BLOCK PLAN_NOT_FROZEN (no MarkChecked).
func TestA62_G3_NakedReady_PrepareBlock(t *testing.T) {
	plan := a62SeedReadyPlan(t, false)
	res := RunPaperOpenPrepare()
	require.Equal(t, plan.ID, res.PlanID)
	require.Contains(t, res.Message, models.ReasonPlanNotFrozen)
	require.Contains(t, res.Message, "prepare blocked")
	require.Empty(t, res.Codes)

	var checked models.TradePlan
	require.NoError(t, db.Dao.First(&checked, plan.ID).Error)
	require.Nil(t, checked.CheckedAt)
	require.Equal(t, models.TradePlanStatusReady, checked.Status)
}

// G4: naked ready → Buy BLOCK PLAN_NOT_FROZEN (no CAS).
func TestA62_G4_NakedReady_BuyBlock(t *testing.T) {
	plan := a62SeedReadyPlan(t, false)
	res := RunPaperOpenBuyOnce(false)
	require.Equal(t, plan.ID, res.PlanID)
	require.Contains(t, res.Message, models.ReasonPlanNotFrozen)
	require.Contains(t, res.Message, "skip buy")

	var after models.TradePlan
	require.NoError(t, db.Dao.First(&after, plan.ID).Error)
	require.Equal(t, models.TradePlanStatusReady, after.Status)
	require.Nil(t, after.ExecutedAt)
}

func TestA62_SourceBoundary_PrepareBuyWireGuard(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("paper_open_buy.go"))
	require.NoError(t, err)
	src := string(b)
	require.GreaterOrEqual(t, strings.Count(src, "RequireFrozenReadyTradePlan"), 2)
	require.Contains(t, src, "TryBeginExecute")
	require.Contains(t, src, "MarkChecked")
}
