package positionstate_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/positionstate"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLotsTestDB(t *testing.T) {
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

func seedAccountOrderFill(t *testing.T, code, tradeDate string, fillQty int64) uint {
	t.Helper()
	acc := &papertrading.PaperSimAccount{
		Name: "m0_lots", InitialCash: 1_000_000, Cash: 1_000_000, Equity: 1_000_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	ord := &papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 9001, PlanItemID: 1,
		TradeDate: tradeDate, StockCode: code, StockName: "测试",
		Side: "buy", Quantity: fillQty, Status: papertrading.OrderStatusFilled,
		FilledPrice: 10, FilledVolume: fillQty, OrderTime: time.Now(),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(ord).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: ord.ID, PlanID: 9001, PlanItemID: 1,
		StockCode: code, StockName: "测试", Side: "buy", Price: 10,
		Volume: fillQty, FillReason: papertrading.FillReasonMarketOpen, FilledAt: time.Now(),
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	require.Equal(t, fillQty, fill.Qty(), "PaperSimFill.Qty must mirror volume")
	return acc.ID
}

func TestFillQtyPhysicalColumn_IsVolume(t *testing.T) {
	require.Equal(t, "volume", positionstate.FillQtyPhysicalColumn)
	require.False(t, dbHasFillsQuantityColumn(t), "paper_sim_fills must not expose quantity as fill-size column")
}

func dbHasFillsQuantityColumn(t *testing.T) bool {
	t.Helper()
	setupLotsTestDB(t)
	return db.Dao.Migrator().HasColumn(&papertrading.PaperSimFill{}, "quantity")
}

func TestLoadLotsByAccount_UsesFillVolumeAsCanonicalQty(t *testing.T) {
	setupLotsTestDB(t)
	const code = "sz000001"
	const wantQty int64 = 700
	accID := seedAccountOrderFill(t, code, "2026-08-15", wantQty)

	buys, sells := positionstate.LoadLotsByAccount(accID)
	require.Empty(t, sells[strings.ToLower(code)])
	lots := buys[strings.ToLower(code)]
	require.Len(t, lots, 1)
	require.Equal(t, wantQty, lots[0].Quantity)
	require.Equal(t, "2026-08-15", lots[0].TradeDate)
	require.Equal(t, "BUY", lots[0].Side)
}

func TestLoadLots_WrongQuantityColumn_YieldsNoLots(t *testing.T) {
	setupLotsTestDB(t)
	const code = "sz000002"
	const wantQty int64 = 500
	accID := seedAccountOrderFill(t, code, "2026-08-14", wantQty)

	// Prove pre-M0 SELECT f.quantity cannot load fill size (column absent / empty scan).
	type row struct {
		StockCode string
		TradeDate string
		Side      string
		Quantity  int64
	}
	var bad []row
	err := db.Dao.Table("paper_sim_fills AS f").
		Select("o.stock_code AS stock_code, o.trade_date AS trade_date, o.side AS side, f.quantity AS quantity").
		Joins("JOIN paper_sim_orders o ON o.id = f.order_id").
		Where("o.account_id = ?", accID).
		Scan(&bad).Error
	// SQLite may error or return zeroed quantity; either way must not equal real fill qty.
	if err == nil {
		var sum int64
		for _, r := range bad {
			sum += r.Quantity
		}
		require.NotEqual(t, wantQty, sum, "f.quantity must not equal fill volume")
		require.Equal(t, int64(0), sum, "wrong column must not fabricate qty")
	}

	buys, _ := positionstate.LoadLotsByAccount(accID)
	require.Equal(t, wantQty, buys[strings.ToLower(code)][0].Quantity, "correct path must read f.volume")
}

func TestPaperSimFill_QtyCanonicalAlias(t *testing.T) {
	f := papertrading.PaperSimFill{Volume: 1234}
	require.Equal(t, int64(1234), f.Qty())
	f.Volume = 0
	require.Equal(t, int64(0), f.Qty())
}
