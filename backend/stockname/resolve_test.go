package stockname_test

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/stockname"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupNameDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(
		&models.TradePlan{},
		&models.TradePlanItem{},
		&models.CandidatePool{},
		&models.CandidatePoolItem{},
		&data.StockBasic{},
		&data.FollowedStock{},
	))
	return gdb
}

func TestResolve_FromTradePlanItem(t *testing.T) {
	gdb := setupNameDB(t)
	plan := &models.TradePlan{TradeDate: "2026-08-01", GeneratedAt: time.Now(), Status: models.TradePlanStatusDraft}
	require.NoError(t, gdb.Create(plan).Error)
	require.NoError(t, gdb.Create(&models.TradePlanItem{
		PlanID: plan.ID, StockCode: "sz000001", StockName: "平安银行", Side: "buy",
	}).Error)

	res := stockname.ResolveWithDB(gdb, "sz000001", stockname.Hint{})
	require.Equal(t, "平安银行", res.Name)
	require.Equal(t, stockname.SourceTradePlan, res.Source)
	require.NoError(t, res.Err)
}

func TestResolve_FromCandidatePool(t *testing.T) {
	gdb := setupNameDB(t)
	pool := &models.CandidatePool{TradeDate: "2026-08-01", GeneratedAt: time.Now(), Status: models.CandidatePoolStatusReady}
	require.NoError(t, gdb.Create(pool).Error)
	require.NoError(t, gdb.Create(&models.CandidatePoolItem{
		PoolID: pool.ID, StockCode: "sz000002", StockName: "万科A", TradeDate: pool.TradeDate,
	}).Error)

	res := stockname.ResolveWithDB(gdb, "sz000002", stockname.Hint{PoolID: pool.ID})
	require.Equal(t, "万科A", res.Name)
	require.Equal(t, stockname.SourceCandidatePool, res.Source)
}

func TestResolve_UnknownSentinel(t *testing.T) {
	gdb := setupNameDB(t)
	res := stockname.ResolveWithDB(gdb, "sz999999", stockname.Hint{})
	require.Equal(t, stockname.UnknownName, res.Name)
	require.Equal(t, stockname.SourceUnknown, res.Source)
	require.ErrorIs(t, res.Err, stockname.ErrUnresolved)
	require.NotEmpty(t, res.Name)
}

func TestResolve_OverrideNeverEmpty(t *testing.T) {
	t.Cleanup(func() { stockname.SetResolveForTest(nil) })
	stockname.SetResolveForTest(func(symbol string, hint stockname.Hint) stockname.Result {
		return stockname.Result{Name: "", Source: ""}
	})
	res := stockname.Resolve("sz000001")
	require.Equal(t, stockname.UnknownName, res.Name)
	require.Equal(t, stockname.SourceUnknown, res.Source)
}
