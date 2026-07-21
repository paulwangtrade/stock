package strategy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupApproveTestDB(t *testing.T) {
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
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureTradePlanTables())
}

func seedDraftPlanForApprove(t *testing.T, tradeDate string) *models.TradePlan {
	t.Helper()
	plan := &models.TradePlan{
		TradeDate:      tradeDate,
		GeneratedAt:    time.Now(),
		Status:         models.TradePlanStatusDraft,
		PlanVersion:    1,
		EnableExecute:  false,
		AmountPerStock: 100_000,
		MaxNames:       5,
		SourceSession:  models.TradePlanSourceAfterClose,
		Side:           "buy",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{
			TradeDate:    tradeDate,
			StockCode:    "sz000001",
			StockName:    "平安",
			Priority:     1,
			TargetAmount: 100_000,
			Status:       models.TradePlanItemPending,
		},
	}))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func withPassRiskContext(t *testing.T) {
	t.Helper()
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled:        false,
			MarketLevel:    3,
			AmountPerStock: amount,
			MaxNames:       maxNames,
			Cash:           1_000_000,
			EquityBase:     1_000_000,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })
}

func withBlockRiskContext(t *testing.T) {
	t.Helper()
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled:         true,
			MarketLevel:     1,
			BlockNewEntries: true,
			AmountPerStock:  amount,
			MaxNames:        maxNames,
			Cash:            1_000_000,
			EquityBase:      1_000_000,
			ScanLimit:       10,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })
}

func TestApproveTradePlan_NormalDraft(t *testing.T) {
	setupApproveTestDB(t)
	withPassRiskContext(t)
	plan := seedDraftPlanForApprove(t, "2026-07-28")

	got, err := ApproveTradePlan(plan, "alice", "looks good")
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.False(t, got.EnableExecute)
	require.Nil(t, got.FreezeAt)
	require.NotNil(t, got.ApprovedAt)
	require.Equal(t, "alice", got.ApprovedBy)
	require.Equal(t, "looks good", got.ApprovalReason)
	require.True(t, got.IsDraft())
	require.False(t, got.IsExecutableStatus())
	require.False(t, got.IsFrozen())
}

func TestApproveTradePlan_RejectsFailedRiskProposal(t *testing.T) {
	setupApproveTestDB(t)
	withBlockRiskContext(t)
	plan := seedDraftPlanForApprove(t, "2026-07-28")

	_, err := ApproveTradePlan(plan, "alice", "should fail")
	require.Error(t, err)
	require.Contains(t, err.Error(), "approve denied")

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.Nil(t, got.ApprovedAt)
	require.Empty(t, got.ApprovedBy)
	require.Empty(t, got.ApprovalReason)
	require.Nil(t, got.FreezeAt)
}

func TestApproveTradePlan_RepeatApproveUpdatesAudit(t *testing.T) {
	setupApproveTestDB(t)
	withPassRiskContext(t)
	plan := seedDraftPlanForApprove(t, "2026-07-28")

	first, err := ApproveTradePlan(plan, "alice", "first")
	require.NoError(t, err)
	require.Equal(t, "alice", first.ApprovedBy)
	require.Equal(t, "first", first.ApprovalReason)
	firstAt := *first.ApprovedAt

	time.Sleep(2 * time.Millisecond)
	second, err := ApproveTradePlan(first, "bob", "second look")
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, second.Status)
	require.Equal(t, "bob", second.ApprovedBy)
	require.Equal(t, "second look", second.ApprovalReason)
	require.False(t, second.ApprovedAt.Before(firstAt))
	require.Nil(t, second.FreezeAt)
	require.False(t, second.IsExecutableStatus())
}

func TestApproveTradePlan_RejectsNonDraft(t *testing.T) {
	setupApproveTestDB(t)
	withPassRiskContext(t)
	plan := seedDraftPlanForApprove(t, "2026-07-28")
	plan.Status = models.TradePlanStatusReady
	_, err := ApproveTradePlan(plan, "alice", "nope")
	require.Error(t, err)
}

func TestApproveTradePlan_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("approve_trade_plan.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"FreezeAt:",
		"TradePlanStatusReady",
		"TryBeginExecute(",
		"RunDailyCandidateAndPlan(",
		"TradingPreflight",
		"PreTradeCheck(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("approve service must not reference %s", token)
		}
	}
	require.Contains(t, src, "ApproveDraft")
	require.Contains(t, src, "EvaluateDraftTradePlanRisk")
	require.Contains(t, src, "TradePlanStatusDraft")
}
