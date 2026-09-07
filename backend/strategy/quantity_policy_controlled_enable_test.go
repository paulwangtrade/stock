package strategy_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingrule"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Phase12-M2.4: controlled enable path
// Materialize (Policy) → target_volume → Freeze → Broker Validate-only → Fill

func setupControlledEnableDB(t *testing.T) {
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
		_ = sqlDB.Close()
	})
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureTradePlanTables())
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 2_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
}

func seedPricedDraft(t *testing.T, tradeDate string, items []models.TradePlanItem) *models.TradePlan {
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
		DefaultEntryRule:     "limit_ref_plus_slip",
		DefaultMaxSlippage:   &slip,
		PricingPolicyVersion: 1,
		PricingStage:         "morning_materialized",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func freezeDraftForBroker(t *testing.T, planID uint) {
	t.Helper()
	now := time.Now()
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Where("id = ?", planID).Updates(map[string]any{
		"status":      models.TradePlanStatusReady,
		"approved_at": now,
		"approved_by": "m2.4-test",
		"freeze_at":   now,
		"freeze_by":   "m2.4-test",
	}).Error)
}

func TestControlledEnable_FullPath_MAIN_ETF_STAR_CB(t *testing.T) {
	setupControlledEnableDB(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.True(t, tradingrule.EnableQuantityPolicy())

	cases := []struct {
		name       string
		code       string
		amount     float64
		price      float64
		wantVol    int64
		wantFill   bool
		brokerOpen float64
	}{
		// price=1 → raw=amount; Policy sizes target_volume
		{"MAIN", "sz000001", 101, 1.0, 100, true, 10.0},
		{"ETF", "sh510300", 100, 1.0, 100, true, 4.0},
		{"STAR", "sh688981", 201, 1.0, 201, true, 50.0},
		{"CB", "sh113052", 10, 1.0, 10, true, 120.0},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			date := fmt.Sprintf("2026-09-%02d", i+1)
			plan := seedPricedDraft(t, date, []models.TradePlanItem{{
				TradeDate: date, StockCode: tc.code, Side: "buy",
				Status: models.TradePlanItemPending, TargetAmount: tc.amount,
				LimitPrice: tc.price, TargetVolume: 0, IntentStatus: "priced",
			}})

			mat, err := strategy.MaterializeMorningTargetVolumes(plan.ID, &strategy.MorningPositionMaterializeOpts{
				Snapshot: &strategy.MorningAccountSnapshot{
					Cash: 1_000_000, Equity: 1_000_000,
					NameMarketValue: map[string]float64{},
					PositionVolumes: map[string]int64{},
				},
				Limits: &strategy.MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.50},
			})
			require.NoError(t, err)
			require.Equal(t, 1, mat.SizedCount, "materialize sized")
			require.NotEmpty(t, mat.QuantityShadows, "shadow retained")

			got, err := data.NewTradePlanRepo().GetByID(plan.ID)
			require.NoError(t, err)
			require.Equal(t, tc.wantVol, got.Items[0].TargetVolume, "Policy target_volume")

			// Shadow: policy path may differ from old; recorded but does not rewrite volume
			sh := mat.QuantityShadows[0]
			require.Equal(t, tc.code, sh.Instrument)
			require.Equal(t, tc.wantVol, sh.PolicyQuantity)

			freezeDraftForBroker(t, plan.ID)

			broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
				Quotes: map[string]papertrading.Quote{
					tc.code: {Open: tc.brokerOpen, LimitUp: tc.brokerOpen * 1.2},
				},
			})
			res, err := broker.RunForPlan(plan.ID)
			require.NoError(t, err)
			if tc.wantFill {
				require.Equal(t, 1, res.FilledCount)
				status, err := papertrading.GetPlanPaperStatus(plan.ID)
				require.NoError(t, err)
				require.Equal(t, papertrading.OrderStatusFilled, status.Orders[0].Status)
				require.Equal(t, tc.wantVol, status.Orders[0].Quantity)
				require.Equal(t, tc.wantVol, status.Orders[0].FilledVolume)
			}
		})
	}
}

func TestControlledEnable_BrokerRejectsDirtySTAR199_AfterMaterializeSkip(t *testing.T) {
	// If someone freezes a dirty STAR 199 Spec while Flag ON, Broker Validate-only rejects.
	setupControlledEnableDB(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	date := "2026-09-10"
	plan := seedPricedDraft(t, date, []models.TradePlanItem{{
		TradeDate: date, StockCode: "sh688981", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 199,
		LimitPrice: 1.0, TargetVolume: 0, IntentStatus: "priced",
	}})
	mat, err := strategy.MaterializeMorningTargetVolumes(plan.ID, &strategy.MorningPositionMaterializeOpts{
		Snapshot: &strategy.MorningAccountSnapshot{
			Cash: 1_000_000, Equity: 1_000_000,
			NameMarketValue: map[string]float64{}, PositionVolumes: map[string]int64{},
		},
		Limits: &strategy.MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.50},
	})
	require.NoError(t, err)
	require.Equal(t, 1, mat.SizeSkipCount) // Policy cannot size 199

	// Force dirty Spec as if DB were hand-edited
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Where("plan_id = ?", plan.ID).Updates(map[string]any{
		"target_volume": 199,
		"intent_status": "priced",
	}).Error)
	freezeDraftForBroker(t, plan.ID)

	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sh688981": {Open: 50, LimitUp: 60}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)
	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
}

func TestControlledEnable_FlagOff_FallbackLegacyLot(t *testing.T) {
	setupControlledEnableDB(t)
	tradingrule.SetEnableQuantityPolicyForTest(false)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	date := "2026-09-11"
	plan := seedPricedDraft(t, date, []models.TradePlanItem{{
		TradeDate: date, StockCode: "sh688981", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 201,
		LimitPrice: 1.0, TargetVolume: 0, IntentStatus: "priced",
	}})
	_, err := strategy.MaterializeMorningTargetVolumes(plan.ID, &strategy.MorningPositionMaterializeOpts{
		Snapshot: &strategy.MorningAccountSnapshot{
			Cash: 1_000_000, Equity: 1_000_000,
			NameMarketValue: map[string]float64{}, PositionVolumes: map[string]int64{},
		},
		Limits: &strategy.MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.50},
	})
	require.NoError(t, err)
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	// Legacy /100: floor(201/100)*100 = 200 — not Policy 201
	require.Equal(t, int64(200), got.Items[0].TargetVolume)
}

func TestControlledEnable_LegacyCalcMorningLotVolumeRetained(t *testing.T) {
	src, err := os.ReadFile("morning_position_materialize.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "func calcMorningLotVolume")
	require.Contains(t, string(src), "EnableQuantityPolicy")
	require.Contains(t, string(src), "ObserveQuantityPolicyShadow")
}
