package provenance

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

const EngineVersion = "provenance_linker.v1"

// LinkStats summarizes a provenance link pass.
type LinkStats struct {
	SnapshotID uint
	HitsTotal  int
	Matched    int
	Skipped    int
	Unmatched  int
	Linked     bool
}

// ProvenanceMapping is the per-item link outcome (read model + persist result).
type ProvenanceMapping struct {
	ItemID           uint   `json:"item_id"`
	StockCode        string `json:"stock_code"`
	SignalSnapshotID uint   `json:"signal_snapshot_id,omitempty"`
	SignalTag        string `json:"signal_tag,omitempty"`
	Updated          bool   `json:"updated"`
	Skipped          bool   `json:"skipped"`
}

// Linker connects candidate pool items to signal scan snapshots (provenance only).
type Linker struct {
	Snapshots SignalSnapshotSource
	Items     ItemProvenanceUpdater
}

// NewLinker returns a linker with default data-layer dependencies.
func NewLinker() *Linker {
	return &Linker{
		Snapshots: data.NewSignalSnapshotRepo(),
		Items:     data.NewCandidatePoolRepo(),
	}
}

// Link resolves a snapshot, matches pool items by stock code, and updates provenance columns.
func (l *Linker) Link(pool *models.CandidatePool, items []models.CandidatePoolItem) (LinkStats, []ProvenanceMapping, error) {
	stats := LinkStats{}
	mappings := make([]ProvenanceMapping, 0, len(items))
	if l == nil {
		return stats, mappings, fmt.Errorf("provenance linker is nil")
	}
	if pool == nil {
		return stats, mappings, fmt.Errorf("pool is nil")
	}
	if len(items) == 0 {
		stats.Linked = true
		return stats, mappings, nil
	}
	snapshots := l.Snapshots
	if snapshots == nil {
		snapshots = data.NewSignalSnapshotRepo()
	}
	updater := l.Items
	if updater == nil {
		updater = data.NewCandidatePoolRepo()
	}

	snap, err := resolveSnapshot(snapshots, pool)
	if err != nil {
		return stats, mappings, err
	}
	if snap == nil {
		stats.Linked = true
		for _, it := range items {
			mappings = append(mappings, mappingForItem(it, false, false))
			stats.Unmatched++
		}
		return stats, mappings, nil
	}

	hits := snapshots.ParseSnapshotHits(snap)
	byCode := BuildHitIndex(hits)
	stats.SnapshotID = snap.ID
	stats.HitsTotal = len(hits)

	for _, it := range items {
		if it.SignalSnapshotID > 0 {
			stats.Skipped++
			mappings = append(mappings, ProvenanceMapping{
				ItemID:           it.ID,
				StockCode:        normalizeCode(it.StockCode),
				SignalSnapshotID: it.SignalSnapshotID,
				SignalTag:        strings.TrimSpace(it.SignalTag),
				Skipped:          true,
			})
			continue
		}
		code := normalizeCode(it.StockCode)
		hit, ok := byCode[code]
		if !ok {
			stats.Unmatched++
			mappings = append(mappings, mappingForItem(it, false, false))
			continue
		}
		tag := strings.TrimSpace(hit.Tag)
		updated, uerr := updater.UpdateItemProvenance(it.ID, snap.ID, tag)
		if uerr != nil {
			return stats, mappings, uerr
		}
		if updated {
			stats.Matched++
		}
		mappings = append(mappings, ProvenanceMapping{
			ItemID:           it.ID,
			StockCode:        code,
			SignalSnapshotID: snap.ID,
			SignalTag:        tag,
			Updated:          updated,
		})
	}
	stats.Linked = true
	return stats, mappings, nil
}

func resolveSnapshot(src SignalSnapshotSource, pool *models.CandidatePool) (*models.SignalScanSnapshot, error) {
	if src == nil || pool == nil {
		return nil, nil
	}
	cfg := parsePoolConfig(pool.ConfigJSON)

	// P0 — snapshot generated in this BuildCandidatePool pass
	if id := configUint(cfg, "universeSnapshotId"); id > 0 {
		if snap, err := src.GetSnapshotByID(id); err != nil {
			return nil, err
		} else if snap != nil && strings.EqualFold(strings.TrimSpace(snap.Status), "done") {
			return snap, nil
		}
	}

	strategyKey := configString(cfg, "strategyKey")
	universeID := configString(cfg, "universeId")
	sourceDate := configString(cfg, "source_date")
	session := models.SignalScanSessionClose

	// P1 — exact universe snap (strategyKey + universeId)
	if strategyKey != "" && universeID != "" {
		tradeDate := sourceDate
		if tradeDate == "" {
			tradeDate = strings.TrimSpace(pool.TradeDate)
		}
		if snap, err := src.GetUniverseSnapshotByRun(tradeDate, session, strategyKey, universeID); err != nil {
			return nil, err
		} else if snap != nil {
			return snap, nil
		}
	}

	// P2 — latest strategy-scoped universe snap before pool trade_date
	if strategyKey != "" {
		if snap, err := src.GetLatestCloseSnapshotByStrategy(
			strings.TrimSpace(pool.TradeDate), session, strategyKey, models.SignalScanScopeUniverse,
		); err != nil {
			return nil, err
		} else if snap != nil {
			return snap, nil
		}
	}

	// P3 — After-Close source_date (legacy Phase16.8)
	if sourceDate != "" {
		if snap, err := src.GetCloseSnapshotByTradeDate(sourceDate); err != nil {
			return nil, err
		} else if snap != nil {
			return snap, nil
		}
	}

	// P4 — legacy default full-market close snapshot
	return src.GetLatestCloseSignalSnapshot(strings.TrimSpace(pool.TradeDate))
}

func parsePoolConfig(configJSON string) map[string]any {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		return map[string]any{}
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil || cfg == nil {
		return map[string]any{}
	}
	return cfg
}

func configString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	v, _ := cfg[key].(string)
	return strings.TrimSpace(v)
}

func configUint(cfg map[string]any, key string) uint {
	if cfg == nil {
		return 0
	}
	switch v := cfg[key].(type) {
	case float64:
		if v > 0 {
			return uint(v)
		}
	case int:
		if v > 0 {
			return uint(v)
		}
	case int64:
		if v > 0 {
			return uint(v)
		}
	case uint:
		return v
	case json.Number:
		n, _ := v.Int64()
		if n > 0 {
			return uint(n)
		}
	}
	return 0
}

func parseSourceDate(configJSON string) string {
	return configString(parsePoolConfig(configJSON), "source_date")
}

func mappingForItem(it models.CandidatePoolItem, updated, skipped bool) ProvenanceMapping {
	return ProvenanceMapping{
		ItemID:    it.ID,
		StockCode: normalizeCode(it.StockCode),
		Updated:   updated,
		Skipped:   skipped,
	}
}

func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
