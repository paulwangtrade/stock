package externalmirror

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"

	"github.com/stretchr/testify/require"
)

// Mirror rows must not become paper_sim inventory or TradePlan input,
// even when the same code already exists on the sim book.
func TestMirrorDoesNotFeedPaperSimOrTradePlan(t *testing.T) {
	setupMirrorDB(t)
	SetNowForTest(func() time.Time { return time.Date(2026, 9, 30, 15, 0, 0, 0, time.Local) })
	SetNameResolverForTest(func(string) string { return "茅台" })
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, db.Dao.AutoMigrate(&data.FollowedStock{}))

	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 100_000, Cash: 80_000, Equity: 100_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	sim := papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600519", StockName: "模拟茅台",
		TotalVolume: 100, AvailableVolume: 100, AvgCost: 1500, MarkPrice: 1510,
	}
	require.NoError(t, db.Dao.Create(&sim).Error)
	require.NoError(t, db.Dao.Create(&data.FollowedStock{
		StockCode: "sh600519", Name: "自选茅台", Volume: 50, CostPrice: 1490,
	}).Error)

	var plansBefore int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&plansBefore).Error)
	var itemsBefore int64
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Count(&itemsBefore).Error)

	view, err := Create(Input{
		StockCode: "600519",
		Quantity:  f64(500),
		CostPrice: f64(1488),
		Note:      "实盘对照",
	})
	require.NoError(t, err)
	require.Equal(t, int64(500), view.Quantity)
	require.Equal(t, Source, view.Source)

	var simRows []papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Find(&simRows).Error)
	require.Len(t, simRows, 1)
	require.Equal(t, int64(100), simRows[0].TotalVolume)
	require.InDelta(t, 1500, simRows[0].AvgCost, 1e-9)

	loaded, err := papertrading.GetPositions(acc.ID)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, int64(100), loaded[0].TotalVolume)

	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	require.NoError(t, err)
	require.True(t, snap.Found)
	require.Equal(t, 1, snap.PositionCount)
	require.Equal(t, "sh600519", snap.Positions[0].StockCode)
	require.Equal(t, int64(100), snap.Positions[0].Volume)
	require.NotEqual(t, int64(500), snap.Positions[0].Volume)
	require.NotEqual(t, int64(600), snap.Positions[0].Volume)

	var plansAfter int64
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Count(&plansAfter).Error)
	require.Equal(t, plansBefore, plansAfter)
	var itemsAfter int64
	require.NoError(t, db.Dao.Model(&models.TradePlanItem{}).Count(&itemsAfter).Error)
	require.Equal(t, itemsBefore, itemsAfter)

	var followed data.FollowedStock
	require.NoError(t, db.Dao.Where("stock_code = ?", "sh600519").First(&followed).Error)
	require.Equal(t, int64(50), followed.Volume)
	require.InDelta(t, 1490, followed.CostPrice, 1e-9)

	var mirrorN int64
	require.NoError(t, db.Dao.Model(&Holding{}).Where("source = ?", Source).Count(&mirrorN).Error)
	require.Equal(t, int64(1), mirrorN)

	_, err = Update(view.ID, Input{Quantity: f64(480), CostPrice: f64(1490)})
	require.NoError(t, err)
	require.NoError(t, db.Dao.Find(&simRows).Error)
	require.Len(t, simRows, 1)
	require.Equal(t, int64(100), simRows[0].TotalVolume)

	require.NoError(t, Delete(view.ID))
	require.NoError(t, db.Dao.Find(&simRows).Error)
	require.Len(t, simRows, 1)
	require.Equal(t, int64(100), simRows[0].TotalVolume)
}
