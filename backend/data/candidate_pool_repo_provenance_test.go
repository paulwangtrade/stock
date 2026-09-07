package data

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestUpdateItemProvenance_InsertsWhenEmpty(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())

	pool := &models.CandidatePool{
		TradeDate: "2026-09-03", GeneratedAt: time.Now(),
		Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{
		{StockCode: "sz000001", StockName: "平安", Rank: 1, Score: 0.91, StrategyName: "s1"},
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	itemID := items[0].ID

	repo := NewCandidatePoolRepo()
	updated, err := repo.UpdateItemProvenance(itemID, 42, "强")
	require.NoError(t, err)
	require.True(t, updated)

	var row models.CandidatePoolItem
	require.NoError(t, db.Dao.First(&row, itemID).Error)
	require.Equal(t, uint(42), row.SignalSnapshotID)
	require.Equal(t, "强", row.SignalTag)
	require.Equal(t, 1, row.Rank)
	require.InDelta(t, 0.91, row.Score, 1e-9)
}

func TestUpdateItemProvenance_IdempotentWhenAlreadySet(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())

	pool := &models.CandidatePool{
		TradeDate: "2026-09-04", GeneratedAt: time.Now(),
		Source: models.CandidatePoolSourceStrategyRun, Status: models.CandidatePoolStatusReady,
	}
	items := []models.CandidatePoolItem{
		{StockCode: "sz000002", Rank: 1, Score: 0.8, SignalSnapshotID: 7, SignalTag: "买"},
	}
	require.NoError(t, NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	itemID := items[0].ID

	repo := NewCandidatePoolRepo()
	updated, err := repo.UpdateItemProvenance(itemID, 99, "强")
	require.NoError(t, err)
	require.False(t, updated)

	var row models.CandidatePoolItem
	require.NoError(t, db.Dao.First(&row, itemID).Error)
	require.Equal(t, uint(7), row.SignalSnapshotID)
	require.Equal(t, "买", row.SignalTag)
}

func TestUpdateItemProvenance_NoOpOnZeroSnapshot(t *testing.T) {
	setupPaperTradingTestDB(t)
	updated, err := NewCandidatePoolRepo().UpdateItemProvenance(1, 0, "强")
	require.NoError(t, err)
	require.False(t, updated)
}
