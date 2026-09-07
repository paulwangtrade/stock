package data

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSignalScanListTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:signal_scan_list_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, testDB.AutoMigrate(&models.SignalScanSnapshot{}))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestListSnapshotsOmitsResultJSON(t *testing.T) {
	setupSignalScanListTestDB(t)

	heavy := `{"items":[` + longJSONArray(200) + `],"hitTotal":200,"scannedTotal":5000,"tradeDate":"2026-07-18","session":"close"}`
	snap := models.SignalScanSnapshot{
		CreatedAt:        time.Now(),
		TradeDate:        "2099-01-02",
		Session:          "close",
		Scope:            models.SignalScanScopeAll,
		StrategyID:       "default",
		StrategyName:     "默认策略",
		SignalParamsJSON: `{"rsi":30}`,
		ScannedTotal:     5000,
		HitTotal:         200,
		Status:           "done",
		ResultJSON:       heavy,
		DurationMs:       1234,
	}
	require.NoError(t, db.Dao.Create(&snap).Error)

	api := NewSignalScanApi()
	resp := api.ListSnapshots(&models.SignalScanSnapshotQuery{
		Page:       1,
		PageSize:   50,
		TradeDate:  "2099-01-02",
		Session:    "close",
		StrategyID: "default",
	})
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Data)

	var found *models.SignalScanSnapshot
	for i := range resp.Data {
		if resp.Data[i].ID == snap.ID {
			found = &resp.Data[i]
			break
		}
	}
	require.NotNil(t, found, "created snapshot not in list")
	require.Empty(t, found.ResultJSON, "list must omit result_json")
	require.Empty(t, found.SignalParamsJSON, "list must omit signal_params_json")
	require.Equal(t, 200, found.HitTotal)
	require.Equal(t, "done", found.Status)
	require.Equal(t, "2099-01-02", found.TradeDate)

	full, err := api.GetSnapshotByID(snap.ID)
	require.NoError(t, err)
	require.NotNil(t, full)
	require.NotEmpty(t, full.ResultJSON, "detail must still return ResultJSON")
	require.Greater(t, len(full.ResultJSON), 100)
}

func TestGetLatestSnapshotMeta_OmitsResultJSON(t *testing.T) {
	setupSignalScanListTestDB(t)
	heavy := `{"items":[{"SECUCODE":"1","tag":"强"}],"hitTotal":1}`
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:        time.Now(),
		TradeDate:        "2099-02-01",
		Session:          "close",
		StrategyID:       "default",
		HitTotal:         1,
		Status:           "done",
		ResultJSON:       heavy,
		SignalParamsJSON: `{"x":1}`,
	}).Error)

	api := NewSignalScanApi()
	meta, err := api.GetLatestSnapshotMeta("2099-02-01", "close")
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, 1, meta.HitTotal)
	require.Empty(t, meta.ResultJSON)
	require.Empty(t, meta.SignalParamsJSON)

	full, err := api.GetLatestSnapshot("2099-02-01", "close")
	require.NoError(t, err)
	require.NotEmpty(t, full.ResultJSON)
}

func longJSONArray(n int) string {
	if n < 1 {
		return ""
	}
	item := `{"SECUCODE":"600000.SH","SECURITY_CODE":"600000","SECURITY_NAME_ABBR":"浦发银行","tag":"强"}`
	out := item
	for i := 1; i < n; i++ {
		out += "," + item
	}
	return out
}
