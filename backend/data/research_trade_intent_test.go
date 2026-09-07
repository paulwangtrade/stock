package data

import (
	"errors"
	"fmt"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupResearchTradeIntentTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:research_intent_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, EnsureResearchTradeIntentTables())
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestCreateResearchTradeIntent_DraftAndFields(t *testing.T) {
	setupResearchTradeIntentTestDB(t)
	repo := NewResearchTradeIntentRepo()

	intent := &models.ResearchTradeIntent{
		Symbol:              "sh600036",
		StockName:           "招商银行",
		Side:                "buy",
		Price:               40.1,
		Volume:              100,
		AccountID:           1,
		CandidateSnapshotID: 88,
		SignalScore:         97,
		SignalTag:           "强",
		Reason:              "候选池模拟买入",
		Status:              models.ResearchTradeIntentStatusConfirmed, // 应被强制为 draft
	}
	require.NoError(t, repo.CreateResearchTradeIntent(intent))
	require.NotZero(t, intent.ID)

	got, err := repo.GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusDraft, got.Status)
	require.Equal(t, "sh600036", got.Symbol)
	require.Equal(t, "招商银行", got.StockName)
	require.Equal(t, models.ResearchSourceSignalScanSnapshot, got.ResearchSource)
	require.Equal(t, uint(88), got.CandidateSnapshotID)
	require.InDelta(t, 97.0, got.SignalScore, 0.01)
	require.Equal(t, "强", got.SignalTag)
	require.Equal(t, "候选池模拟买入", got.Reason)
	require.Empty(t, got.ClientOrderID)
	require.Zero(t, got.OrderID)
}

func TestResearchTradeIntent_AllowedTransitions(t *testing.T) {
	setupResearchTradeIntentTestDB(t)
	repo := NewResearchTradeIntentRepo()

	intent := &models.ResearchTradeIntent{Symbol: "sz000001", Price: 10, Volume: 100}
	require.NoError(t, repo.CreateResearchTradeIntent(intent))

	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusConfirmed, nil))
	got, err := repo.GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusConfirmed, got.Status)

	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusSubmitted, &ResearchTradeIntentStatusPatch{
		ClientOrderID: "cli-1",
		OrderID:       9,
	}))
	got, err = repo.GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusSubmitted, got.Status)
	require.Equal(t, "cli-1", got.ClientOrderID)
	require.Equal(t, uint(9), got.OrderID)
}

func TestResearchTradeIntent_ConfirmedToRejected(t *testing.T) {
	setupResearchTradeIntentTestDB(t)
	repo := NewResearchTradeIntentRepo()

	intent := &models.ResearchTradeIntent{Symbol: "sh600519", Price: 1800, Volume: 100}
	require.NoError(t, repo.CreateResearchTradeIntent(intent))
	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusConfirmed, nil))
	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusRejected, &ResearchTradeIntentStatusPatch{
		ErrorCode:    "pretrade_cash",
		ErrorMessage: "现金不足",
	}))

	got, err := repo.GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusRejected, got.Status)
	require.Equal(t, "pretrade_cash", got.ErrorCode)
	require.Equal(t, "现金不足", got.ErrorMessage)
}

func TestResearchTradeIntent_IllegalTransitions(t *testing.T) {
	setupResearchTradeIntentTestDB(t)
	repo := NewResearchTradeIntentRepo()

	intent := &models.ResearchTradeIntent{Symbol: "sz300001", Price: 12, Volume: 100}
	require.NoError(t, repo.CreateResearchTradeIntent(intent))

	err := repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusSubmitted, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidResearchTradeIntentTransition))

	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusConfirmed, nil))
	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusSubmitted, nil))

	err = repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusConfirmed, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidResearchTradeIntentTransition))

	err = repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusDraft, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidResearchTradeIntentTransition))
}
