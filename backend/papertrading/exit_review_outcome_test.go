package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedOutcomeAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedOutcomeFill(t *testing.T, accID uint, tradeDate, code, name string, price float64, vol int64) {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{TradeDate: tradeDate, GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: tradeDate, StockCode: code, StockName: name,
		Side: "buy", Status: models.TradePlanItemPending,
	}
	require.NoError(t, db.Dao.Create(item).Error)
	order := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: tradeDate,
		StockCode: code, StockName: name, Side: "buy", Quantity: vol, OrderPrice: price,
		Status: papertrading.OrderStatusFilled, FilledPrice: price, FilledVolume: vol, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: code, StockName: name, Side: "buy", Price: price, Volume: vol, FilledAt: now,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
}

func TestSaveExitReviewOutcome_HOLD(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)

	row, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:           acc.ID,
		StockCode:           "SH600363",
		Decision:            papertrading.ExitReviewDecisionHold,
		Reason:              "逻辑仍成立",
		ExitStateSnapshot:   papertrading.ExitEvalStateReviewRequired,
		ReasonCodesSnapshot: []string{papertrading.ExitReasonLossReview},
	})
	require.NoError(t, err)
	require.NotEmpty(t, row.ID)
	require.Equal(t, "sh600363", row.StockCode)
	require.Equal(t, papertrading.ExitReviewDecisionHold, row.Decision)
	require.Equal(t, "逻辑仍成立", row.Reason)
	require.Equal(t, papertrading.ExitEvalStateReviewRequired, row.ExitStateSnapshot)
}

func TestSaveExitReviewOutcome_WATCH(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)

	row, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:           acc.ID,
		StockCode:           "sz000001",
		Decision:            papertrading.ExitReviewDecisionWatch,
		ExitStateSnapshot:   papertrading.ExitEvalStateWatch,
		ReasonCodesSnapshot: []string{papertrading.ExitReasonTimeReview},
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExitReviewDecisionWatch, row.Decision)
}

func TestSaveExitReviewOutcome_CREATE_SELL_PLAN(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)
	planID := uint(42)

	row, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:           acc.ID,
		StockCode:           "sh600105",
		Decision:            papertrading.ExitReviewDecisionCreateSellPlan,
		ExitStateSnapshot:   papertrading.ExitEvalStateReviewRequired,
		ReasonCodesSnapshot: []string{papertrading.ExitReasonLossReview},
		RelatedTradePlanID:  &planID,
	})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExitReviewDecisionCreateSellPlan, row.Decision)
	require.NotNil(t, row.RelatedTradePlanID)
	require.Equal(t, planID, *row.RelatedTradePlanID)
}

func TestLoadLatestOutcomeForStock(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	_, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionHold,
		ReviewTime:        older,
		ExitStateSnapshot: papertrading.ExitEvalStateWatch,
	})
	require.NoError(t, err)

	_, err = papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionWatch,
		ReviewTime:        newer,
		ExitStateSnapshot: papertrading.ExitEvalStateWatch,
	})
	require.NoError(t, err)

	latest, err := papertrading.LoadLatestOutcomeForStock(acc.ID, "SH600363")
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, papertrading.ExitReviewDecisionWatch, latest.Decision)
}

func TestAttachLatestOutcomes(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)

	today := time.Now().Format("2006-01-02")
	seedOutcomeFill(t, acc.ID, today, "sh600363", "联创光电", 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 8.0,
	}).Error)

	_, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionHold,
		ExitStateSnapshot: papertrading.ExitEvalStateReviewRequired,
	})
	require.NoError(t, err)

	view, err := papertrading.BuildExitEvaluation(papertrading.ExitEvaluationBuildOptions{})
	require.NoError(t, err)
	require.NoError(t, papertrading.AttachLatestOutcomes(view))
	require.Len(t, view.Holdings, 1)
	require.NotNil(t, view.Holdings[0].LatestOutcome)
	require.Equal(t, papertrading.ExitReviewDecisionHold, view.Holdings[0].LatestOutcome.Decision)
}

func TestSaveExitReviewOutcome_DuplicateSubmit(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	first, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionHold,
		ReviewTime:        older,
		ExitStateSnapshot: papertrading.ExitEvalStateReviewRequired,
		Reason:            "第一次 HOLD",
	})
	require.NoError(t, err)

	second, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionWatch,
		ReviewTime:        newer,
		ExitStateSnapshot: papertrading.ExitEvalStateWatch,
		Reason:            "第二次 WATCH",
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)

	var count int64
	require.NoError(t, db.Dao.Model(&papertrading.ExitReviewOutcome{}).
		Where("account_id = ? AND stock_code = ?", acc.ID, "sh600363").
		Count(&count).Error)
	require.Equal(t, int64(2), count)

	latest, err := papertrading.LoadLatestOutcomeForStock(acc.ID, "sh600363")
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, papertrading.ExitReviewDecisionWatch, latest.Decision)
	require.Equal(t, "第二次 WATCH", latest.Reason)
}

func TestLoadLatestOutcomesByAccount_LazyMigrateMissingTable(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)
	require.True(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))
	require.NoError(t, db.Dao.Migrator().DropTable(&papertrading.ExitReviewOutcome{}))
	require.False(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))

	out, err := papertrading.LoadLatestOutcomesByAccount(acc.ID, []string{"sh600363"})
	require.NoError(t, err)
	require.Empty(t, out)
	require.True(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))
}

func TestSaveExitReviewOutcome_LazyMigrateMissingTable(t *testing.T) {
	setupTestDB(t)
	acc := seedOutcomeAccount(t)
	require.NoError(t, db.Dao.Migrator().DropTable(&papertrading.ExitReviewOutcome{}))
	require.False(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))

	row, err := papertrading.SaveExitReviewOutcome(papertrading.SaveExitReviewOutcomeInput{
		AccountID:         acc.ID,
		StockCode:         "sh600363",
		Decision:          papertrading.ExitReviewDecisionHold,
		ExitStateSnapshot: papertrading.ExitEvalStateReviewRequired,
	})
	require.NoError(t, err)
	require.NotEmpty(t, row.ID)
	require.True(t, db.Dao.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))
}
