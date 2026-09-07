package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestExitEvaluationAPI_BetaDB_SMK001 verifies lazy migrate on a real Beta DB copy.
// Run: BETA_DB_PATH=D:\stock\build\bin\data\stock.db go test ./backend/api -run TestExitEvaluationAPI_BetaDB_SMK001 -v -count=1
func TestExitEvaluationAPI_BetaDB_SMK001(t *testing.T) {
	src := os.Getenv("BETA_DB_PATH")
	if src == "" {
		t.Skip("BETA_DB_PATH not set")
	}
	srcInfo, err := os.Stat(src)
	require.NoError(t, err)
	if srcInfo.IsDir() {
		t.Fatalf("BETA_DB_PATH must be a file: %s", src)
	}

	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "stock.db")
	copyFile(t, src, dst)

	original := db.Dao
	testDB, err := gorm.Open(sqlite.Open(dst), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})

	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)

	if testDB.Migrator().HasTable(&papertrading.ExitReviewOutcome{}) {
		require.NoError(t, testDB.Migrator().DropTable(&papertrading.ExitReviewOutcome{}))
	}
	require.False(t, testDB.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))

	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/holdings/exit-evaluation", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	require.NotNil(t, body["exit_evaluation"])
	require.True(t, testDB.Migrator().HasTable(&papertrading.ExitReviewOutcome{}))
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.Create(dst)
	require.NoError(t, err)
	defer out.Close()
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
}
