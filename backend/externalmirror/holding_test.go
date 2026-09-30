package externalmirror

import (
	"fmt"
	"math"
	"testing"
	"time"

	"go-stock/backend/db"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMirrorDB(t *testing.T) {
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
		SetNowForTest(nil)
		SetNameResolverForTest(nil)
	})
}

func f64(v float64) *float64 { return &v }

func TestCreateRejectsIncompleteAndForeignSource(t *testing.T) {
	setupMirrorDB(t)
	SetNowForTest(func() time.Time { return time.Date(2026, 9, 30, 9, 0, 0, 0, time.Local) })
	SetNameResolverForTest(func(string) string { return "" })

	_, err := Create(Input{Quantity: f64(100), CostPrice: f64(10)})
	require.Error(t, err)
	require.Equal(t, "invalid_code", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "不是代码", Quantity: f64(100), CostPrice: f64(10)})
	require.Equal(t, "invalid_code", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", CostPrice: f64(10)})
	require.Equal(t, "invalid_quantity", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(0), CostPrice: f64(10)})
	require.Equal(t, "invalid_quantity", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(100.5), CostPrice: f64(10)})
	require.Equal(t, "invalid_quantity", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(100)})
	require.Equal(t, "invalid_cost", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(100), CostPrice: f64(0)})
	require.Equal(t, "invalid_cost", asValidation(err).Reason)

	nan := math.NaN()
	_, err = Create(Input{StockCode: "600519", Quantity: f64(100), CostPrice: &nan})
	require.Equal(t, "invalid_cost", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(100), CostPrice: f64(10), EntryDate: "2026-13-01"})
	require.Equal(t, "invalid_entry_date", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(100), CostPrice: f64(10), Source: "paper_sim"})
	require.Equal(t, "invalid_source", asValidation(err).Reason)

	yes := true
	_, err = Create(Input{StockCode: "600519", Quantity: f64(100), CostPrice: f64(10), FeedsTradePlan: &yes})
	require.Equal(t, "observation_only", asValidation(err).Reason)

	_, err = Create(Input{StockCode: "600519", Quantity: f64(100), CostPrice: f64(10), Tradable: &yes})
	require.Equal(t, "observation_only", asValidation(err).Reason)

	var n int64
	require.NoError(t, db.Dao.Model(&Holding{}).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestCreateListUpdateDeleteRoundTrip(t *testing.T) {
	setupMirrorDB(t)
	SetNowForTest(func() time.Time { return time.Date(2026, 9, 30, 9, 0, 0, 0, time.Local) })
	SetNameResolverForTest(func(code string) string {
		if code == "sh600519" {
			return "贵州茅台"
		}
		return ""
	})

	view, err := Create(Input{
		StockCode: "600519",
		Quantity:  f64(200),
		CostPrice: f64(1400.5),
		Note:      "券商App手工对照",
	})
	require.NoError(t, err)
	require.Equal(t, Source, view.Source)
	require.Equal(t, "sh600519", view.StockCode)
	require.Equal(t, "贵州茅台", view.StockName)
	require.Equal(t, int64(200), view.Quantity)
	require.InDelta(t, 1400.5, view.CostPrice, 1e-9)
	require.Equal(t, "2026-09-30", view.EntryDate)
	require.False(t, view.FeedsTradePlan)
	require.False(t, view.Tradable)

	_, err = Create(Input{StockCode: "SH600519", Quantity: f64(1), CostPrice: f64(1)})
	require.Equal(t, "duplicate_code", asValidation(err).Reason)

	updated, err := Update(view.ID, Input{
		Quantity:  f64(180),
		CostPrice: f64(1410),
		EntryDate: "2026-09-01",
		Note:      "调整",
		NoteSet:   true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(180), updated.Quantity)
	require.InDelta(t, 1410, updated.CostPrice, 1e-9)
	require.Equal(t, "2026-09-01", updated.EntryDate)
	require.Equal(t, "贵州茅台", updated.StockName)
	require.Equal(t, "调整", updated.Note)
	require.Equal(t, "sh600519", updated.StockCode)

	rows, err := List()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, Source, rows[0].Source)

	require.NoError(t, Delete(view.ID))
	rows, err = List()
	require.NoError(t, err)
	require.Empty(t, rows)

	err = Delete(view.ID)
	require.Equal(t, "not_found", asValidation(err).Reason)
}

func TestListHidesNonMirrorSource(t *testing.T) {
	setupMirrorDB(t)
	SetNameResolverForTest(func(string) string { return "" })
	require.NoError(t, EnsureSchema(db.Dao))
	rogue := Holding{
		Source: "paper_sim", StockCode: "sz000001", Quantity: 100, CostPrice: 10, EntryDate: "2026-09-30",
	}
	require.NoError(t, db.Dao.Create(&rogue).Error)
	rows, err := List()
	require.NoError(t, err)
	require.Empty(t, rows)
}
