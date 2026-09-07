package provenance

import (
	"testing"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

type snapStub struct {
	latest      *models.SignalScanSnapshot
	byTradeDate *models.SignalScanSnapshot
	byID        map[uint]*models.SignalScanSnapshot
	byUniverse  *models.SignalScanSnapshot
	byStrategy  *models.SignalScanSnapshot
	hits        []models.SignalScanHit
}

func (s *snapStub) GetLatestCloseSignalSnapshot(asOfDate string) (*models.SignalScanSnapshot, error) {
	return s.latest, nil
}

func (s *snapStub) GetCloseSnapshotByTradeDate(tradeDate string) (*models.SignalScanSnapshot, error) {
	return s.byTradeDate, nil
}

func (s *snapStub) GetSnapshotByID(id uint) (*models.SignalScanSnapshot, error) {
	if s.byID == nil {
		return nil, nil
	}
	return s.byID[id], nil
}

func (s *snapStub) GetUniverseSnapshotByRun(tradeDate, session, strategyKey, universeID string) (*models.SignalScanSnapshot, error) {
	return s.byUniverse, nil
}

func (s *snapStub) GetLatestCloseSnapshotByStrategy(asOfDate, session, strategyKey, scope string) (*models.SignalScanSnapshot, error) {
	return s.byStrategy, nil
}

func (s *snapStub) ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit {
	if snap == nil {
		return nil
	}
	return s.hits
}

type updaterStub struct {
	calls []updateCall
}

type updateCall struct {
	itemID     uint
	snapshotID uint
	tag        string
}

func (u *updaterStub) UpdateItemProvenance(itemID uint, snapshotID uint, tag string) (bool, error) {
	for _, c := range u.calls {
		if c.itemID == itemID && c.snapshotID > 0 {
			return false, nil
		}
	}
	u.calls = append(u.calls, updateCall{itemID: itemID, snapshotID: snapshotID, tag: tag})
	return true, nil
}

func TestLink_NoSnapshot(t *testing.T) {
	linker := &Linker{
		Snapshots: &snapStub{latest: nil},
		Items:     &updaterStub{},
	}
	pool := &models.CandidatePool{TradeDate: "2026-09-02"}
	items := []models.CandidatePoolItem{
		{ID: 1, StockCode: "sz000001", Rank: 1, Score: 0.9},
	}
	stats, mappings, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.True(t, stats.Linked)
	require.Equal(t, 0, stats.Matched)
	require.Equal(t, 1, stats.Unmatched)
	require.Len(t, mappings, 1)
	require.False(t, mappings[0].Updated)
	require.Equal(t, 0.9, items[0].Score)
	require.Equal(t, 1, items[0].Rank)
}

func TestLink_MatchOne(t *testing.T) {
	updater := &updaterStub{}
	linker := &Linker{
		Snapshots: &snapStub{
			latest: &models.SignalScanSnapshot{ID: 123, TradeDate: "2026-09-01"},
			hits:   []models.SignalScanHit{{SECUCODE: "000001.SZ", Tag: "强"}},
		},
		Items: updater,
	}
	pool := &models.CandidatePool{TradeDate: "2026-09-02"}
	items := []models.CandidatePoolItem{
		{ID: 10, StockCode: "sz000001", Rank: 1, Score: 0.88},
		{ID: 11, StockCode: "sh600519", Rank: 2, Score: 0.55},
	}
	stats, mappings, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.Equal(t, uint(123), stats.SnapshotID)
	require.Equal(t, 1, stats.Matched)
	require.Equal(t, 1, stats.Unmatched)
	require.Len(t, updater.calls, 1)
	require.Equal(t, uint(10), updater.calls[0].itemID)
	require.Equal(t, uint(123), updater.calls[0].snapshotID)
	require.Equal(t, "强", updater.calls[0].tag)
	require.True(t, mappings[0].Updated)
	require.Equal(t, uint(123), mappings[0].SignalSnapshotID)
	require.Equal(t, "强", mappings[0].SignalTag)
	require.Equal(t, 0.88, items[0].Score)
	require.Equal(t, 1, items[0].Rank)
}

func TestLink_SkipsExistingProvenance(t *testing.T) {
	updater := &updaterStub{}
	linker := &Linker{
		Snapshots: &snapStub{
			latest: &models.SignalScanSnapshot{ID: 99},
			hits:   []models.SignalScanHit{{SECUCODE: "000001.SZ", Tag: "突"}},
		},
		Items: updater,
	}
	items := []models.CandidatePoolItem{
		{ID: 5, StockCode: "sz000001", SignalSnapshotID: 7, SignalTag: "买", Rank: 1, Score: 0.7},
	}
	stats, mappings, err := linker.Link(&models.CandidatePool{TradeDate: "2026-09-02"}, items)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Skipped)
	require.Equal(t, 0, stats.Matched)
	require.Empty(t, updater.calls)
	require.True(t, mappings[0].Skipped)
	require.Equal(t, uint(7), mappings[0].SignalSnapshotID)
}

