package strategysnapshot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/strategysnapshot"
	"go-stock/backend/strategyexplain"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPersistDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:sshot_persist_%s?mode=memory&cache=shared", t.Name())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, strategysnapshot.MigrateStrategySnapshots(gdb))
	return gdb
}

func TestMigrateStrategySnapshots_Idempotent(t *testing.T) {
	gdb := setupPersistDB(t)
	require.NoError(t, strategysnapshot.MigrateStrategySnapshots(gdb))
	require.True(t, gdb.Migrator().HasTable(&strategysnapshot.StrategySnapshotRow{}))
	require.True(t, gdb.Migrator().HasTable(&strategysnapshot.PlanStrategyRefRow{}))
	require.True(t, gdb.Migrator().HasColumn(&strategysnapshot.StrategySnapshotRow{}, "PayloadJSON"))
	require.True(t, gdb.Migrator().HasColumn(&strategysnapshot.PlanStrategyRefRow{}, "StrategySnapshotID"))
}

func TestSQLiteStore_CapturePersistsAndSurvivesMemoryClear(t *testing.T) {
	gdb := setupPersistDB(t)
	mem := strategysnapshot.NewMemoryStore()
	store := strategysnapshot.NewLayeredStore(strategysnapshot.NewSQLiteStore(gdb), mem)
	svc := strategysnapshot.NewService(store)

	plan := samplePlan()
	pool := samplePool()
	res, err := svc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{
		Plan: plan,
		Pool: pool,
		Now:  time.Date(2026, 9, 4, 16, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.ItemSnaps)

	var snapCount int64
	require.NoError(t, gdb.Model(&strategysnapshot.StrategySnapshotRow{}).Count(&snapCount).Error)
	require.GreaterOrEqual(t, snapCount, int64(1+len(res.ItemSnaps)))

	var refCount int64
	require.NoError(t, gdb.Model(&strategysnapshot.PlanStrategyRefRow{}).Where("plan_id = ?", plan.ID).Count(&refCount).Error)
	require.GreaterOrEqual(t, refCount, int64(1+len(res.ItemSnaps)))

	// Simulate restart: new layered store with fresh memory over same DB.
	cold := strategysnapshot.NewService(strategysnapshot.NewLayeredStore(
		strategysnapshot.NewSQLiteStore(gdb),
		strategysnapshot.NewMemoryStore(),
	))
	got, err := cold.Get(res.ItemSnaps[0].SnapshotID)
	require.NoError(t, err)
	require.Equal(t, "强", got.SignalResult.Tag)
	require.Equal(t, 10.5, got.MarketDataRef.RefPrice)

	ref, err := cold.GetPlanReference(plan.ID)
	require.NoError(t, err)
	require.Equal(t, res.ItemSnaps[0].SnapshotID, ref.ItemSnapshotIDs[1001])
}

func TestExplain_UsesDBAfterMemoryCleared(t *testing.T) {
	gdb := setupPersistDB(t)
	store := strategysnapshot.NewLayeredStore(strategysnapshot.NewSQLiteStore(gdb), strategysnapshot.NewMemoryStore())
	snapSvc := strategysnapshot.NewService(store)
	plan := samplePlan()
	pool := samplePool()
	res, err := snapSvc.CaptureAfterTradePlanCreate(strategysnapshot.CaptureInput{Plan: plan, Pool: pool})
	require.NoError(t, err)

	// Cold service: memory empty, DB has rows.
	coldSnap := strategysnapshot.NewService(strategysnapshot.NewLayeredStore(
		strategysnapshot.NewSQLiteStore(gdb),
		strategysnapshot.NewMemoryStore(),
	))
	expl := strategyexplain.NewService(coldSnap)
	u := &featuregate.User{ID: "persist-pro", Tier: featuregate.TierPro}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(u))

	out, err := expl.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:       u,
		PlanID:     plan.ID,
		PlanItemID: 1001,
	})
	require.NoError(t, err)
	require.NotEqual(t, strategyexplain.StatusMissing, out.Status)
	require.Equal(t, res.ItemSnaps[0].SnapshotID, out.SnapshotID)
	require.True(t, out.Sections.Signal.Available)
	require.Equal(t, "强", out.Sections.Signal.Tag)
	require.Equal(t, 10.5, out.Sections.Entry.RefPrice)
}

func TestExplain_MissingWhenNoSnapshot(t *testing.T) {
	gdb := setupPersistDB(t)
	coldSnap := strategysnapshot.NewService(strategysnapshot.NewLayeredStore(
		strategysnapshot.NewSQLiteStore(gdb),
		strategysnapshot.NewMemoryStore(),
	))
	expl := strategyexplain.NewService(coldSnap)
	u := &featuregate.User{ID: "persist-pro2", Tier: featuregate.TierPro}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(u))

	out, err := expl.Explain(context.Background(), strategyexplain.ExplainRequest{
		User:       u,
		PlanID:     99999,
		PlanItemID: 1,
	})
	require.NoError(t, err)
	require.Equal(t, strategyexplain.StatusMissing, out.Status)
	require.Contains(t, out.Headline, "未保存策略快照")
}
