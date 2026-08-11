package portfolio_test

import (
	"fmt"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPortfolioDB(t *testing.T) {
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
		_ = sqlDB.Close()
	})
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
}

func TestService_MissingAccountDoesNotCreate(t *testing.T) {
	setupPortfolioDB(t)
	svc := portfolio.NewService()
	snap, err := svc.Snapshot(portfolio.SnapshotOptions{})
	require.NoError(t, err)
	require.False(t, snap.Found)
	require.Equal(t, 0.0, snap.Cash)

	var n int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestService_ReadsCashAndPositionsWithoutWrites(t *testing.T) {
	setupPortfolioDB(t)
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 700_000, Equity: 1_000_000, MarketValue: 300_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	pos := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 11,
	}
	require.NoError(t, db.Dao.Create(&pos).Error)

	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	require.NoError(t, err)
	require.True(t, snap.Found)
	require.Equal(t, acc.ID, snap.AccountID)
	require.InDelta(t, 700_000, snap.Cash, 1e-9)
	require.Equal(t, 1, snap.PositionCount)
	require.InDelta(t, 11_000, snap.MarketValue, 1e-9)
	require.InDelta(t, 711_000, snap.TotalEquity, 1e-9)
	require.Equal(t, snap.MarketValue, snap.TotalExposure)

	var cash float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Where("id = ?", acc.ID).Pluck("cash", &cash).Error)
	require.InDelta(t, 700_000, cash, 1e-9)
	var mark float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimPosition{}).Where("id = ?", pos.ID).Pluck("mark_price", &mark).Error)
	require.InDelta(t, 11.0, mark, 1e-9)
}
