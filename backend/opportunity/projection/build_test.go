package projection_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity/projection"
	"go-stock/backend/papertrading"
	"go-stock/backend/research"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupProjectionTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:proj_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, testDB.AutoMigrate(&models.SignalScanSnapshot{}))
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(func() {
		db.Dao = original
		papertrading.ResetConfigCache()
		_ = sqlDB.Close()
	})
}

func seedSignalSnapshot301125(t *testing.T, tradeDate string) uint {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE:           "301125.SZ",
				SECURITY_NAME_ABBR: "腾亚精工",
				Tag:                "突",
				StatusText:         "突破+放量",
				SignalTime:         tradeDate + "T15:00:00+08:00",
				SignalPrice:        42.15,
				SignalPriceStatus:  models.SignalPriceStatusFrozen,
				SchemaVersion:      models.SignalSchemaVersionV1,
			},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{
		TradeDate: tradeDate, Session: models.SignalScanSessionClose, Status: "done",
		ResultJSON: string(raw), HitTotal: 1,
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	return snap.ID
}

func seedSignalSnapshot600519(t *testing.T, tradeDate string) uint {
	t.Helper()
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE:           "600519.SH",
				SECURITY_NAME_ABBR: "贵州茅台",
				Tag:                "强",
				INDUSTRY:           "白酒",
				StatusText:         "趋势增强",
				SignalTime:         tradeDate + "T15:00:00+08:00",
				SignalPrice:        1800.0,
				SignalPriceStatus:  models.SignalPriceStatusFrozen,
				RSI:                28,
			},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	snap := &models.SignalScanSnapshot{
		TradeDate: tradeDate, Session: models.SignalScanSessionClose, Status: "done",
		ResultJSON: string(raw), HitTotal: 1,
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	return snap.ID
}

func seedPool301125(t *testing.T, tradeDate string, snapID uint) *models.CandidatePool {
	t.Helper()
	now := time.Now()
	pool := &models.CandidatePool{
		TradeDate: tradeDate, GeneratedAt: now,
		Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{
		{
			StockCode: "sz301125", StockName: "腾亚精工", Rank: 3, Score: 0.86,
			StrategyName: "trend_breakout", StrategyVersion: "run:1",
			SignalTag: "突", SignalSnapshotID: snapID, Reason: "strategy_run",
		},
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	return pool
}

// Case1: Signal + CandidatePool, no TradePlan → WATCH.
func TestProjectOne_Case1_SignalAndPool_NoPlan_Watch(t *testing.T) {
	setupProjectionTestDB(t)
	snapID := seedSignalSnapshot301125(t, "2026-09-01")
	seedPool301125(t, "2026-09-02", snapID)

	got, err := projection.ProjectOne("sz301125", projection.ProjectOptions{TradeDate: "2026-09-02"})
	require.NoError(t, err)
	require.NotNil(t, got)

	require.True(t, got.Signal.Present)
	require.Equal(t, "突", got.Signal.SignalTag)
	require.InDelta(t, 42.15, got.Signal.SignalPrice, 1e-6)
	require.Contains(t, got.Signal.TriggerReason, "突破")
	require.Equal(t, models.SignalPriceStatusFrozen, got.Signal.SignalStatus)

	require.True(t, got.Opportunity.Present)
	require.InDelta(t, 0.86, got.Opportunity.Score, 1e-6)
	require.Equal(t, 3, got.Opportunity.Rank)
	require.Contains(t, got.Opportunity.StrategySource, "trend_breakout")

	require.Equal(t, projection.DecisionStatusWatch, got.Decision.DecisionStatus)
	require.Equal(t, projection.CandidateStatusInPool, got.Decision.CandidateStatus)
	require.False(t, got.TradePlan.Present)
	require.NotEmpty(t, got.OpportunityID)
}

// Case2: Signal → Pool → TradePlan → Position full loop.
func TestProjectOne_Case2_FullClosedLoop(t *testing.T) {
	setupProjectionTestDB(t)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000}
	require.NoError(t, db.Dao.Create(acc).Error)

	snapID := seedSignalSnapshot301125(t, "2026-09-01")
	pool := seedPool301125(t, "2026-09-02", snapID)

	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-09-02", GeneratedAt: now, PoolID: pool.ID, MaxNames: 5,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
		DecisionProvider: models.TradePlanDecisionProviderFixed,
	}
	planItems := []models.TradePlanItem{
		{
			TradeDate: "2026-09-02", StockCode: "sz301125", StockName: "腾亚精工",
			Side: "buy", Priority: 1, TargetAmount: 100_000,
			Score: 0.86, StrategyName: "trend_breakout", Status: models.TradePlanItemPending,
		},
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems))

	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz301125", StockName: "腾亚精工",
		TotalVolume: 500, AvailableVolume: 500, AvgCost: 41.0, MarkPrice: 43.0,
	}).Error)

	got, err := projection.ProjectOne("sz301125", projection.ProjectOptions{TradeDate: "2026-09-02"})
	require.NoError(t, err)

	require.True(t, got.Signal.Present)
	require.True(t, got.Opportunity.Present)
	require.Equal(t, projection.DecisionStatusBuyCandidate, got.Decision.DecisionStatus)
	require.Equal(t, projection.CandidateStatusPlanPending, got.Decision.CandidateStatus)
	require.True(t, got.TradePlan.Present)
	require.Equal(t, plan.ID, got.TradePlan.PlanID)
	require.Equal(t, models.TradePlanItemPending, got.TradePlan.ItemStatus)
	require.Equal(t, projection.HoldingStatusHeld, got.Portfolio.HoldingStatus)
	require.InDelta(t, 500, got.Portfolio.PositionQty, 1e-6)
	require.Equal(t, projection.QualityComplete, got.Metadata.Quality)
}

// Case3: Research-only overlay; must not enter Decision as BUY_CANDIDATE.
func TestProjectOne_Case3_ResearchOnly_NoDecisionBuy(t *testing.T) {
	setupProjectionTestDB(t)
	tradeDate := "2026-09-01"
	seedSignalSnapshot600519(t, tradeDate)

	got, err := projection.ProjectOne("sh600519", projection.ProjectOptions{
		TradeDate:       tradeDate,
		IncludeResearch: true,
	})
	require.NoError(t, err)

	require.True(t, got.Signal.Present)
	require.Equal(t, "强", got.Signal.SignalTag)

	require.False(t, got.Opportunity.Present)
	require.NotEqual(t, projection.DecisionStatusBuyCandidate, got.Decision.DecisionStatus)
	require.Equal(t, projection.CandidateStatusNotInPool, got.Decision.CandidateStatus)
	require.False(t, got.TradePlan.Present)

	require.NotNil(t, got.Research)
	require.Equal(t, research.MakeCandidateID(tradeDate, "sh600519"), got.Research.ResearchID)
	require.NotNil(t, got.Research.SignalScore)
}

func TestProjectList_FromPoolRankOrder(t *testing.T) {
	setupProjectionTestDB(t)
	snapID := seedSignalSnapshot301125(t, "2026-09-01")
	seedPool301125(t, "2026-09-02", snapID)

	list, err := projection.ProjectList(projection.ProjectOptions{TradeDate: "2026-09-02", Limit: 5})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "sz301125", list[0].StockCode)
	require.True(t, list[0].Opportunity.Present)
}

func TestProjectOne_InvalidCode(t *testing.T) {
	setupProjectionTestDB(t)
	_, err := projection.ProjectOne("", projection.ProjectOptions{})
	require.ErrorIs(t, err, projection.ErrInvalidStockCode)
}
