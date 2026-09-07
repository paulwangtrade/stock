package healthcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenOptions configures read-only database open for CLI probes.
type OpenOptions struct {
	DBPath              string
	ReadOnly            bool
	RequiredSchemaVersion int
}

// OpenReadOnly opens a SQLite database for health checks (default mode=ro).
func OpenReadOnly(opts OpenOptions) (*gorm.DB, string, error) {
	path := strings.TrimSpace(opts.DBPath)
	if path == "" {
		path = filepath.Join("build", "bin", "data", "stock.db")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve db path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, "", fmt.Errorf("db not found: %s (%w)", abs, err)
	}
	dsn := abs + "?_busy_timeout=10000"
	if opts.ReadOnly {
		dsn += "&mode=ro"
	}
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("open db: %w", err)
	}
	return gdb, abs, nil
}

// RunFile opens db at path and runs health checks (read-only).
func RunFile(opts OpenOptions) (Result, error) {
	if opts.RequiredSchemaVersion <= 0 {
		opts.RequiredSchemaVersion = RequiredSchemaVersion
	}
	opts.ReadOnly = true
	gdb, abs, err := OpenReadOnly(opts)
	if err != nil {
		return Result{}, err
	}
	sqlDB, err := gdb.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	res := run(gdb, opts.RequiredSchemaVersion)
	res.Messages = append([]string{fmt.Sprintf("db=%s", abs)}, res.Messages...)
	return res, nil
}
