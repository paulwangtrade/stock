package healthcheck_test

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/healthcheck"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupHealthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open("file:healthcheck_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	orig := db.Dao
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = orig })
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, testDB.AutoMigrate(&models.SignalScanSnapshot{}))
	require.NoError(t, papertrading.EnsureSchema(testDB))
	require.NoError(t, db.MarkSchemaVersion(healthcheck.RequiredSchemaVersion))
	return testDB
}

func TestHealthCheck_EmptyDB_TablesExist(t *testing.T) {
	gdb := setupHealthTestDB(t)
	res := healthcheck.Run(gdb)
	require.False(t, res.Failed(), res.Summary())
	require.True(t, res.Schema.OK)
	for _, tbl := range res.Tables {
		if tbl.Key == healthcheck.TablePositionStates {
			continue
		}
		require.True(t, tbl.Exists, "table %s", tbl.Key)
	}
}

func TestHealthCheck_FillMissingPlanID_Fails(t *testing.T) {
	gdb := setupHealthTestDB(t)
	acc := &papertrading.PaperSimAccount{Name: "t", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, gdb.Create(acc).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: acc.ID,
		StockCode: "sz000001",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		FilledAt:  time.Now(),
	}
	require.NoError(t, gdb.Create(fill).Error)

	res := healthcheck.Run(gdb)
	var fillCheck healthcheck.Check
	for _, c := range res.Checks {
		if c.Name == "fill_has_plan_id" {
			fillCheck = c
			break
		}
	}
	require.False(t, fillCheck.OK)
	require.True(t, res.Failed())
}

func TestHealthCheck_PositionMissingCode_Fails(t *testing.T) {
	gdb := setupHealthTestDB(t)
	acc := &papertrading.PaperSimAccount{Name: "t", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, gdb.Create(acc).Error)
	pos := &papertrading.PaperSimPosition{
		AccountID:       acc.ID,
		StockCode:       "",
		StockName:       "x",
		TotalVolume:     100,
		AvailableVolume: 100,
		AvgCost:         10,
		MarkPrice:       10,
	}
	require.NoError(t, gdb.Create(pos).Error)

	res := healthcheck.Run(gdb)
	var posCheck healthcheck.Check
	for _, c := range res.Checks {
		if c.Name == "position_has_stock_code" {
			posCheck = c
			break
		}
	}
	require.False(t, posCheck.OK)
}

func TestFormatText_NonEmpty(t *testing.T) {
	gdb := setupHealthTestDB(t)
	res := healthcheck.Run(gdb)
	text := healthcheck.FormatText(res)
	require.Contains(t, text, "DB Health Check")
	require.Contains(t, text, "schema")
}
