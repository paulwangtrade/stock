package data

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newKLineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("file:kline_%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(name), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = MigrateStockKLineTables(database); err != nil {
		t.Fatal(err)
	}
	return database
}

func TestMigrateStockKLineTables(t *testing.T) {
	database := newKLineTestDB(t)
	for _, table := range []string{"stock_kline_day", "stock_kline_minute", "stock_kline_sync_state"} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
}

func TestUpsertAndQueryDayBars(t *testing.T) {
	database := newKLineTestDB(t)
	repo := NewStockKLineRepoWithDB(database)
	loc := time.Local
	d1 := time.Date(2026, 7, 14, 15, 0, 0, 0, loc)
	d2 := time.Date(2026, 7, 15, 15, 0, 0, 0, loc)

	n, err := repo.UpsertBars([]KLineBar{
		{Market: MarketCN, TSCode: "000001.SZ", Symbol: "000001", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
			BarTime: d1, Open: 10, High: 11, Low: 9.5, Close: 10.5, Volume: 1000, Source: "test"},
		{Market: MarketCN, TSCode: "000001.SZ", Symbol: "000001", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
			BarTime: d2, Open: 10.5, High: 12, Low: 10, Close: 11, Volume: 2000, Source: "test"},
	})
	if err != nil || n != 2 {
		t.Fatalf("upsert n=%d err=%v", n, err)
	}

	// 同键更新 close，不新增行
	n, err = repo.UpsertBars([]KLineBar{
		{Market: MarketCN, TSCode: "000001.SZ", Symbol: "000001", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
			BarTime: d2, Open: 10.5, High: 12, Low: 10, Close: 11.5, Volume: 2100, Source: "test2"},
	})
	if err != nil || n != 1 {
		t.Fatalf("upsert update n=%d err=%v", n, err)
	}

	// qfq 独立序列，不覆盖 none
	n, err = repo.UpsertBars([]KLineBar{
		{Market: MarketCN, TSCode: "000001.SZ", Symbol: "000001", Period: KLinePeriod1D, AdjustType: KLineAdjustQFQ,
			BarTime: d2, Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Source: "test"},
	})
	if err != nil || n != 1 {
		t.Fatalf("qfq upsert n=%d err=%v", n, err)
	}

	start := time.Date(2026, 7, 14, 0, 0, 0, 0, loc)
	end := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)
	bars, err := repo.QueryBars(KLineQuery{
		Market: MarketCN, TSCode: "000001.SZ", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
		Start: &start, End: &end, OrderAsc: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 {
		t.Fatalf("query none len=%d", len(bars))
	}
	if bars[1].Close != 11.5 {
		t.Fatalf("updated close=%v want 11.5", bars[1].Close)
	}
	// 日线 bar_time 应归一到 00:00
	if bars[0].BarTime.Hour() != 0 || bars[0].BarTime.Minute() != 0 {
		t.Fatalf("day bar_time not normalized: %v", bars[0].BarTime)
	}

	qfq, err := repo.QueryBars(KLineQuery{
		Market: MarketCN, TSCode: "000001.SZ", Period: KLinePeriod1D, AdjustType: KLineAdjustQFQ, OrderAsc: true,
	})
	if err != nil || len(qfq) != 1 || qfq[0].Close != 1 {
		t.Fatalf("qfq series=%+v err=%v", qfq, err)
	}
}

func TestUpsertAndQueryMinuteBars(t *testing.T) {
	database := newKLineTestDB(t)
	repo := NewStockKLineRepoWithDB(database)
	loc := time.Local
	t1 := time.Date(2026, 7, 16, 9, 31, 0, 0, loc)
	t2 := time.Date(2026, 7, 16, 9, 32, 0, 0, loc)

	_, err := repo.UpsertBars([]KLineBar{
		{Market: MarketCN, TSCode: "600519.SH", Symbol: "600519", Period: KLinePeriod1m, AdjustType: "",
			BarTime: t1, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 10},
		{Market: MarketCN, TSCode: "600519.SH", Symbol: "600519", Period: KLinePeriod1m,
			BarTime: t2, Open: 100.5, High: 102, Low: 100, Close: 101, Volume: 12},
	})
	if err != nil {
		t.Fatal(err)
	}

	start := t1
	bars, err := repo.QueryBars(KLineQuery{
		TSCode: "600519.SH", Period: KLinePeriod1m, Start: &start, OrderAsc: true, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 2 {
		t.Fatalf("minute bars len=%d", len(bars))
	}
	if bars[0].BarTime.Minute() != 31 {
		t.Fatalf("minute bar_time should keep clock: %v", bars[0].BarTime)
	}
}

func TestKLineSyncState(t *testing.T) {
	database := newKLineTestDB(t)
	repo := NewStockKLineRepoWithDB(database)
	last := time.Date(2026, 7, 15, 0, 0, 0, 0, time.Local)
	now := time.Now()
	if err := repo.UpsertSyncState(StockKLineSyncState{
		Market: MarketHK, TSCode: "00700.HK", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
		LastBarTime: last, LastSuccessAt: &now, Source: "test", SourceSecID: "128.00700",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSyncState(MarketHK, "00700.HK", KLinePeriod1D, KLineAdjustNone)
	if err != nil || got == nil {
		t.Fatalf("get state=%v err=%v", got, err)
	}
	if !got.LastBarTime.Equal(last) {
		t.Fatalf("last_bar_time=%v", got.LastBarTime)
	}
}