func TestLink_IdempotentSecondPass(t *testing.T) {
	updater := &updaterStub{}
	linker := &Linker{
		Snapshots: &snapStub{
			latest: &models.SignalScanSnapshot{ID: 50},
			hits:   []models.SignalScanHit{{SECUCODE: "000001.SZ", Tag: "强"}},
		},
		Items: updater,
	}
	pool := &models.CandidatePool{TradeDate: "2026-09-02"}
	items := []models.CandidatePoolItem{{ID: 3, StockCode: "sz000001", Rank: 1, Score: 0.6}}

	stats1, _, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.Equal(t, 1, stats1.Matched)

	items[0].SignalSnapshotID = 50
	items[0].SignalTag = "强"
	stats2, mappings2, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.Equal(t, 1, stats2.Skipped)
	require.Equal(t, 0, stats2.Matched)
	require.True(t, mappings2[0].Skipped)
	require.Len(t, updater.calls, 1)
}

func TestLink_UsesSourceDateFromConfig(t *testing.T) {
	updater := &updaterStub{}
	linker := &Linker{
		Snapshots: &snapStub{
			byTradeDate: &models.SignalScanSnapshot{ID: 200, TradeDate: "2026-09-01"},
			latest:      &models.SignalScanSnapshot{ID: 999, TradeDate: "2026-08-31"},
			hits:        []models.SignalScanHit{{SECUCODE: "000001.SZ", Tag: "趋"}},
		},
		Items: updater,
	}
	pool := &models.CandidatePool{
		TradeDate:  "2026-09-02",
		ConfigJSON: `{"source_date":"2026-09-01"}`,
	}
	items := []models.CandidatePoolItem{{ID: 8, StockCode: "sz000001", Rank: 1, Score: 0.5}}
	stats, _, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.Equal(t, uint(200), stats.SnapshotID)
	require.Equal(t, 1, stats.Matched)
}

func TestLink_PrefersUniverseSnapshotId(t *testing.T) {
	updater := &updaterStub{}
	uniSnap := &models.SignalScanSnapshot{ID: 21, TradeDate: "2026-09-02", Status: "done"}
	linker := &Linker{
		Snapshots: &snapStub{
			byID:        map[uint]*models.SignalScanSnapshot{21: uniSnap},
			byTradeDate: &models.SignalScanSnapshot{ID: 200, Status: "done"},
			latest:      &models.SignalScanSnapshot{ID: 999, Status: "done"},
			hits:        []models.SignalScanHit{{SECUCODE: "000001.SZ", Tag: "强"}},
		},
		Items: updater,
	}
	pool := &models.CandidatePool{
		TradeDate:  "2026-09-03",
		ConfigJSON: `{"universeSnapshotId":21,"source_date":"2026-09-02"}`,
	}
	items := []models.CandidatePoolItem{{ID: 1, StockCode: "sz000001", Rank: 1, Score: 0.9}}
	stats, mappings, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.Equal(t, uint(21), stats.SnapshotID)
	require.Equal(t, 1, stats.Matched)
	require.True(t, mappings[0].Updated)
	require.Equal(t, "强", mappings[0].SignalTag)
}

func TestLink_UniverseSnapshotWithZeroHits(t *testing.T) {
	updater := &updaterStub{}
	uniSnap := &models.SignalScanSnapshot{ID: 21, TradeDate: "2026-09-02", Status: "done"}
	linker := &Linker{
		Snapshots: &snapStub{
			byID: map[uint]*models.SignalScanSnapshot{21: uniSnap},
			hits: nil,
		},
		Items: updater,
	}
	pool := &models.CandidatePool{
		TradeDate:  "2026-09-03",
		ConfigJSON: `{"universeSnapshotId":21}`,
	}
	items := []models.CandidatePoolItem{
		{ID: 1, StockCode: "sz300274", Rank: 1, Score: 0.6},
	}
	stats, mappings, err := linker.Link(pool, items)
	require.NoError(t, err)
	require.Equal(t, uint(21), stats.SnapshotID)
	require.Equal(t, 0, stats.Matched)
	require.Equal(t, 1, stats.Unmatched)
	require.Empty(t, updater.calls)
	require.False(t, mappings[0].Updated)
	require.Equal(t, 0.6, items[0].Score)
	require.Equal(t, 1, items[0].Rank)
}
