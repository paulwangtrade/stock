package instrument_test

import (
	"fmt"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/instrument"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTemplateQuantityMeta_ProductDefaults(t *testing.T) {
	cases := []struct {
		name    string
		secType instrument.SecurityType
		board   string
		unit    string
		lot     int64
		key     string
	}{
		{"main", instrument.SecurityEquity, instrument.BoardMAIN, instrument.QtyUnitShare, 100, "CN:EQUITY:MAIN"},
		{"star", instrument.SecurityEquity, instrument.BoardSTAR, instrument.QtyUnitShare, 200, "CN:EQUITY:STAR"},
		{"chinext", instrument.SecurityEquity, instrument.BoardCHINEXT, instrument.QtyUnitShare, 100, "CN:EQUITY:CHINEXT"},
		{"cb", instrument.SecurityConvertibleBond, instrument.BoardUNKNOWN, instrument.QtyUnitBondUnit, 10, "CN:CONVERTIBLE_BOND"},
		{"etf", instrument.SecurityETF, instrument.BoardUNKNOWN, instrument.QtyUnitETFShare, 100, "CN:ETF:DEFAULT"},
		{"unknown", instrument.SecurityUnknown, instrument.BoardUNKNOWN, instrument.QtyUnitShare, 100, "CN:UNKNOWN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := instrument.TemplateQuantityMeta(tc.secType, tc.board)
			require.Equal(t, tc.unit, m.QtyUnit)
			require.Equal(t, tc.lot, m.LotSize)
			require.Equal(t, tc.key, m.TemplateKey)
		})
	}
}

func TestClassify_MarketSegmentBoard(t *testing.T) {
	cases := []struct {
		code  string
		board string
	}{
		{"600519", instrument.BoardMAIN},
		{"000001", instrument.BoardMAIN},
		{"300750", instrument.BoardCHINEXT},
		{"688981", instrument.BoardSTAR},
		{"510300", instrument.BoardUNKNOWN}, // ETF
		{"113052", instrument.BoardUNKNOWN}, // CB
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			id, err := instrument.Classify(tc.code)
			require.NoError(t, err)
			require.Equal(t, tc.board, id.MarketSegment)
		})
	}
}

func TestResolveInstrumentMetadata_Fixtures(t *testing.T) {
	cases := []struct {
		code string
		unit string
		lot  int64
		seg  string
		sec  instrument.SecurityType
	}{
		{"600519", instrument.QtyUnitShare, 100, instrument.BoardMAIN, instrument.SecurityEquity},
		{"sh688981", instrument.QtyUnitShare, 200, instrument.BoardSTAR, instrument.SecurityEquity},
		{"113052", instrument.QtyUnitBondUnit, 10, instrument.BoardUNKNOWN, instrument.SecurityConvertibleBond},
		{"510300", instrument.QtyUnitETFShare, 100, instrument.BoardUNKNOWN, instrument.SecurityETF},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			m, err := instrument.ResolveInstrumentMetadata(tc.code, nil)
			require.NoError(t, err)
			require.Equal(t, tc.unit, m.QtyUnit)
			require.Equal(t, tc.lot, m.LotSize)
			require.Equal(t, tc.seg, m.MarketSegment)
			require.Equal(t, tc.sec, m.SecurityType)
			require.Equal(t, instrument.MetaSourceTemplate, m.Source)
			dto := m.ToDTO()
			require.Equal(t, tc.lot, dto.LotSize)
			require.Equal(t, string(tc.sec), dto.SecurityType)
			require.Equal(t, tc.seg, dto.MarketSegment)
		})
	}
}

func setupMetaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:instr_meta_%s?mode=memory&cache=shared", t.Name())
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
	require.NoError(t, instrument.EnsureQuantityMetaTable(testDB))
	return testDB
}

func TestResolveInstrumentMetadata_DBOverrideETF(t *testing.T) {
	database := setupMetaTestDB(t)
	id, err := instrument.Classify("510300")
	require.NoError(t, err)
	require.NoError(t, instrument.UpsertQuantityMetaOverride(database, &instrument.InstrumentQuantityMeta{
		StockCode:     id.Symbol,
		SecurityType:  string(instrument.SecurityETF),
		MarketSegment: instrument.BoardUNKNOWN,
		QtyUnit:       instrument.QtyUnitETFShare,
		LotSize:       200, // individual ETF override
		Notes:         "M1 test override",
	}))

	m, err := instrument.ResolveInstrumentMetadata("510300", database)
	require.NoError(t, err)
	require.Equal(t, int64(200), m.LotSize)
	require.Equal(t, instrument.MetaSourceDBOverride, m.Source)
	require.Equal(t, instrument.QtyUnitETFShare, m.QtyUnit)
}

func TestEnsureQuantityMetaTable_Idempotent(t *testing.T) {
	database := setupMetaTestDB(t)
	require.NoError(t, instrument.EnsureQuantityMetaTable(database))
	require.True(t, database.Migrator().HasTable(&instrument.InstrumentQuantityMeta{}))
	require.True(t, database.Migrator().HasColumn(&instrument.InstrumentQuantityMeta{}, "MarketSegment"))
	require.True(t, database.Migrator().HasColumn(&instrument.InstrumentQuantityMeta{}, "LotSize"))
}
