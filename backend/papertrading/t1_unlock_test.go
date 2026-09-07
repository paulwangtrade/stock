package papertrading_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/tradingcalendar"

	"github.com/stretchr/testify/require"
)

func fridayUnlockNow() time.Time {
	return time.Date(2026, 8, 14, 9, 0, 0, 0, time.Local)
}

func saturdayUnlockNow() time.Time {
	return time.Date(2026, 8, 15, 9, 0, 0, 0, time.Local)
}

func seedUnlockAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000, Equity: 1_000_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedLockedPosition(t *testing.T, accID uint, code string, total int64) *papertrading.PaperSimPosition {
	t.Helper()
	pos := &papertrading.PaperSimPosition{
		AccountID: accID, StockCode: code, StockName: "测试",
		TotalVolume: total, AvailableVolume: 0, LockedVolume: total,
		AvgCost: 10, MarkPrice: 10, UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(pos).Error)
	return pos
}

func seedBuyFill(t *testing.T, accID uint, code, tradeDate string, volume int64) {
	t.Helper()
	ord := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: 1, PlanItemID: uint(time.Now().UnixNano()%1_000_000_000 + 1),
		TradeDate: tradeDate, StockCode: code, StockName: "测试",
		Side: "buy", Quantity: volume, Status: papertrading.OrderStatusFilled,
		FilledPrice: 10, FilledVolume: volume, OrderTime: time.Now(),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(ord).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: ord.ID, PlanID: 1, StockCode: code, StockName: "测试",
		Side: "buy", Price: 10, Volume: volume, FillReason: papertrading.FillReasonMarketOpen,
		FilledAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(fill).Error)
}

func reloadPos(t *testing.T, id uint) papertrading.PaperSimPosition {
	t.Helper()
	var p papertrading.PaperSimPosition
	require.NoError(t, db.Dao.First(&p, id).Error)
	return p
}

func TestPositionUnlockJob_YesterdayFill_UnlocksAll(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedUnlockAccount(t)
	pos := seedLockedPosition(t, acc.ID, "sz000001", 100)
	seedBuyFill(t, acc.ID, "sz000001", "2026-08-13", 100)

	require.True(t, tradingcalendar.IsTradingDay(fridayUnlockNow()))
	res, err := papertrading.PositionUnlockJob(fridayUnlockNow())
	require.NoError(t, err)
	require.False(t, res.Skipped)
	require.Equal(t, int64(100), res.UnlockVolumeTotal)

	got := reloadPos(t, pos.ID)
	require.Equal(t, int64(0), got.LockedVolume)
	require.Equal(t, int64(100), got.AvailableVolume)
	require.Equal(t, int64(100), got.TotalVolume)
}

func TestPositionUnlockJob_TodayFill_StaysLocked(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedUnlockAccount(t)
	pos := seedLockedPosition(t, acc.ID, "sz000001", 100)
	seedBuyFill(t, acc.ID, "sz000001", "2026-08-14", 100)

	res, err := papertrading.PositionUnlockJob(fridayUnlockNow())
	require.NoError(t, err)
	require.False(t, res.Skipped)
	require.Equal(t, int64(0), res.UnlockVolumeTotal)
	require.Equal(t, 0, res.PositionsUnlocked)

	got := reloadPos(t, pos.ID)
	require.Equal(t, int64(100), got.LockedVolume)
	require.Equal(t, int64(0), got.AvailableVolume)
}

func TestPositionUnlockJob_MixedLots_UnlocksOnlyPriorDay(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedUnlockAccount(t)
	pos := seedLockedPosition(t, acc.ID, "sz000001", 150)
	seedBuyFill(t, acc.ID, "sz000001", "2026-08-13", 100)
	seedBuyFill(t, acc.ID, "sz000001", "2026-08-14", 50)

	res, err := papertrading.PositionUnlockJob(fridayUnlockNow())
	require.NoError(t, err)
	require.Equal(t, int64(100), res.UnlockVolumeTotal)

	got := reloadPos(t, pos.ID)
	require.Equal(t, int64(50), got.LockedVolume)
	require.Equal(t, int64(100), got.AvailableVolume)
	require.Equal(t, int64(150), got.TotalVolume)
}

func TestPositionUnlockJob_NonTradingDay_NoWrite(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedUnlockAccount(t)
	pos := seedLockedPosition(t, acc.ID, "sz000001", 100)
	seedBuyFill(t, acc.ID, "sz000001", "2026-08-13", 100)

	require.False(t, tradingcalendar.IsTradingDay(saturdayUnlockNow()))
	res, err := papertrading.PositionUnlockJob(saturdayUnlockNow())
	require.NoError(t, err)
	require.True(t, res.Skipped)
	require.Equal(t, "non trading day", res.Message)

	got := reloadPos(t, pos.ID)
	require.Equal(t, int64(100), got.LockedVolume)
	require.Equal(t, int64(0), got.AvailableVolume)
}

func TestPositionUnlockJob_DoesNotCallSettleNewTradingDay(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "t1_unlock.go"))
	require.NoError(t, err)
	src := string(body)
	require.NotContains(t, src, "SettleNewTradingDay(")
	require.NotContains(t, src, "IsWeekdayLocal")
	require.Contains(t, src, "tradingcalendar.IsTradingDay")
	require.Contains(t, src, "tradingcalendar.FormatDate")
}

func TestPositionUnlockJob_IdempotentAndDoesNotTouchFills(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedUnlockAccount(t)
	pos := seedLockedPosition(t, acc.ID, "sz000001", 100)
	seedBuyFill(t, acc.ID, "sz000001", "2026-08-13", 100)

	var fillsBefore, ordersBefore int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fillsBefore).Error)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&ordersBefore).Error)

	_, err := papertrading.PositionUnlockJob(fridayUnlockNow())
	require.NoError(t, err)
	res2, err := papertrading.PositionUnlockJob(fridayUnlockNow())
	require.NoError(t, err)
	require.Equal(t, int64(0), res2.UnlockVolumeTotal)

	got := reloadPos(t, pos.ID)
	require.Equal(t, int64(0), got.LockedVolume)
	require.Equal(t, int64(100), got.AvailableVolume)

	var fillsAfter, ordersAfter int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimFill{}).Count(&fillsAfter).Error)
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimOrder{}).Count(&ordersAfter).Error)
	require.Equal(t, fillsBefore, fillsAfter)
	require.Equal(t, ordersBefore, ordersAfter)
}

func TestPhase10C6C_CronWiring_Markers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	cronBody, err := os.ReadFile(filepath.Join(root, "app_paper_trading.go"))
	require.NoError(t, err)
	src := string(cronBody)
	require.Contains(t, src, "paper_trading_t1_unlock")
	require.Contains(t, src, "0 20 9 * * 1-5")
	require.Contains(t, src, "runPaperTradingT1UnlockJob")
	require.Contains(t, src, "RunMorningSettlement")
	require.False(t, strings.Contains(src, "SettleNewTradingDay("))

	for _, startupFile := range []string{"app_windows.go", "app_linux.go", "app_darwin.go"} {
		body, err := os.ReadFile(filepath.Join(root, startupFile))
		require.NoError(t, err, startupFile)
		require.Contains(t, string(body), "InitPaperTradingJobs()", startupFile+" OnStartup must register paper trading jobs")
	}

	morningBody, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "portfolio", "positionstate", "morning.go"))
	require.NoError(t, err)
	morningSrc := string(morningBody)
	require.Contains(t, morningSrc, "papertrading.PositionUnlockJob(")
	require.False(t, strings.Contains(morningSrc, "SettleNewTradingDay("))
}
