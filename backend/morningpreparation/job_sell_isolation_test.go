package morningpreparation

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMorningIsolationDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, data.EnsureTradePlanTables())
}

func restoreMorningJobFns(t *testing.T) {
	t.Helper()
	origLookup := lookupPlanFn
	origMat := runMaterializeFn
	lookupPlanFn = defaultLookupPlan
	t.Cleanup(func() {
		lookupPlanFn = origLookup
		runMaterializeFn = origMat
	})
}

func seedBuyMorningDraft(t *testing.T, tradeDate string) *models.TradePlan {
	t.Helper()
	return seedBuyMorningDraftID(t, tradeDate, 0)
}

func seedBuyMorningDraftID(t *testing.T, tradeDate string, id uint) *models.TradePlan {
	t.Helper()
	slip := 0.03
	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		Side:                 "buy",
		AmountPerStock:       100_000,
		EnableExecute:        false,
		PlanVersion:          1,
		SourceSession:        models.TradePlanSourceAfterClose,
		DefaultEntryRule:     "LIMIT_REF_PLUS_SLIP",
		DefaultMaxSlippage:   &slip,
		PricingPolicyVersion: 1,
		PricingStage:         "after_close_intent",
	}
	if id != 0 {
		plan.ID = id
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, EntryRule: "LIMIT_REF_PLUS_SLIP", MaxSlippage: &slip,
		IntentStatus: "selected", LimitPrice: 0, TargetVolume: 0,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func seedSellTDraft(t *testing.T, tradeDate string, qty int64) *models.TradePlan {
	t.Helper()
	return seedSellTDraftID(t, tradeDate, 0, qty)
}

func seedSellTDraftID(t *testing.T, tradeDate string, id uint, qty int64) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		Side:                 "sell",
		EnableExecute:        false,
		PlanVersion:          2,
		SourceSession:        models.TradePlanSourceTSell,
		PricingPolicyVersion: 1,
		PricingStage:         tSellDraftPricingStage,
	}
	if id != 0 {
		plan.ID = id
	}
	items := []models.TradePlanItem{{
		TradeDate: tradeDate, StockCode: "sz000001", StockName: "平安银行", Side: "sell",
		Status: models.TradePlanItemPending, TargetAmount: 0,
		IntentStatus: "priced", LimitPrice: 0, TargetVolume: qty,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func injectBuyMaterialize(t *testing.T, captured *uint) {
	t.Helper()
	runMaterializeFn = func(planID uint) (*strategy.MorningIntentMaterializeResult, error) {
		if captured != nil {
			*captured = planID
		}
		return strategy.RunMorningIntentMaterialize(planID, &strategy.MorningIntentMaterializeOpts{
			OpenPriceFn: func(string) (float64, bool) { return 10.2, true },
			VolumeOpts: &strategy.MorningPositionMaterializeOpts{
				Snapshot: &strategy.MorningAccountSnapshot{
					Cash: 1_000_000, Equity: 1_000_000,
					NameMarketValue: map[string]float64{},
					PositionVolumes: map[string]int64{},
				},
				Limits: &strategy.MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.50},
			},
			EvaluateReadiness: func(p *models.TradePlan) readiness.ExecutionIntentReadinessResult {
				return readiness.EvaluateExecutionIntentReadiness(p, &readiness.Options{SkipQualityGate: true})
			},
		})
	}
}

func TestIsMorningBuySide(t *testing.T) {
	require.False(t, isMorningBuySide(nil))
	require.True(t, isMorningBuySide(&models.TradePlan{Side: "buy"}))
	require.True(t, isMorningBuySide(&models.TradePlan{Side: ""}))
	require.False(t, isMorningBuySide(&models.TradePlan{Side: "sell"}))
	require.True(t, isSellIsolatedFromBuyMaterialize(&models.TradePlan{Side: "sell"}))
	require.False(t, shouldAttemptMaterialize(&models.TradePlan{
		Status: models.TradePlanStatusDraft, Side: "sell", PricingStage: tSellDraftPricingStage,
	}))
}

func TestLookupMorningMaterializePlan_CaseA_SellDoesNotCoverBuy(t *testing.T) {
	setupMorningIsolationDB(t)
	restoreMorningJobFns(t)

	const td = "2026-08-18"
	buy := seedBuyMorningDraftID(t, td, 100)
	sell := seedSellTDraftID(t, td, 101, 500)
	require.Equal(t, uint(100), buy.ID)
	require.Equal(t, uint(101), sell.ID)

	latest, err := data.NewTradePlanRepo().GetLatestByTradeDate(td)
	require.NoError(t, err)
	require.Equal(t, uint(101), latest.ID, "shared GetLatestByTradeDate must still return sell id=101")

	looked, err := LookupMorningMaterializePlan(td)
	require.NoError(t, err)
	require.NotNil(t, looked)
	require.Equal(t, uint(100), looked.ID)
	require.Equal(t, "buy", looked.Side)
	require.NotEqual(t, uint(101), looked.ID)
}

func TestLookupMorningMaterializePlan_CaseB_OnlySellReturnsNil(t *testing.T) {
	setupMorningIsolationDB(t)
	restoreMorningJobFns(t)

	const td = "2026-08-18"
	sell := seedSellTDraftID(t, td, 101, 500)
	require.Equal(t, uint(101), sell.ID)

	looked, err := LookupMorningMaterializePlan(td)
	require.Nil(t, looked)
	require.Error(t, err)
	require.False(t, isMorningBuySide(sell))
}

func TestLookupMorningMaterializePlan_CaseC_OnlyBuyUnchanged(t *testing.T) {
	setupMorningIsolationDB(t)
	restoreMorningJobFns(t)

	const td = "2026-08-18"
	buy := seedBuyMorningDraftID(t, td, 100)
	require.Equal(t, uint(100), buy.ID)

	looked, err := LookupMorningMaterializePlan(td)
	require.NoError(t, err)
	require.NotNil(t, looked)
	require.Equal(t, uint(100), looked.ID)
	require.Equal(t, "buy", looked.Side)
}

func TestRunMorningPlanPreparationJob_MixedBuySellSameDay_IsolatesSell(t *testing.T) {
	setupMorningIsolationDB(t)
	restoreMorningJobFns(t)

	const td = "2026-08-18"
	buy := seedBuyMorningDraft(t, td)
	sell := seedSellTDraft(t, td, 500)
	require.Greater(t, sell.ID, buy.ID)

	latest, err := data.NewTradePlanRepo().GetLatestByTradeDate(td)
	require.NoError(t, err)
	require.Equal(t, sell.ID, latest.ID, "shared GetLatestByTradeDate must still return newest sell")

	looked, err := LookupMorningMaterializePlan(td)
	require.NoError(t, err)
	require.Equal(t, buy.ID, looked.ID)

	var captured uint
	injectBuyMaterialize(t, &captured)

	res := RunMorningPlanPreparationJob(JobOptions{
		TradeDate:          td,
		Now:                time.Date(2026, 8, 18, 9, 26, 0, 0, time.Local),
		AttemptMaterialize: true,
	})
	require.Equal(t, buy.ID, captured)
	require.True(t, res.MaterializeAttempted)
	require.True(t, res.MaterializeSuccess)
	require.Equal(t, buy.ID, res.Observation.PlanID)

	gotBuy, err := data.NewTradePlanRepo().GetByID(buy.ID)
	require.NoError(t, err)
	require.Equal(t, "morning_materialized", gotBuy.PricingStage)
	require.Greater(t, gotBuy.Items[0].LimitPrice, 0.0)
	require.GreaterOrEqual(t, gotBuy.Items[0].TargetVolume, int64(100))

	gotSell, err := data.NewTradePlanRepo().GetByID(sell.ID)
	require.NoError(t, err)
	require.Equal(t, "sell", gotSell.Side)
	require.Equal(t, tSellDraftPricingStage, gotSell.PricingStage)
	require.Equal(t, int64(500), gotSell.Items[0].TargetVolume)
	require.Equal(t, 0.0, gotSell.Items[0].LimitPrice)
}

func TestRunMorningPlanPreparationJob_OnlySellPlan_DoesNotMaterialize(t *testing.T) {
	setupMorningIsolationDB(t)
	restoreMorningJobFns(t)

	const td = "2026-08-18"
	sell := seedSellTDraft(t, td, 500)

	called := false
	runMaterializeFn = func(planID uint) (*strategy.MorningIntentMaterializeResult, error) {
		called = true
		return &strategy.MorningIntentMaterializeResult{PlanID: planID}, nil
	}

	res := RunMorningPlanPreparationJob(JobOptions{
		TradeDate:          td,
		Now:                time.Date(2026, 8, 18, 9, 26, 0, 0, time.Local),
		AttemptMaterialize: true,
	})
	require.False(t, called)
	require.False(t, res.MaterializeAttempted)
	require.False(t, res.MaterializeSuccess)
	require.Zero(t, res.Observation.PlanID)

	gotSell, err := data.NewTradePlanRepo().GetByID(sell.ID)
	require.NoError(t, err)
	require.Equal(t, int64(500), gotSell.Items[0].TargetVolume)
	require.Equal(t, tSellDraftPricingStage, gotSell.PricingStage)
}

func TestRunMorningPlanPreparationJob_BuyOnlyRegression(t *testing.T) {
	setupMorningIsolationDB(t)
	restoreMorningJobFns(t)

	const td = "2026-08-18"
	buy := seedBuyMorningDraft(t, td)

	var captured uint
	injectBuyMaterialize(t, &captured)

	res := RunMorningPlanPreparationJob(JobOptions{
		TradeDate:          td,
		Now:                time.Date(2026, 8, 18, 9, 26, 0, 0, time.Local),
		AttemptMaterialize: true,
	})
	require.Equal(t, buy.ID, captured)
	require.True(t, res.MaterializeAttempted)
	require.True(t, res.MaterializeSuccess)

	gotBuy, err := data.NewTradePlanRepo().GetByID(buy.ID)
	require.NoError(t, err)
	require.Equal(t, "morning_materialized", gotBuy.PricingStage)
	require.Greater(t, gotBuy.Items[0].LimitPrice, 0.0)
	require.GreaterOrEqual(t, gotBuy.Items[0].TargetVolume, int64(100))
}
