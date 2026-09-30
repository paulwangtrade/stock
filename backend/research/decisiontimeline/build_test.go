package decisiontimeline

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTimelineDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:decision_timeline_%d?mode=memory&cache=shared&_busy_timeout=10000", time.Now().UnixNano())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, testDB.AutoMigrate(
		&models.SignalScanSnapshot{},
		&opportunity.UserOpportunityAction{},
		&data.FollowedStock{},
		&data.TradingRecord{},
	))
	require.NoError(t, papertrading.EnsureSchema(testDB))
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestBuild_SmokeEvidenceTrailDoesNotWriteOrders(t *testing.T) {
	setupTimelineDB(t)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 100000, Cash: 100000,
	}).Error)

	iceRaw, err := json.Marshal(models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{
			SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安银行",
			Tag: "冰", SignalPrice: 10.5, SignalTime: "2026-08-01T15:00:00+08:00",
		}},
	})
	require.NoError(t, err)
	trendRaw, err := json.Marshal(models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{
			SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安银行",
			Tag: "趋", SignalPrice: 11.2, SignalTime: "2026-08-20T15:00:00+08:00",
		}},
	})
	require.NoError(t, err)
	otherRaw, err := json.Marshal(models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{
			SECUCODE: "600000.SH", SECURITY_CODE: "600000", Tag: "冰",
		}},
	})
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		TradeDate: "2026-08-01", Session: "close", Status: "done", ResultJSON: string(iceRaw),
	}).Error)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		TradeDate: "2026-08-20", Session: "close", Status: "done", StrategyName: "趋势观察", ResultJSON: string(trendRaw),
	}).Error)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		TradeDate: "2026-08-20", Session: "close", Status: "done", ResultJSON: string(otherRaw),
	}).Error)

	watchedAt := time.Date(2026, 8, 21, 9, 30, 0, 0, shanghai)
	require.NoError(t, db.Dao.Create(&opportunity.UserOpportunityAction{
		ID: "uoa_smoke", AccountID: 1, ScanBatchKey: "snap:1", OpportunityID: "opp_smoke",
		StockCode: "sz000001", Action: opportunity.ActionWatch, CreatedBy: "ui", CreatedAt: watchedAt,
	}).Error)
	require.NoError(t, db.Dao.Create(&data.FollowedStock{
		StockCode: "sz000001", Name: "平安银行", Time: time.Date(2026, 8, 2, 10, 0, 0, 0, shanghai),
	}).Error)

	orderTime := time.Date(2026, 8, 22, 9, 31, 0, 0, shanghai)
	order := &papertrading.PaperSimOrder{
		AccountID: 1, PlanID: 0, PlanItemID: 0, TradeDate: "2026-08-22",
		StockCode: "sz000001", StockName: "平安银行", Side: "buy", Quantity: 100,
		OrderPrice: 11, Status: papertrading.OrderStatusFilled, OrderTime: orderTime,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
		AccountID: 1, OrderID: order.ID, StockCode: "sz000001", StockName: "平安银行",
		Side: "buy", Price: 11, Volume: 100, FilledAt: orderTime,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: 1, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 100, AvailableVolume: 0, AvgCost: 11,
		UpdatedAt: time.Date(2026, 8, 25, 15, 0, 0, 0, shanghai),
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.ExitReviewOutcome{
		ID: "ero_smoke", AccountID: 1, StockCode: "sz000001",
		ReviewTime: time.Date(2026, 9, 1, 15, 0, 0, 0, shanghai),
		Decision:   papertrading.ExitReviewDecisionHold,
		Reason:     "还在观察，不是卖出", CreatedBy: "ui",
		ExitStateSnapshot: "watch",
	}).Error)
	require.NoError(t, db.Dao.Create(&data.TradingRecord{
		StockCode: "sz000001", Direction: "卖出", Status: "confirmed",
		Reason: "当时觉得该兑现", Mindset: "事后看并不确定",
		TradingTime: time.Date(2026, 9, 2, 14, 0, 0, 0, shanghai),
	}).Error)

	var ordersBefore, actionsBefore int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&ordersBefore).Error)
	require.NoError(t, db.Dao.Model(&opportunity.UserOpportunityAction{}).Count(&actionsBefore).Error)
	planTableBefore := db.Dao.Migrator().HasTable(&models.TradePlan{})

	tl, err := Build("000001.SZ", Options{})
	require.NoError(t, err)
	require.Equal(t, Disclaimer, tl.Disclaimer)
	require.True(t, tl.ObservationOnly)
	require.Equal(t, "平安银行", tl.StockName)
	require.NotEmpty(t, tl.Events)

	var ordersAfter, actionsBeforeAfter int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&ordersAfter).Error)
	require.NoError(t, db.Dao.Model(&opportunity.UserOpportunityAction{}).Count(&actionsBeforeAfter).Error)
	require.Equal(t, ordersBefore, ordersAfter)
	require.Equal(t, actionsBefore, actionsBeforeAfter)
	require.Equal(t, planTableBefore, db.Dao.Migrator().HasTable(&models.TradePlan{}))

	var sawIce, sawTrend, sawWatch, sawFollow, sawFill, sawHold, sawReview, sawLog bool
	var lanes = map[string]map[string]struct{}{}
	prevDay := "9999-99-99"
	for _, ev := range tl.Events {
		if ev.OccurredOn != "" {
			require.LessOrEqual(t, ev.OccurredOn, prevDay)
			prevDay = ev.OccurredOn
		}
		if lanes[ev.Lane] == nil {
			lanes[ev.Lane] = map[string]struct{}{}
		}
		lanes[ev.Lane][ev.Kind] = struct{}{}
		switch {
		case ev.Kind == KindSignal && ev.Tag == "冰":
			sawIce = true
			require.Equal(t, LaneObserve, ev.Lane)
			require.Equal(t, "2026-08-01", ev.KlineDate)
		case ev.Kind == KindSignal && ev.Tag == "趋":
			sawTrend = true
			require.Contains(t, ev.Detail, "趋势观察")
		case ev.Kind == KindWatch:
			sawWatch = true
			require.Equal(t, LaneObserve, ev.Lane)
		case ev.Kind == KindFollow:
			sawFollow = true
		case ev.Kind == KindPaperSim && ev.Lane == LaneEntry && ev.Source == "paper_sim_fills":
			sawFill = true
			require.NotContains(t, ev.Title, "下单")
		case ev.Kind == KindPaperSim && ev.Lane == LaneHolding:
			sawHold = true
		case ev.Kind == KindOutcome && ev.Lane == LaneHolding && ev.Source == "exit_review_outcomes":
			sawReview = true
			require.Contains(t, ev.Detail, "不会据此生成卖出计划")
		case ev.Kind == KindOutcome && ev.Lane == LaneExit && ev.Source == "trading_records":
			sawLog = true
		}
	}
	require.NotContains(t, lanes[LaneExit], KindPaperSim)
	require.NotContains(t, lanes[LaneEntry], KindSignal)
	require.True(t, sawIce)
	require.True(t, sawTrend)
	require.True(t, sawWatch)
	require.True(t, sawFollow)
	require.True(t, sawFill)
	require.True(t, sawHold)
	require.True(t, sawReview)
	require.True(t, sawLog)
	require.Contains(t, lanes[LaneObserve], KindSignal)
	require.Contains(t, lanes[LaneEntry], KindPaperSim)
	require.Contains(t, lanes[LaneHolding], KindPaperSim)
	require.Contains(t, lanes[LaneExit], KindOutcome)

	bySource := map[string]SourceStatus{}
	for _, s := range tl.Sources {
		bySource[s.Source] = s
	}
	require.Equal(t, StatusOK, bySource[SourceSignal].Status)
	require.Equal(t, StatusOK, bySource[SourceWatchFollow].Status)
	require.Equal(t, StatusOK, bySource[SourcePaperSim].Status)
	require.Equal(t, StatusUnavailable, bySource[SourceExternalMirror].Status)
	require.Equal(t, StatusOK, bySource[SourceOutcome].Status)
	require.Contains(t, bySource[SourceExternalMirror].Message, "external_mirror")
}

func TestBuild_EmptyHistoryAndInvalidCode(t *testing.T) {
	setupTimelineDB(t)
	_, err := Build("  ", Options{})
	require.ErrorIs(t, err, ErrInvalidStockCode)
	_, err = Build("平安银行", Options{})
	require.ErrorIs(t, err, ErrInvalidStockCode)

	tl, err := Build("sh600519", Options{})
	require.NoError(t, err)
	require.Empty(t, tl.Events)
	require.Equal(t, Disclaimer, tl.Disclaimer)
	for _, s := range tl.Sources {
		if s.Source == SourceExternalMirror {
			require.Equal(t, StatusUnavailable, s.Status)
			continue
		}
		require.NotEqual(t, StatusOK, s.Status)
		require.NotEmpty(t, s.Message)
	}
}

func TestBuild_DBUnavailableIsEmptyNotInvented(t *testing.T) {
	original := db.Dao
	db.Dao = nil
	t.Cleanup(func() { db.Dao = original })

	tl, err := Build("sz000001", Options{})
	require.NoError(t, err)
	require.Empty(t, tl.Events)
	require.NotEmpty(t, tl.Sources)
	for _, s := range tl.Sources {
		require.Equal(t, StatusUnavailable, s.Status)
	}
}
