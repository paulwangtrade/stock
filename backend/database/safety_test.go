package database_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/database"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTempDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=5000"), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return gdb
}

func TestBackup_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "stock.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("sqlite-placeholder"), 0o644))

	fixed := time.Date(2026, 8, 9, 11, 31, 0, 0, time.Local)
	svc := &database.DatabaseSafetyService{Now: func() time.Time { return fixed }}
	bak, err := svc.Backup(dbPath)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "stock.db.backup.20260809113100"), bak)
	got, err := os.ReadFile(bak)
	require.NoError(t, err)
	require.Equal(t, []byte("sqlite-placeholder"), got)
}

func TestIntegrityCheck_Pass(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ok.db")
	gdb := openTempDB(t, dbPath)
	require.NoError(t, gdb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY)").Error)
	require.NoError(t, gdb.Exec("INSERT INTO t(id) VALUES (1)").Error)

	svc := database.Default()
	require.NoError(t, svc.IntegrityCheck(gdb))

	res := svc.RunAtStartup(dbPath, gdb)
	require.True(t, res.OK)
	require.True(t, res.TradingAllowed)
	require.Equal(t, "ok", res.Integrity)
	require.NotEmpty(t, res.BackupPath)
	_, err := os.Stat(res.BackupPath)
	require.NoError(t, err)
}

func TestIntegrityCheck_Fail(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bad.db")
	gdb := openTempDB(t, dbPath)
	require.NoError(t, gdb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)").Error)
	require.NoError(t, gdb.Exec("INSERT INTO t(id,name) VALUES (1,'x')").Error)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	// Corrupt page payload while keeping a recognizable SQLite header so open may succeed
	// but integrity_check fails.
	raw, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	require.Greater(t, len(raw), 200)
	for i := 100; i < 180 && i < len(raw); i++ {
		raw[i] ^= 0xFF
	}
	require.NoError(t, os.WriteFile(dbPath, raw, 0o644))

	gdb2, err := gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=5000&mode=rw"), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		// Some corruptions fail at open — still a fail path for startup safety.
		require.Error(t, err)
		return
	}
	sqlDB2, err := gdb2.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB2.Close() })

	svc := database.Default()
	err = svc.IntegrityCheck(gdb2)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "integrity_check")

	res := svc.RunAtStartup(dbPath, gdb2)
	require.False(t, res.OK)
	require.False(t, res.TradingAllowed)
	require.NotEmpty(t, res.Error)
	require.Contains(t, res.RecoveryHint, "trading initialization is blocked")
	require.NotEmpty(t, res.BackupPath) // forensic copy still attempted
}

func TestIsTradingSafe_UsesLastResult(t *testing.T) {
	t.Cleanup(database.ResetLastResultForTest)
	database.SetLastResult(database.SafetyResult{OK: false, TradingAllowed: false, Error: "boom"})
	require.False(t, database.IsTradingSafe())
	database.ResetLastResultForTest()
	require.True(t, database.IsTradingSafe())
}

func TestStripDSNPath(t *testing.T) {
	require.Equal(t, "data/stock.db", database.StripDSNPath("data/stock.db?_busy_timeout=1"))
	require.Equal(t, "data/stock.db", database.StripDSNPath("data/stock.db"))
}
