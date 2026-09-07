package opportunity_test

import (
	"fmt"
	"testing"

	"go-stock/backend/db"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpportunityTestDB(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() { db.Dao = original })

	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	require.NoError(t, opportunity.EnsureSchema(db.Dao))
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func TestSaveUserOpportunityAction_WATCH(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	batch := "snap:1"

	row, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		Secucode:     "600363.SH",
		SignalTime:   "2026-08-18",
		SignalTag:    "强",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)
	require.NotEmpty(t, row.ID)
	require.Equal(t, opportunity.ActionWatch, row.Action)
	require.Equal(t, "sh600363", row.StockCode)
	require.NotEmpty(t, row.OpportunityID)

	latest, err := opportunity.LoadLatestActionsByBatch(acc.ID, batch)
	require.NoError(t, err)
	require.Equal(t, opportunity.ActionWatch, latest[row.OpportunityID].Action)
}

func TestSaveUserOpportunityAction_AppendOnly(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	batch := "snap:2"
	oid := opportunity.BuildOpportunityID(batch, "600363.SH", "2026-08-18", "强")

	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     acc.ID,
		ScanBatchKey:  batch,
		OpportunityID: oid,
		StockCode:     "sh600363",
		Action:        opportunity.ActionWatch,
	})
	require.NoError(t, err)

	_, err = opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     acc.ID,
		ScanBatchKey:  batch,
		OpportunityID: oid,
		StockCode:     "sh600363",
		Action:        opportunity.ActionIgnore,
	})
	require.NoError(t, err)

	latest, err := opportunity.LoadLatestActionsByBatch(acc.ID, batch)
	require.NoError(t, err)
	require.Equal(t, opportunity.ActionIgnore, latest[oid].Action)

	var count int64
	require.NoError(t, db.Dao.Model(&opportunity.UserOpportunityAction{}).
		Where("opportunity_id = ?", oid).Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestSaveUserOpportunityAction_InvalidAction(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: "snap:3",
		StockCode:    "sh600363",
		Action:       "CREATE_PLAN",
	})
	require.Error(t, err)
}
