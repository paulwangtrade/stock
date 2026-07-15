package data

import (
	"fmt"
	"sync"
	"testing"

	"go-stock/backend/db"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPaperTradingTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, MigratePaperTrading(testDB))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		require.NoError(t, sqlDB.Close())
	})
}

func TestFillPaperOrderConcurrentIsIdempotent(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)
	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sh600000",
		StockName: "浦发银行",
		Side:      "buy",
		Price:     10,
		Volume:    100,
	})
	require.NoError(t, err)

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- api.FillPaperOrder(order.ID, 10)
		}()
	}
	wg.Wait()
	close(errs)
	for fillErr := range errs {
		require.NoError(t, fillErr)
	}

	var fillCount int64
	require.NoError(t, db.Dao.Model(&PaperFill{}).Where("order_id = ?", order.ID).Count(&fillCount).Error)
	require.Equal(t, int64(1), fillCount)
	var position PaperPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", account.ID, order.StockCode).First(&position).Error)
	require.Equal(t, int64(100), position.Volume)
}

func TestFillPaperOrderRollsBackAllTables(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(1_000)
	require.NoError(t, err)
	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		Side:      "buy",
		Price:     100,
		Volume:    100,
	})
	require.NoError(t, err)

	require.Error(t, api.FillPaperOrder(order.ID, 100))
	require.NoError(t, db.Dao.First(&order, order.ID).Error)
	require.Equal(t, "pending", order.Status)
	var fillCount, positionCount int64
	require.NoError(t, db.Dao.Model(&PaperFill{}).Count(&fillCount).Error)
	require.NoError(t, db.Dao.Model(&PaperPosition{}).Count(&positionCount).Error)
	require.Zero(t, fillCount)
	require.Zero(t, positionCount)
	require.NoError(t, db.Dao.First(account, account.ID).Error)
	require.Equal(t, 1_000.0, account.Cash)
}

func TestPaperValuationUsesMarksAndUpsertsDailyEquity(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&PaperPosition{
		AccountID: account.ID,
		StockCode: "sh600001",
		Volume:    10,
		AvgCost:   100,
	}).Error)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sh600002",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)
	require.Equal(t, "filled", order.Status)
	require.NoError(t, db.Dao.First(account, account.ID).Error)
	require.Equal(t, "partial", account.ValuationStatus)
	require.Equal(t, 1, account.UnpricedPositions)
	require.Equal(t, 99_995.0, account.Equity)

	require.NoError(t, api.SetMarkPrice(account.ID, "sh600001", 120))
	require.NoError(t, api.SetMarkPrice(account.ID, "sh600001", 121))
	var pointCount int64
	require.NoError(t, db.Dao.Model(&PaperEquityPoint{}).Where("account_id = ?", account.ID).Count(&pointCount).Error)
	require.Equal(t, int64(1), pointCount)
	require.NoError(t, db.Dao.First(account, account.ID).Error)
	require.Equal(t, "complete", account.ValuationStatus)
	require.Equal(t, 101_205.0, account.Equity)
}

func TestMigratePaperTradingDeduplicatesLegacyRows(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, testDB.AutoMigrate(&PaperAccount{}, &PaperPosition{}, &PaperOrder{}, &PaperFill{}, &PaperEquityPoint{}))
	require.NoError(t, testDB.Create(&[]PaperFill{{OrderID: 7}, {OrderID: 7}}).Error)
	require.NoError(t, testDB.Create(&[]PaperEquityPoint{
		{AccountID: 1, DayKey: "2026-07-14", Equity: 100},
		{AccountID: 1, DayKey: "2026-07-14", Equity: 101},
	}).Error)

	require.NoError(t, MigratePaperTrading(testDB))
	var fills, points int64
	require.NoError(t, testDB.Model(&PaperFill{}).Count(&fills).Error)
	require.NoError(t, testDB.Model(&PaperEquityPoint{}).Count(&points).Error)
	require.Equal(t, int64(1), fills)
	require.Equal(t, int64(1), points)
	require.Error(t, testDB.Create(&PaperFill{OrderID: 7}).Error)
	require.Error(t, testDB.Create(&PaperEquityPoint{AccountID: 1, DayKey: "2026-07-14"}).Error)
}
