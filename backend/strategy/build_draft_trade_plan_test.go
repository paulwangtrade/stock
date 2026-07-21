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

func setupDraftPlanTestDB(t *testing.T) {
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
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    false,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))
}

func seedReadyPool(t *testing.T, tradeDate string, codes ...string) *models.CandidatePool {
	t.Helper()
	items := make([]models.CandidatePoolItem, 0, len(codes))
	for i, code := range codes {
		items = append(items, models.CandidatePoolItem{
			TradeDate: tradeDate,
			StockCode: code,
			StockName: code,
			Rank:      i + 1,
			Score:     float64(len(codes) - i),
		})
	}
	pool := &models.CandidatePool{
		TradeDate:   tradeDate,
		GeneratedAt: time.Now(),
		Source:      models.CandidatePoolSourceFollow,
		Status:      models.CandidatePoolStatusReady,
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	return pool
}

func TestBuildDraftTradePlanFromCandidatePool_PersistsDraftFields(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-07-28", "sz000001", "sh600519")

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.False(t, plan.EnableExecute)
	require.Equal(t, models.TradePlanSourceAfterClose, plan.SourceSession)
	require.Equal(t, pool.ID, plan.PoolID)
	require.Equal(t, "2026-07-28", plan.TradeDate)
	require.Equal(t, 1, plan.PlanVersion)
	require.Nil(t, plan.FreezeAt)
	require.Nil(t, plan.ApprovedAt)
	require.Empty(t, plan.ApprovedBy)
	require.GreaterOrEqual(t, len(plan.Items), 1)
	require.True(t, plan.IsDraft())
	require.False(t, plan.IsExecutableStatus())
}

func TestBuildDraftTradePlanFromCandidatePool_AppendOnlyVersions(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-07-28", "sz000001")

	first, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	second, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)

	require.Equal(t, 1, first.PlanVersion)
	require.Equal(t, 2, second.PlanVersion)
	require.NotEqual(t, first.ID, second.ID)

	gotFirst, err := data.NewTradePlanRepo().GetByID(first.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, gotFirst.Status)
	require.Equal(t, 1, gotFirst.PlanVersion)

	gotSecond, err := data.NewTradePlanRepo().GetByID(second.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, gotSecond.Status)
	require.Equal(t, 2, gotSecond.PlanVersion)
}

func TestBuildDraftTradePlanFromCandidatePool_RejectsInvalidPool(t *testing.T) {
	setupDraftPlanTestDB(t)

	_, err := BuildDraftTradePlanFromCandidatePool(nil)
	require.Error(t, err)

	_, err = BuildDraftTradePlanFromCandidatePool(&models.CandidatePool{
		ID: 1, TradeDate: "2026-07-28", Status: models.CandidatePoolStatusReady,
	})
	require.Error(t, err)

	pool := seedReadyPool(t, "2026-07-28", "sz000001")
	pool.Status = models.CandidatePoolStatusFailed
	_, err = BuildDraftTradePlanFromCandidatePool(pool)
	require.Error(t, err)
}

func TestBuildDraftTradePlanFromCandidatePool_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("build_draft_trade_plan.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"RunDailyCandidateAndPlan(",
		"TryBeginExecute(",
		"TradePlanStatusReady",
		"ApprovedAt:",
		"ApprovedBy:",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("draft builder must not reference %s", token)
		}
	}
	require.Contains(t, src, "TradePlanStatusDraft")
	require.Contains(t, src, "EnableExecute:     false")
	require.Contains(t, src, "TradePlanSourceAfterClose")
	require.Contains(t, src, "NextPlanVersion")
}
