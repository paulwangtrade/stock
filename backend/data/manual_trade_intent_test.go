package data

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupManualTradeIntentTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:manual_intent_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, EnsureManualTradeIntentTables())
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestCreateManualTradeIntent_DefaultFields(t *testing.T) {
	setupManualTradeIntentTestDB(t)
	repo := NewManualTradeIntentRepo()

	intent := &models.ManualTradeIntent{
		Symbol:    "sh600036",
		StockName: "招商银行",
		Price:     40.1,
		Volume:    100,
		AccountID: 1,
		Reason:    "模拟执行台",
		Status:    models.ManualTradeIntentStatusConfirmed, // 应被强制为 draft
	}
	require.NoError(t, repo.CreateManualTradeIntent(intent))
	require.NotZero(t, intent.ID)

	got, err := repo.GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusDraft, got.Status)
	require.Equal(t, "sh600036", got.Symbol)
	require.Equal(t, "招商银行", got.StockName)
	require.Equal(t, "buy", got.Side)
	require.Equal(t, models.ManualSourcePaperTradingPanel, got.ManualSource)
	require.Equal(t, models.ManualTradeOrderKindNormal, got.OrderKind)
	require.Equal(t, models.ManualTradeStrategyTag, "manual")
	require.Empty(t, got.ClientOrderID)
	require.Zero(t, got.OrderID)
	require.Empty(t, got.ErrorCode)
	require.Empty(t, got.ErrorMessage)
}

func TestCreateManualTradeIntent_AlwaysDraft(t *testing.T) {
	setupManualTradeIntentTestDB(t)
	repo := NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{
		Symbol: "sz000001",
		Side:   "sell",
		Price:  11,
		Volume: 200,
		Status: models.ManualTradeIntentStatusSubmitted,
	}
	require.NoError(t, repo.CreateManualTradeIntent(intent))
	got, err := repo.GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusDraft, got.Status)
	require.Equal(t, "sell", got.Side)
}

func TestManualTradeIntent_DraftToConfirmed(t *testing.T) {
	setupManualTradeIntentTestDB(t)
	repo := NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{Symbol: "sh600000", Price: 10, Volume: 100}
	require.NoError(t, repo.CreateManualTradeIntent(intent))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusConfirmed, nil))
	got, err := repo.GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusConfirmed, got.Status)
}

func TestManualTradeIntent_ConfirmedToSubmitted(t *testing.T) {
	setupManualTradeIntentTestDB(t)
	repo := NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{Symbol: "sh600000", Price: 10, Volume: 100}
	require.NoError(t, repo.CreateManualTradeIntent(intent))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusConfirmed, nil))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusSubmitted, &ManualTradeIntentStatusPatch{
		ClientOrderID: "cli-m-1",
		OrderID:       42,
	}))
	got, err := repo.GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusSubmitted, got.Status)
	require.Equal(t, "cli-m-1", got.ClientOrderID)
	require.Equal(t, uint(42), got.OrderID)
}

func TestManualTradeIntent_ConfirmedToRejected(t *testing.T) {
	setupManualTradeIntentTestDB(t)
	repo := NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{Symbol: "sz300001", Price: 12, Volume: 100}
	require.NoError(t, repo.CreateManualTradeIntent(intent))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusConfirmed, nil))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusRejected, &ManualTradeIntentStatusPatch{
		ErrorCode:    "CASH_INSUFFICIENT",
		ErrorMessage: "现金不足",
	}))
	got, err := repo.GetManualTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualTradeIntentStatusRejected, got.Status)
	require.Equal(t, "CASH_INSUFFICIENT", got.ErrorCode)
	require.Equal(t, "现金不足", got.ErrorMessage)
}

func TestManualTradeIntent_IllegalTransitions(t *testing.T) {
	setupManualTradeIntentTestDB(t)
	repo := NewManualTradeIntentRepo()
	intent := &models.ManualTradeIntent{Symbol: "bj430047", Price: 5, Volume: 100}
	require.NoError(t, repo.CreateManualTradeIntent(intent))

	err := repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusSubmitted, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidManualTradeIntentTransition))

	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusConfirmed, nil))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusSubmitted, nil))

	err = repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusConfirmed, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidManualTradeIntentTransition))

	err = repo.UpdateManualTradeIntentStatus(intent.ID, models.ManualTradeIntentStatusDraft, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidManualTradeIntentTransition))

	// rejected 终态
	intent2 := &models.ManualTradeIntent{Symbol: "sh601398", Price: 5, Volume: 100}
	require.NoError(t, repo.CreateManualTradeIntent(intent2))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent2.ID, models.ManualTradeIntentStatusConfirmed, nil))
	require.NoError(t, repo.UpdateManualTradeIntentStatus(intent2.ID, models.ManualTradeIntentStatusRejected, nil))
	err = repo.UpdateManualTradeIntentStatus(intent2.ID, models.ManualTradeIntentStatusConfirmed, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidManualTradeIntentTransition))
}

func TestManualTradeIntent_ModelHasNoResearchFields(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	modelPath := filepath.Join(filepath.Dir(thisFile), "..", "models", "manual_trade_intent.go")
	src, err := os.ReadFile(modelPath)
	require.NoError(t, err)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, modelPath, src, 0)
	require.NoError(t, err)

	forbidden := map[string]bool{
		"CandidateSnapshotID": true,
		"ResearchSource":      true,
		"SignalScore":         true,
		"SignalTag":           true,
	}
	var structName string
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name == nil {
			return true
		}
		if ts.Name.Name != "ManualTradeIntent" {
			return true
		}
		structName = ts.Name.Name
		st, ok := ts.Type.(*ast.StructType)
		require.True(t, ok)
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				require.False(t, forbidden[name.Name],
					"ManualTradeIntent 不得包含研究字段 %s（防 ResearchTradeIntent 污染）", name.Name)
			}
		}
		return false
	})
	require.Equal(t, "ManualTradeIntent", structName)

	body := string(src)
	for _, needle := range []string{
		"candidate_snapshot_id", "CandidateSnapshotID",
		"research_source", "ResearchSource",
		"signal_score", "SignalScore",
		"signal_tag", "SignalTag",
	} {
		require.NotContains(t, body, needle)
	}
}
