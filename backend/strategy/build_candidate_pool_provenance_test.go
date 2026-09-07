package strategy

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/research/provenance"

	"github.com/stretchr/testify/require"
)

func TestApplyPoolProvenanceLink_DoesNotChangeScoreOrRank(t *testing.T) {
	origHook := linkPoolProvenanceHook
	t.Cleanup(func() { linkPoolProvenanceHook = origHook })

	linkPoolProvenanceHook = func(pool *models.CandidatePool, items []models.CandidatePoolItem) (provenance.LinkStats, []provenance.ProvenanceMapping, error) {
		stats := provenance.LinkStats{SnapshotID: 99, Matched: 1, Linked: true, HitsTotal: 1}
		mappings := []provenance.ProvenanceMapping{
			{ItemID: items[0].ID, StockCode: "sz000001", SignalSnapshotID: 99, SignalTag: "强", Updated: true},
		}
		return stats, mappings, nil
	}

	items := []models.CandidatePoolItem{
		{ID: 1, StockCode: "sz000001", Rank: 1, Score: 0.88},
		{ID: 2, StockCode: "sh600519", Rank: 2, Score: 0.55},
	}
	before := snapshotRankScore(items)

	pool := &models.CandidatePool{ID: 10, TradeDate: "2026-09-02", ConfigJSON: `{}`}
	applyPoolProvenanceLink(pool, items)

	require.Equal(t, before, snapshotRankScore(items))
	require.Equal(t, uint(99), items[0].SignalSnapshotID)
	require.Equal(t, "强", items[0].SignalTag)
	require.Equal(t, 1, items[0].Rank)
	require.InDelta(t, 0.88, items[0].Score, 1e-9)
	require.Contains(t, pool.ConfigJSON, `"provenanceMatched":1`)
	require.Contains(t, pool.ConfigJSON, `"provenanceTagged":1`)
}

func TestApplyPoolProvenanceLink_WarnAndKeepPoolOnError(t *testing.T) {
	origHook := linkPoolProvenanceHook
	t.Cleanup(func() { linkPoolProvenanceHook = origHook })

	linkPoolProvenanceHook = func(pool *models.CandidatePool, items []models.CandidatePoolItem) (provenance.LinkStats, []provenance.ProvenanceMapping, error) {
		return provenance.LinkStats{}, nil, assertErr("link failed")
	}

	items := []models.CandidatePoolItem{{ID: 3, StockCode: "sz000001", Rank: 1, Score: 0.7}}
	pool := &models.CandidatePool{ID: 11, TradeDate: "2026-09-02", ConfigJSON: `{}`}

	require.NotPanics(t, func() { applyPoolProvenanceLink(pool, items) })
	require.Equal(t, 1, items[0].Rank)
	require.InDelta(t, 0.7, items[0].Score, 1e-9)
	require.Equal(t, uint(0), items[0].SignalSnapshotID)
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func snapshotRankScore(items []models.CandidatePoolItem) [][3]any {
	out := make([][3]any, len(items))
	for i, it := range items {
		out[i] = [3]any{it.StockCode, it.Rank, it.Score}
	}
	return out
}
