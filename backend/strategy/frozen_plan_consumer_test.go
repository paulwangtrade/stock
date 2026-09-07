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

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFrozenConsumerTestDB(t *testing.T) {
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

func seedPlanDirect(t *testing.T, plan *models.TradePlan, items []models.TradePlanItem) *models.TradePlan {
	t.Helper()
	now := time.Now()
	if plan.GeneratedAt.IsZero() {
		plan.GeneratedAt = now
	}
	plan.CreatedAt = now
	plan.UpdatedAt = now
	require.NoError(t, db.Dao.Create(plan).Error)
	for i := range items {
		items[i].PlanID = plan.ID
		items[i].TradeDate = plan.TradeDate
		items[i].CreatedAt = now
		items[i].UpdatedAt = now
		require.NoError(t, db.Dao.Create(&items[i]).Error)
	}
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func TestGetFrozenTradePlan_FindsFrozenReady(t *testing.T) {
	setupFrozenConsumerTestDB(t)
	freezeAt := time.Date(2026, 7, 21, 16, 0, 0, 0, time.Local)
	approvedAt := freezeAt.Add(-time.Hour)
	seeded := seedPlanDirect(t, &models.TradePlan{
		TradeDate:     "2026-07-22",
		Status:        models.TradePlanStatusReady,
		PlanVersion:   3,
		ApprovedAt:    &approvedAt,
		FreezeAt:      &freezeAt,
		FreezeBy:      "alice",
		SourceSession: models.TradePlanSourceAfterClose,
		Side:          "buy",
	}, []models.TradePlanItem{{
		StockCode: "sz000001", StockName: "平安", Priority: 1,
		TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})

	got, err := GetFrozenTradePlan("2026-07-22")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, seeded.ID, got.ID)
	require.True(t, got.IsFrozen())
	require.Equal(t, models.TradePlanStatusReady, got.Status)
	require.Equal(t, 3, got.PlanVersion)
	require.Len(t, got.Items, 1)
	require.Equal(t, "sz000001", got.Items[0].StockCode)
}

func TestGetFrozenTradePlan_ReadyWithoutFreezeAtExcluded(t *testing.T) {
	setupFrozenConsumerTestDB(t)
	seedPlanDirect(t, &models.TradePlan{
		TradeDate:   "2026-07-22",
		Status:      models.TradePlanStatusReady,
		PlanVersion: 1,
		FreezeAt:    nil,
		Side:        "buy",
	}, []models.TradePlanItem{{
		StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})

	got, err := GetFrozenTradePlan("2026-07-22")
	require.Error(t, err)
	require.Nil(t, got)

	// Still visible to legacy ready consumer.
	ready, err := data.NewTradePlanRepo().GetReadyByTradeDate("2026-07-22")
	require.NoError(t, err)
	require.False(t, ready.IsFrozen())
}

func TestGetFrozenTradePlan_DraftExcluded(t *testing.T) {
	setupFrozenConsumerTestDB(t)
	freezeAt := time.Now()
	// Even if FreezeAt is mistakenly set on a draft, Status must be ready.
	seedPlanDirect(t, &models.TradePlan{
		TradeDate:     "2026-07-22",
		Status:        models.TradePlanStatusDraft,
		PlanVersion:   2,
		FreezeAt:      &freezeAt,
		SourceSession: models.TradePlanSourceAfterClose,
		Side:          "buy",
	}, []models.TradePlanItem{{
		StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})

	got, err := GetFrozenTradePlan("2026-07-22")
	require.Error(t, err)
	require.Nil(t, got)
}

func TestGetFrozenTradePlan_MultiVersionPrefersHighestPlanVersion(t *testing.T) {
	setupFrozenConsumerTestDB(t)
	t1 := time.Date(2026, 7, 21, 15, 30, 0, 0, time.Local)
	t2 := time.Date(2026, 7, 21, 16, 0, 0, 0, time.Local)
	approvedAt := t1.Add(-time.Hour)

	// Bypass CreatePlanWithItems supersede so both rows stay ready+frozen for selection test.
	older := seedPlanDirect(t, &models.TradePlan{
		TradeDate: "2026-07-22", Status: models.TradePlanStatusReady,
		PlanVersion: 1, ApprovedAt: &approvedAt, FreezeAt: &t1, FreezeBy: "v1", Side: "buy",
		SourceSession: models.TradePlanSourceAfterClose,
	}, []models.TradePlanItem{{
		StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})
	newer := seedPlanDirect(t, &models.TradePlan{
		TradeDate: "2026-07-22", Status: models.TradePlanStatusReady,
		PlanVersion: 2, ApprovedAt: &approvedAt, FreezeAt: &t2, FreezeBy: "v2", Side: "buy",
		SourceSession: models.TradePlanSourceAfterClose,
	}, []models.TradePlanItem{{
		StockCode: "sz000002", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})
	require.NotEqual(t, older.ID, newer.ID)
	require.Less(t, older.PlanVersion, newer.PlanVersion)

	got, err := GetFrozenTradePlan("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, newer.ID, got.ID, "must prefer highest plan_version among frozen ready")
	require.Equal(t, 2, got.PlanVersion)
	require.Equal(t, "sz000002", got.Items[0].StockCode)

	// Same version: higher id wins.
	t3 := time.Date(2026, 7, 21, 17, 0, 0, 0, time.Local)
	sameVerLater := seedPlanDirect(t, &models.TradePlan{
		TradeDate: "2026-07-22", Status: models.TradePlanStatusReady,
		PlanVersion: 2, ApprovedAt: &approvedAt, FreezeAt: &t3, FreezeBy: "v2b", Side: "buy",
		SourceSession: models.TradePlanSourceAfterClose,
	}, []models.TradePlanItem{{
		StockCode: "sz000003", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})
	got2, err := GetFrozenTradePlan("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, sameVerLater.ID, got2.ID, "equal plan_version → highest id")
}

func TestGetFrozenTradePlan_DoesNotMutate(t *testing.T) {
	setupFrozenConsumerTestDB(t)
	freezeAt := time.Now()
	approvedAt := freezeAt.Add(-time.Hour)
	seeded := seedPlanDirect(t, &models.TradePlan{
		TradeDate: "2026-07-22", Status: models.TradePlanStatusReady,
		PlanVersion: 1, ApprovedAt: &approvedAt, FreezeAt: &freezeAt, Side: "buy",
	}, []models.TradePlanItem{{
		StockCode: "sz000001", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
	}})

	_, err := GetFrozenTradePlan("2026-07-22")
	require.NoError(t, err)

	again, err := data.NewTradePlanRepo().GetByID(seeded.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, again.Status)
	require.True(t, again.IsFrozen())
}

func TestFrozenPlanConsumer_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frozen_plan_consumer.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"BuildCandidatePool(",
		"BuildTradePlan(",
		"CreatePlanWithItems(",
		"RunDailyCandidateAndPlan(",
		"ApproveTradePlan(",
		"FreezeTradePlan(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunPaperOpenPrepare(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("consumer must not reference %s", token)
		}
	}
	require.Contains(t, src, "GetFrozenByTradeDate")
}
