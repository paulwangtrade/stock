package tradeplanorigin_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/tradeplanorigin"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOriginTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:origin_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, testDB.AutoMigrate(&models.SignalScanSnapshot{}))
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func seedSignalSnapshot(t *testing.T, code, tag string, signalPrice float64, signalTime string) uint {
	t.Helper()
	secucode := "600363.SH"
	if code == "sz000001" {
		secucode = "000001.SZ"
	}
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE:          secucode,
				SECURITY_CODE:     "000001",
				SECURITY_NAME_ABBR: "平安银行",
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
	snap := &models.SignalScanSnapshot{
		TradeDate: "2026-08-28", Session: "close", Status: "done", HitTotal: 1, ResultJSON: string(raw),
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	return snap.ID
}

func TestProjectPlanOrigin_WithSignalAndPlan(t *testing.T) {
	setupOriginTestDB(t)
	snapID := seedSignalSnapshot(t, "sz000001", "突", 12.34, "2026-08-28T15:00:00+08:00")

	poolRepo := data.NewCandidatePoolRepo()
	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-08-28", GeneratedAt: now, Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{
		{
			StockCode: "sz000001", StockName: "平安银行", Rank: 2, Score: 0.82,
			Reason: models.CandidatePoolSourceStrategyRun, StrategyName: "动量策略 v1",
			SignalTag: "突", SignalSnapshotID: snapID,
		},
	}
	require.NoError(t, poolRepo.CreatePoolWithItems(pool, items))

	planRepo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-08-29", GeneratedAt: now, PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusDraft, PlanVersion: 1, SourceSession: models.TradePlanSourceAfterClose,
	}
	planItems := []models.TradePlanItem{
		{
			TradeDate: "2026-08-29", StockCode: "sz000001", Side: "buy", Priority: 1,
			Score: 0.82, Reason: models.CandidatePoolSourceStrategyRun, StrategyName: "动量策略 v1",
			Status: models.TradePlanItemPending,
		},
	}
	require.NoError(t, planRepo.CreatePlanWithItems(plan, planItems))

	got, err := tradeplanorigin.ProjectPlanOrigin(plan.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	origin := got[0]
	require.Equal(t, "sz000001", origin.StockCode)
	require.Equal(t, fmt.Sprintf("%d", plan.ID), origin.PlanID)
	require.Equal(t, "2026-08-28T15:00:00+08:00", origin.SignalTime)
	require.Equal(t, "12.34", origin.SignalPrice)
	require.Equal(t, "突", origin.SignalTag)
	require.Equal(t, snapID, origin.SignalSnapshotID)
	require.Equal(t, "动量策略 v1", origin.StrategyName)
	require.Equal(t, "0.82", origin.Score)
	require.Contains(t, origin.SourceReason, "动量策略 v1")
	require.Contains(t, origin.SourceReason, "突")
	require.Contains(t, origin.SelectionReason, "排名第 2")
	require.Contains(t, origin.SelectionReason, "纳入计划")
}

func TestProjectPlanOrigin_PlanWithoutSignal(t *testing.T) {
	setupOriginTestDB(t)

	poolRepo := data.NewCandidatePoolRepo()
	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: "2026-08-28", GeneratedAt: now, Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{
		{
			StockCode: "sh600363", Rank: 1, Score: 0.55,
			Reason: models.CandidatePoolSourceStrategyRun, StrategyName: "默认策略",
		},
	}
	require.NoError(t, poolRepo.CreatePoolWithItems(pool, items))

	planRepo := data.NewTradePlanRepo()
	plan := &models.TradePlan{
		TradeDate: "2026-08-29", GeneratedAt: now, PoolID: pool.ID, MaxNames: 3,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
	}
	planItems := []models.TradePlanItem{
		{
			TradeDate: "2026-08-29", StockCode: "sh600363", Side: "buy",
			Score: 0.55, Reason: models.CandidatePoolSourceStrategyRun, StrategyName: "默认策略",
			Status: models.TradePlanItemPending,
		},
	}
	require.NoError(t, planRepo.CreatePlanWithItems(plan, planItems))

	got, err := tradeplanorigin.ProjectPlanOrigin(plan.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	origin := got[0]
	require.Equal(t, tradeplanorigin.Missing, origin.SignalTime)
	require.Equal(t, tradeplanorigin.Missing, origin.SignalPrice)
	require.Equal(t, tradeplanorigin.Missing, origin.SignalTag)
	require.Equal(t, "默认策略", origin.StrategyName)
	require.Equal(t, "0.55", origin.Score)
	require.Equal(t, "策略选股结果入选（当日无匹配信号快照）", origin.SourceReason)
	require.Contains(t, origin.SelectionReason, "纳入计划")
}

func TestProjectPlanOrigin_AllMissing(t *testing.T) {
	setupOriginTestDB(t)

	planRepo := data.NewTradePlanRepo()
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-08-29", GeneratedAt: now, Status: models.TradePlanStatusDraft, PlanVersion: 1,
	}
	planItems := []models.TradePlanItem{
		{TradeDate: "2026-08-29", StockCode: "sh999999", Side: "buy", Status: models.TradePlanItemPending},
	}
	require.NoError(t, planRepo.CreatePlanWithItems(plan, planItems))

	got, err := tradeplanorigin.ProjectPlanOrigin(plan.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	origin := got[0]
	require.Equal(t, "sh999999", origin.StockCode)
	require.Equal(t, fmt.Sprintf("%d", plan.ID), origin.PlanID)
	require.Equal(t, tradeplanorigin.Missing, origin.SignalTime)
	require.Equal(t, tradeplanorigin.Missing, origin.SignalPrice)
	require.Equal(t, tradeplanorigin.Missing, origin.SignalTag)
	require.Equal(t, tradeplanorigin.Missing, origin.SourceReason)
	require.Equal(t, tradeplanorigin.Missing, origin.StrategyName)
	require.Equal(t, tradeplanorigin.Missing, origin.Score)
	require.Equal(t, tradeplanorigin.Missing, origin.SelectionReason)
}

func TestProjectPlanOrigin_PlanNotFound(t *testing.T) {
	setupOriginTestDB(t)
	_, err := tradeplanorigin.ProjectPlanOrigin(99999)
	require.Error(t, err)
}
