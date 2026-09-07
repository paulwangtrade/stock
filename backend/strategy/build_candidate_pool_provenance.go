package strategy

import (
	"encoding/json"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/research/provenance"
)

// linkPoolProvenanceHook is overridable in tests.
var linkPoolProvenanceHook = func(pool *models.CandidatePool, items []models.CandidatePoolItem) (provenance.LinkStats, []provenance.ProvenanceMapping, error) {
	return provenance.NewLinker().Link(pool, items)
}

func applyPoolProvenanceLink(pool *models.CandidatePool, items []models.CandidatePoolItem) {
	if pool == nil || pool.ID == 0 || len(items) == 0 {
		return
	}

	stats, mappings, err := linkPoolProvenanceHook(pool, items)
	syncPoolItemProvenance(items, mappings)
	tagged := countTaggedPoolItems(items)

	if err != nil {
		logger.SugaredLogger.Warnf(
			"candidate pool provenance link failed (pool kept): poolId=%d err=%v",
			pool.ID, err,
		)
		return
	}

	mergeProvenanceConfig(pool, stats, tagged)
	if cfg := pool.ConfigJSON; cfg != "" {
		if uerr := data.NewCandidatePoolRepo().UpdatePoolConfigJSON(pool.ID, cfg); uerr != nil {
			logger.SugaredLogger.Warnf(
				"candidate pool provenance config persist failed: poolId=%d err=%v",
				pool.ID, uerr,
			)
		}
	}

	logger.SugaredLogger.Infof(
		"candidate pool provenance: poolId=%d provenanceMatched=%d provenanceTagged=%d provenanceSkipped=%d snapshot_id=%d hits=%d unmatched=%d",
		pool.ID, stats.Matched, tagged, stats.Skipped, stats.SnapshotID, stats.HitsTotal, stats.Unmatched,
	)

	pool.Items = items
}

func syncPoolItemProvenance(items []models.CandidatePoolItem, mappings []provenance.ProvenanceMapping) {
	byID := map[uint]provenance.ProvenanceMapping{}
	for _, m := range mappings {
		byID[m.ItemID] = m
	}
	for i := range items {
		m, ok := byID[items[i].ID]
		if !ok {
			continue
		}
		if m.Updated {
			items[i].SignalSnapshotID = m.SignalSnapshotID
			items[i].SignalTag = m.SignalTag
		} else if m.Skipped {
			items[i].SignalSnapshotID = m.SignalSnapshotID
			items[i].SignalTag = m.SignalTag
		}
	}
}

func countTaggedPoolItems(items []models.CandidatePoolItem) int {
	n := 0
	for _, it := range items {
		if it.SignalTag != "" {
			n++
		}
	}
	return n
}

func mergeProvenanceConfig(pool *models.CandidatePool, stats provenance.LinkStats, tagged int) {
	if pool == nil {
		return
	}
	cfg := map[string]any{}
	if pool.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(pool.ConfigJSON), &cfg)
	}
	cfg["provenanceLinked"] = stats.Linked
	cfg["provenanceSnapshotId"] = stats.SnapshotID
	cfg["provenanceHits"] = stats.HitsTotal
	cfg["provenanceMatched"] = stats.Matched
	cfg["provenanceTagged"] = tagged
	cfg["provenanceSkipped"] = stats.Skipped
	cfg["provenanceUnmatched"] = stats.Unmatched
	cfg["provenanceEngine"] = provenance.EngineVersion
	raw, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	pool.ConfigJSON = string(raw)
}
