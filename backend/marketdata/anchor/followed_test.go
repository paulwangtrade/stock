package anchor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFollowedTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	testDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
	require.NoError(t, testDB.AutoMigrate(&data.FollowedStock{}))
}

func seedFollowed(t *testing.T, code string, followPrice, price float64, at time.Time) {
	t.Helper()
	require.NoError(t, db.Dao.Create(&data.FollowedStock{
		StockCode:   code,
		Name:        code,
		FollowPrice: followPrice,
		Price:       price,
		Time:        at,
	}).Error)
}

func TestFollowedResolve_Hit(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sz000001", 10.5, 11.0, time.Date(2026, 7, 21, 15, 0, 0, 0, time.Local))

	r, ok := FollowedStockAnchorProvider{}.Resolve(Context{
		StockCode:  "sz000001",
		TradeDate:  "2026-07-28",
		PoolSource: "follow",
	})
	require.True(t, ok)
	require.True(t, Valid(r))
	require.InDelta(t, 10.5, r.RefPrice, 1e-9)
	require.Equal(t, RefSourcePrevClose, r.RefSource)
	require.Equal(t, "2026-07-21", r.RefAsOf)
}

func TestFollowedResolve_FollowPricePreferred(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sh600000", 9.8, 12.3, time.Time{})

	r, ok := FollowedStockAnchorProvider{}.Resolve(Context{
		StockCode: "sh600000",
		TradeDate: "2026-07-28",
	})
	require.True(t, ok)
	require.InDelta(t, 9.8, r.RefPrice, 1e-9)
	require.Equal(t, "2026-07-28", r.RefAsOf)
}

func TestFollowedResolve_PriceFallback(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sz000002", 0, 7.2, time.Time{})

	r, ok := FollowedStockAnchorProvider{}.Resolve(Context{
		StockCode: "sz000002",
		TradeDate: "2026-07-28",
	})
	require.True(t, ok)
	require.InDelta(t, 7.2, r.RefPrice, 1e-9)
}

func TestFollowedResolve_Miss(t *testing.T) {
	setupFollowedTestDB(t)

	_, ok := FollowedStockAnchorProvider{}.Resolve(Context{StockCode: "sz999999", TradeDate: "2026-07-28"})
	require.False(t, ok)

	seedFollowed(t, "sz000003", 0, 0, time.Time{})
	_, ok = FollowedStockAnchorProvider{}.Resolve(Context{StockCode: "sz000003", TradeDate: "2026-07-28"})
	require.False(t, ok)

	_, ok = FollowedStockAnchorProvider{}.Resolve(Context{StockCode: "", TradeDate: "2026-07-28"})
	require.False(t, ok)
}

func TestFollowedResolve_StrategyRunSourceLabel(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sh603986", 50, 0, time.Time{})

	r, ok := FollowedStockAnchorProvider{}.Resolve(Context{
		StockCode:  "sh603986",
		TradeDate:  "2026-07-28",
		PoolSource: PoolSourceStrategyRun,
	})
	require.True(t, ok)
	require.Equal(t, RefSourceStrategySnapshot, r.RefSource)
}

func TestFollowedResolve_NeverUsesOpenAsRefPrice(t *testing.T) {
	// Static guard: Followed adapter must not call realtime open / quote APIs.
	body, err := os.ReadFile(filepath.Join("followed.go"))
	require.NoError(t, err)
	src := string(body)
	for _, bad := range []string{
		"OpenQuote", "RealtimeOpen", "GetStockCodeRealTimeData", "info.Open", ".Open ",
		"open_ref_price", "MarkPrice",
	} {
		require.NotContains(t, src, bad, "Followed provider must not use %q as ref_price source", bad)
	}

	setupFollowedTestDB(t)
	// Zero FollowPrice/Price must miss; there is no open-price fallback path.
	seedFollowed(t, "sz000004", 0, 0, time.Time{})
	_, ok := FollowedStockAnchorProvider{}.Resolve(Context{StockCode: "sz000004", TradeDate: "2026-07-28"})
	require.False(t, ok)
}

func TestChain_OnlyFollowed(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sz000001", 10, 0, time.Time{})

	r1, ok1 := FollowedStockAnchorProvider{}.Resolve(Context{StockCode: "sz000001", TradeDate: "2026-07-28"})
	r2, ok2 := DefaultProvider().Resolve(Context{StockCode: "sz000001", TradeDate: "2026-07-28"})
	require.True(t, ok1)
	require.True(t, ok2)
	require.Equal(t, r1.RefPrice, r2.RefPrice)
	require.Equal(t, r1.RefSource, r2.RefSource)
}

func TestValid_RequiresPositivePrice(t *testing.T) {
	require.False(t, Valid(Result{RefPrice: 0, RefSource: RefSourcePrevClose}))
	require.False(t, Valid(Result{RefPrice: 1, RefSource: ""}))
	require.True(t, Valid(Result{RefPrice: 1, RefSource: RefSourcePrevClose}))
}

func TestFollowedResolve_SourceDateDoesNotOverrideAsOf(t *testing.T) {
	setupFollowedTestDB(t)
	seedFollowed(t, "sz000001", 10, 0, time.Date(2026, 7, 21, 0, 0, 0, 0, time.Local))

	r, ok := FollowedStockAnchorProvider{}.Resolve(Context{
		StockCode:  "sz000001",
		TradeDate:  "2026-07-28",
		SourceDate: "2026-07-27",
	})
	require.True(t, ok)
	require.Equal(t, "2026-07-21", r.RefAsOf)
	require.False(t, strings.Contains(r.RefSource, "open"))
}
