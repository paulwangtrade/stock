package strategy

import (
	"encoding/json"
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/research/provenance"

	"github.com/stretchr/testify/require"
)

func TestApplyUniverseSignalSnapshot_FailureDoesNotPanic(t *testing.T) {
	orig := runUniverseSignalHook
	t.Cleanup(func() { runUniverseSignalHook = orig })

	runUniverseSignalHook = func(uni universeBuildResult, tradeDate, sourceDate string) (*models.SignalScanSnapshot, error) {
		return nil, assertErr("scan failed")
	}

	uni := universeBuildResult{
		Source:        models.CandidatePoolSourceStrategyRun,
		StrategyID:    4,
		StrategyRunID: 82,
		StrategyName:  "冰点超跌",
		Items:         []UniverseCandidate{{StockCode: "sz300274"}},
	}
	cfg := map[string]any{}
	require.NotPanics(t, func() {
		snap := applyUniverseSignalSnapshot(uni, "2026-09-03", cfg)
		require.Nil(t, snap)
	})
	require.Equal(t, false, cfg["universeScanOk"])
	require.Equal(t, uint(4), cfg["strategyId"])
	require.Equal(t, "run:82", cfg["universeId"])
	require.Equal(t, "stock_strategy:4", cfg["strategyKey"])
	_, hasSnapID := cfg["universeSnapshotId"]
	require.False(t, hasSnapID)
}

func TestApplyUniverseSignalSnapshot_ZeroHitsStillOk(t *testing.T) {
	orig := runUniverseSignalHook
	t.Cleanup(func() { runUniverseSignalHook = orig })

	runUniverseSignalHook = func(uni universeBuildResult, tradeDate, sourceDate string) (*models.SignalScanSnapshot, error) {
		return &models.SignalScanSnapshot{
			ID:           21,
			HitTotal:     0,
			ScannedTotal: 41,
			Scope:        models.SignalScanScopeUniverse,
			Status:       "done",
		}, nil
	}

	uni := universeBuildResult{
		Source:        models.CandidatePoolSourceStrategyRun,
		StrategyID:    4,
		StrategyRunID: 82,
		Items:         []UniverseCandidate{{StockCode: "sz300274"}},
	}
	cfg := map[string]any{"source_date": "2026-09-02"}
	snap := applyUniverseSignalSnapshot(uni, "2026-09-03", cfg)
	require.NotNil(t, snap)
	require.Equal(t, true, cfg["universeScanOk"])
	require.Equal(t, uint(21), cfg["universeSnapshotId"])
	require.Equal(t, 0, cfg["universeSnapshotHits"])
	require.Equal(t, 41, cfg["universeSnapshotScanned"])
}

func TestM1B_ProvenanceWritesSignalSnapshotIDWhenHitExists(t *testing.T) {
	origHook := linkPoolProvenanceHook
	t.Cleanup(func() { linkPoolProvenanceHook = origHook })

	// Stub linker path that mimics Resolve P0 + match (fixture/stub).
	linkPoolProvenanceHook = func(pool *models.CandidatePool, items []models.CandidatePoolItem) (provenance.LinkStats, []provenance.ProvenanceMapping, error) {
		var cfg map[string]any
		_ = json.Unmarshal([]byte(pool.ConfigJSON), &cfg)
		snapID, _ := cfg["universeSnapshotId"].(float64)
		require.Equal(t, float64(21), snapID)

		stats := provenance.LinkStats{SnapshotID: 21, Matched: 1, Linked: true, HitsTotal: 1}
		mappings := []provenance.ProvenanceMapping{
			{ItemID: items[0].ID, StockCode: items[0].StockCode, SignalSnapshotID: 21, SignalTag: "强", Updated: true},
		}
		return stats, mappings, nil
	}

	items := []models.CandidatePoolItem{
		{ID: 101, StockCode: "sz000001", Rank: 1, Score: 0.88},
		{ID: 102, StockCode: "sh600519", Rank: 2, Score: 0.55},
	}
	before := snapshotRankScore(items)
	pool := &models.CandidatePool{
		ID:         55,
		TradeDate:  "2026-09-03",
		ConfigJSON: `{"universeSnapshotId":21,"strategyKey":"stock_strategy:4","universeId":"run:82"}`,
	}
	applyPoolProvenanceLink(pool, items)

	require.Equal(t, before, snapshotRankScore(items))
	require.Equal(t, uint(21), items[0].SignalSnapshotID)
	require.Equal(t, "强", items[0].SignalTag)
	require.Equal(t, uint(0), items[1].SignalSnapshotID)
	require.Contains(t, pool.ConfigJSON, `"provenanceMatched":1`)
}

func TestM1B_BuildConfigKeepsRankScoreWhenWiringUniverse(t *testing.T) {
	// Pure config merge + zero-hit snap must not touch item scores.
	items := []models.CandidatePoolItem{
		{ID: 1, StockCode: "sz300274", Rank: 1, Score: 0.6},
	}
	before := snapshotRankScore(items)

	orig := runUniverseSignalHook
	t.Cleanup(func() { runUniverseSignalHook = orig })
	runUniverseSignalHook = func(uni universeBuildResult, tradeDate, sourceDate string) (*models.SignalScanSnapshot, error) {
		return &models.SignalScanSnapshot{ID: 21, HitTotal: 0, ScannedTotal: 41, Status: "done"}, nil
	}

	cfg := map[string]any{}
	snap := applyUniverseSignalSnapshot(universeBuildResult{
		Source: models.CandidatePoolSourceStrategyRun, StrategyID: 4, StrategyRunID: 82,
		Items: []UniverseCandidate{{StockCode: "sz300274"}},
	}, "2026-09-03", cfg)
	require.NotNil(t, snap)
	require.Equal(t, before, snapshotRankScore(items))
}
