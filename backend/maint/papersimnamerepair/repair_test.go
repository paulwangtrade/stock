package papersimnamerepair_test

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/maint/papersimnamerepair"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/stockname"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupRepairDB(t *testing.T) *gorm.DB {
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
		&papertrading.PaperSimAccount{},
		&papertrading.PaperSimPosition{},
		&papertrading.PaperSimFill{},
		&papertrading.PaperSimOrder{},
		&data.StockBasic{},
		&data.FollowedStock{},
	))
	return gdb
}

func TestRepair_DryRun_FindsEmpty(t *testing.T) {
	gdb := setupRepairDB(t)
	acc := papertrading.PaperSimAccount{Name: "paper_sim_default", Cash: 1e6, InitialCash: 1e6}
	require.NoError(t, gdb.Create(&acc).Error)
	empty := papertrading.PaperSimPosition{AccountID: acc.ID, StockCode: "sz000001", StockName: "", TotalVolume: 100}
	named := papertrading.PaperSimPosition{AccountID: acc.ID, StockCode: "sz300408", StockName: "三环集团", TotalVolume: 700}
	require.NoError(t, gdb.Create(&empty).Error)
	require.NoError(t, gdb.Create(&named).Error)

	plan := &models.TradePlan{TradeDate: "2026-07-27", GeneratedAt: time.Now(), Status: models.TradePlanStatusReady}
	require.NoError(t, gdb.Create(plan).Error)
	item := &models.TradePlanItem{PlanID: plan.ID, StockCode: "sz000001", StockName: "平安银行"}
	require.NoError(t, gdb.Create(item).Error)
	require.NoError(t, gdb.Create(&papertrading.PaperSimFill{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, StockCode: "sz000001", Volume: 100, Price: 10,
	}).Error)

	res, err := papersimnamerepair.Run(gdb, papersimnamerepair.Options{Apply: false})
	require.NoError(t, err)
	require.Equal(t, "dry-run", res.Mode)
	require.Equal(t, 1, res.EmptyScanned)
	require.Equal(t, 1, res.WouldWrite)
	require.Equal(t, 0, res.Applied)
	require.Equal(t, "sz000001", res.Rows[0].StockCode)
	require.Equal(t, "平安银行", res.Rows[0].ResolvedName)
	require.Equal(t, stockname.SourceTradePlan, res.Rows[0].Source)

	var got papertrading.PaperSimPosition
	require.NoError(t, gdb.First(&got, empty.ID).Error)
	require.Equal(t, "", got.StockName)
}

func TestRepair_Apply_OnlyEmpty_NeverOverwrite(t *testing.T) {
	gdb := setupRepairDB(t)
	acc := papertrading.PaperSimAccount{Name: "paper_sim_default", Cash: 1e6, InitialCash: 1e6}
	require.NoError(t, gdb.Create(&acc).Error)
	empty := papertrading.PaperSimPosition{AccountID: acc.ID, StockCode: "sz000001", StockName: "  ", TotalVolume: 100}
	keep := papertrading.PaperSimPosition{AccountID: acc.ID, StockCode: "sz300408", StockName: "三环集团", TotalVolume: 700}
	require.NoError(t, gdb.Create(&empty).Error)
	require.NoError(t, gdb.Create(&keep).Error)
	require.NoError(t, gdb.Create(&models.TradePlanItem{StockCode: "sz000001", StockName: "平安银行"}).Error)

	res, err := papersimnamerepair.Run(gdb, papersimnamerepair.Options{Apply: true})
	require.NoError(t, err)
	require.Equal(t, "apply", res.Mode)
	require.Equal(t, 1, res.Applied)
	require.Equal(t, 1, res.WouldWrite)

	var e papertrading.PaperSimPosition
	require.NoError(t, gdb.First(&e, empty.ID).Error)
	require.Equal(t, "平安银行", e.StockName)
	require.Equal(t, int64(100), e.TotalVolume)

	var k papertrading.PaperSimPosition
	require.NoError(t, gdb.First(&k, keep.ID).Error)
	require.Equal(t, "三环集团", k.StockName)
	require.Equal(t, int64(700), k.TotalVolume)
}
