package stocknamerepair_test

import (
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/maint/stocknamerepair"
	"go-stock/backend/models"

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
		&models.CandidatePool{},
		&models.CandidatePoolItem{},
		&data.StockBasic{},
		&data.FollowedStock{},
	))
	return gdb
}

func TestRepair_DryRun_P3FromNewerPlan(t *testing.T) {
	gdb := setupRepairDB(t)
	now := time.Now()

	old := &models.TradePlan{TradeDate: "2026-07-01", GeneratedAt: now, Status: models.TradePlanStatusDraft, PlanVersion: 1, PoolID: 1}
	require.NoError(t, gdb.Create(old).Error)
	empty := &models.TradePlanItem{
		PlanID: old.ID, TradeDate: old.TradeDate, StockCode: "sz000001", StockName: "",
		Side: "buy", Status: models.TradePlanItemPending, TargetAmount: 100000, LimitPrice: 10, TargetVolume: 1000,
	}
	require.NoError(t, gdb.Create(empty).Error)

	newer := &models.TradePlan{TradeDate: "2026-08-01", GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1, PoolID: 2}
	require.NoError(t, gdb.Create(newer).Error)
	require.NoError(t, gdb.Create(&models.TradePlanItem{
		PlanID: newer.ID, TradeDate: newer.TradeDate, StockCode: "sz000001", StockName: "平安银行",
		Side: "buy", Status: models.TradePlanItemPending,
	}).Error)

	res, err := stocknamerepair.Run(gdb, stocknamerepair.Options{Apply: false})
	require.NoError(t, err)
	require.Equal(t, "dry-run", res.Mode)
	require.Equal(t, 1, res.WouldWrite)
	require.Equal(t, 0, res.Applied)
	require.Len(t, res.Rows, 1)
	require.Equal(t, empty.ID, res.Rows[0].ItemID)
	require.Equal(t, "平安银行", res.Rows[0].NewName)
	require.Equal(t, stocknamerepair.SourceP3OtherLocal, res.Rows[0].Source)
	require.Equal(t, "(empty)", func() string {
		if strings.TrimSpace(res.Rows[0].OldName) == "" {
			return "(empty)"
		}
		return res.Rows[0].OldName
	}())

	// DB unchanged
	var got models.TradePlanItem
	require.NoError(t, gdb.First(&got, empty.ID).Error)
	require.Equal(t, "", got.StockName)
	require.Equal(t, float64(10), got.LimitPrice)
	require.Equal(t, int64(1000), got.TargetVolume)
}

func TestRepair_Apply_OnlyEmpty_NeverOverwrite(t *testing.T) {
	gdb := setupRepairDB(t)
	now := time.Now()

	plan := &models.TradePlan{TradeDate: "2026-07-02", GeneratedAt: now, Status: models.TradePlanStatusDraft, PlanVersion: 1, PoolID: 9}
	require.NoError(t, gdb.Create(plan).Error)
	empty := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: plan.TradeDate, StockCode: "sz000001", StockName: "  ",
		Side: "buy", Status: models.TradePlanItemPending, LimitPrice: 12.3, TargetVolume: 500,
	}
	filled := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: plan.TradeDate, StockCode: "sh600519", StockName: "贵州茅台",
		Side: "buy", Status: models.TradePlanItemPending, LimitPrice: 1800, TargetVolume: 100,
	}
	require.NoError(t, gdb.Create(empty).Error)
	require.NoError(t, gdb.Create(filled).Error)

	require.NoError(t, gdb.Create(&data.StockBasic{Symbol: "000001", Name: "平安银行"}).Error)

	res, err := stocknamerepair.Run(gdb, stocknamerepair.Options{Apply: true})
	require.NoError(t, err)
	require.Equal(t, "apply", res.Mode)
	require.Equal(t, 1, res.Applied)
	require.Equal(t, stocknamerepair.SourceP4Basic, res.Rows[0].Source)

	var e models.TradePlanItem
	require.NoError(t, gdb.First(&e, empty.ID).Error)
	require.Equal(t, "平安银行", e.StockName)
	require.Equal(t, 12.3, e.LimitPrice)
	require.Equal(t, int64(500), e.TargetVolume)
	require.Equal(t, models.TradePlanItemPending, e.Status)

	var f models.TradePlanItem
	require.NoError(t, gdb.First(&f, filled.ID).Error)
	require.Equal(t, "贵州茅台", f.StockName) // never overwritten
}

func TestRepair_P2_CandidatePool(t *testing.T) {
	gdb := setupRepairDB(t)
	now := time.Now()
	pool := &models.CandidatePool{TradeDate: "2026-07-03", GeneratedAt: now, Status: models.CandidatePoolStatusReady}
	require.NoError(t, gdb.Create(pool).Error)
	require.NoError(t, gdb.Create(&models.CandidatePoolItem{
		PoolID: pool.ID, TradeDate: pool.TradeDate, StockCode: "sz000002", StockName: "万科A", Rank: 1,
	}).Error)

	plan := &models.TradePlan{TradeDate: pool.TradeDate, GeneratedAt: now, Status: models.TradePlanStatusDraft, PoolID: pool.ID}
	require.NoError(t, gdb.Create(plan).Error)
	item := &models.TradePlanItem{PlanID: plan.ID, TradeDate: plan.TradeDate, StockCode: "sz000002", StockName: ""}
	require.NoError(t, gdb.Create(item).Error)

	res, err := stocknamerepair.Run(gdb, stocknamerepair.Options{Apply: true})
	require.NoError(t, err)
	require.Equal(t, 1, res.Applied)
	require.Equal(t, stocknamerepair.SourceP2Pool, res.Rows[0].Source)

	var got models.TradePlanItem
	require.NoError(t, gdb.First(&got, item.ID).Error)
	require.Equal(t, "万科A", got.StockName)
}

func TestRepair_CASMiss_WhenNameFilledConcurrently(t *testing.T) {
	gdb := setupRepairDB(t)
	now := time.Now()
	plan := &models.TradePlan{TradeDate: "2026-07-04", GeneratedAt: now, Status: models.TradePlanStatusDraft}
	require.NoError(t, gdb.Create(plan).Error)
	item := &models.TradePlanItem{PlanID: plan.ID, StockCode: "sz000001", StockName: ""}
	require.NoError(t, gdb.Create(item).Error)
	require.NoError(t, gdb.Create(&data.StockBasic{Symbol: "000001", Name: "平安银行"}).Error)

	// Pretend concurrent fill before apply path: we simulate by applying twice.
	res1, err := stocknamerepair.Run(gdb, stocknamerepair.Options{Apply: true})
	require.NoError(t, err)
	require.Equal(t, 1, res1.Applied)

	res2, err := stocknamerepair.Run(gdb, stocknamerepair.Options{Apply: true})
	require.NoError(t, err)
	require.Equal(t, 0, res2.EmptyScanned) // no longer empty
}

func TestFormatTable_ContainsColumns(t *testing.T) {
	s := stocknamerepair.FormatTable([]stocknamerepair.Row{{
		PlanID: 1, ItemID: 2, StockCode: "sz000001", OldName: "", NewName: "平安银行", Source: stocknamerepair.SourceP4Basic,
	}})
	require.Contains(t, s, "plan_id")
	require.Contains(t, s, "item_id")
	require.Contains(t, s, "stock_code")
	require.Contains(t, s, "old_name")
	require.Contains(t, s, "new_name")
	require.Contains(t, s, "source")
	require.Contains(t, s, "平安银行")
}
