package investmentnarrative_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/investmentnarrative"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupNarrativeTestDB(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = original })

	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}))
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)

	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 500_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedSignalSnapshot(t *testing.T, code, tag string, signalPrice float64, signalTime string) {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE:          "600363.SH",
				SECURITY_NAME_ABBR: "联创光电",
				Tag:               tag,
				SignalTime:        signalTime,
				SignalPrice:       signalPrice,
				SignalPriceStatus: models.SignalPriceStatusFrozen,
			},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		TradeDate: "2026-08-18", Session: "close", StrategyID: "default",
		Status: "done", HitTotal: 1, ResultJSON: string(raw),
	}).Error)
	_ = code
}

func seedPosition(t *testing.T, accID uint, code, name string, vol int64, avgCost, mark float64) {
	t.Helper()
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: accID, StockCode: code, StockName: name,
		TotalVolume: vol, AvailableVolume: vol, AvgCost: avgCost, MarkPrice: mark,
		UpdatedAt: time.Now(),
	}).Error)
}

func seedBuyPlanFill(t *testing.T, accID uint, code, name, strategy, reason string, price float64, vol int64) uint {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{TradeDate: "2026-08-18", GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: "2026-08-18", StockCode: code, StockName: name,
		Side: "buy", StrategyName: strategy, Reason: reason,
	}
	require.NoError(t, db.Dao.Create(item).Error)
	order := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: "2026-08-18",
		StockCode: code, StockName: name, Side: "buy", Quantity: vol, OrderPrice: price,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: code, StockName: name, Side: "buy", Price: price, Volume: vol, FilledAt: now,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	return plan.ID
}

func TestInvestmentNarrative_SignalOnlyNoPlan(t *testing.T) {
	setupNarrativeTestDB(t)
	seedSignalSnapshot(t, "sh600363", "强", 28.5, "2026-08-18")

	n, err := investmentnarrative.BuildInvestmentNarrative(investmentnarrative.BuildOptions{
		StockCode: "sh600363",
	})
	require.NoError(t, err)
	require.True(t, n.Discovery.Exists)
	require.Equal(t, "2026-08-18", n.Discovery.SignalTime)
	require.NotNil(t, n.Discovery.SignalPrice)
	require.Equal(t, 28.5, *n.Discovery.SignalPrice)
	require.Equal(t, "强", n.Discovery.Reason)
	require.False(t, n.PlanOrigin.Exists)
	require.Equal(t, investmentnarrative.SectionStatusMissing, n.PlanOrigin.Status)
	require.False(t, n.HoldingBasis.Exists)
}

func TestInvestmentNarrative_PlanAndHolding(t *testing.T) {
	acc := setupNarrativeTestDB(t)
	seedSignalSnapshot(t, "sh600363", "强", 28.0, "2026-08-18")
	planID := seedBuyPlanFill(t, acc.ID, "sh600363", "联创光电", "momentum_v1", "breakout signal", 30.0, 1000)
	seedPosition(t, acc.ID, "sh600363", "联创光电", 1000, 30.0, 33.0)

	n, err := investmentnarrative.BuildInvestmentNarrative(investmentnarrative.BuildOptions{
		StockCode: "600363.SH",
		AccountID: acc.ID,
	})
	require.NoError(t, err)
	require.True(t, n.PlanOrigin.Exists)
	require.Equal(t, planID, n.PlanOrigin.PlanID)
	require.Equal(t, "momentum_v1", n.PlanOrigin.Strategy)
	require.Equal(t, "breakout signal", n.PlanOrigin.Reason)
	require.True(t, n.HoldingBasis.Exists)
	require.Equal(t, int64(1000), n.HoldingBasis.Quantity)
	require.Equal(t, 30.0, n.HoldingBasis.AvgCost)
	require.NotNil(t, n.PriceStory.SignalPrice)
	require.NotNil(t, n.PriceStory.CurrentPrice)
	require.NotNil(t, n.PriceStory.VsSignalPct)
}

func TestInvestmentNarrative_ExitOutcome(t *testing.T) {
	acc := setupNarrativeTestDB(t)
	seedBuyPlanFill(t, acc.ID, "sh600363", "联创光电", "s1", "r1", 10.0, 500)
	seedPosition(t, acc.ID, "sh600363", "联创光电", 500, 10.0, 9.0)

	_, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionWatch,
		ExitStateSnapshot: papertrading.ExitEvalStateWatch,
		ReasonCodesSnapshot: []string{papertrading.ExitReasonLossReview},
	})
	require.NoError(t, err)

	n, err := investmentnarrative.BuildInvestmentNarrative(investmentnarrative.BuildOptions{
		StockCode: "sh600363",
		AccountID: acc.ID,
	})
	require.NoError(t, err)
	require.True(t, n.ExitReview.Exists)
	require.NotEmpty(t, n.ExitReview.Status)
	require.NotEqual(t, investmentnarrative.SectionStatusMissing, n.ExitReview.Status)
	require.NotNil(t, n.ExitReview.LatestOutcome)
	require.Equal(t, papertrading.ExitReviewDecisionWatch, n.ExitReview.LatestOutcome.Decision)
}

func TestInvestmentNarrative_AllMissing(t *testing.T) {
	setupNarrativeTestDB(t)

	n, err := investmentnarrative.BuildInvestmentNarrative(investmentnarrative.BuildOptions{
		StockCode: "sh999999",
	})
	require.NoError(t, err)
	require.Equal(t, "sh999999", n.StockCode)
	require.False(t, n.Discovery.Exists)
	require.False(t, n.PlanOrigin.Exists)
	require.False(t, n.HoldingBasis.Exists)
	require.False(t, n.ExitReview.Exists)
	require.Equal(t, investmentnarrative.SectionStatusNA, n.ExitReview.Status)
	require.Equal(t, investmentnarrative.SectionStatusMissing, n.PriceStory.Status)
}
