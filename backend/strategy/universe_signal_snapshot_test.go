package strategy

import (
	"os"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

// TestUniverseSignalSnapshot_StrategyRun82 validates M1-A against the beta runtime DB.
// Requires network for K-line fetch. Skip unless P1612_DB or default beta path exists.
func TestUniverseSignalSnapshot_StrategyRun82(t *testing.T) {
	dbPath := os.Getenv("P1612_DB")
	if dbPath == "" {
		dbPath = `D:\stock\build\bin\data\stock.db`
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("beta db missing:", dbPath)
	}

	origWD, _ := os.Getwd()
	_ = os.Chdir(`D:\stock\build\bin`)
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	db.Init(dbPath + "?_busy_timeout=60000&_journal_mode=WAL&_synchronous=NORMAL")
	require.NotNil(t, db.Dao)

	var run models.StockStrategyRun
	err := db.Dao.First(&run, 82).Error
	require.NoError(t, err, "strategy run #82 must exist")
	require.Equal(t, uint(4), run.StrategyID)

	snap, err := UniverseSignalSnapshotFromRunID(82, "2026-09-02")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.NotZero(t, snap.ID)
	require.Equal(t, models.SignalScanScopeUniverse, snap.Scope)
	require.Equal(t, "stock_strategy:4", snap.StrategyID)
	require.Equal(t, "2026-09-02", snap.TradeDate)
	require.GreaterOrEqual(t, snap.HitTotal, 0)
	require.Greater(t, snap.ScannedTotal, 0)
	require.Contains(t, snap.Message, "universeId=run:82")

	cfg := data.ParseUniverseSnapshotConfig(snap)
	require.NotNil(t, cfg)
	require.Equal(t, uint(4), cfg.StrategyID)
	require.Equal(t, uint(82), cfg.StrategyRunID)
	require.Equal(t, "run:82", cfg.UniverseID)
	require.Equal(t, models.SignalScanScopeUniverse, cfg.Scope)
	require.Equal(t, "stock_strategy:4", cfg.StrategyKey)

	t.Logf("M1-A ok: snap_id=%d scanned=%d hits=%d duration_ms=%d",
		snap.ID, snap.ScannedTotal, snap.HitTotal, snap.DurationMs)
}

func TestUniverseSignalRequestFromCollect_StrategyRun(t *testing.T) {
	uni := universeBuildResult{
		Source:        models.CandidatePoolSourceStrategyRun,
		StrategyID:    4,
		StrategyRunID: 82,
		StrategyName:  "冰点超跌·出坑买点",
		Items: []UniverseCandidate{
			{StockCode: "sz300274", StockName: "阳光电源"},
			{StockCode: "sh600276", StockName: "恒瑞医药"},
		},
	}
	req, err := universeSignalRequestFromCollect(uni, "2026-09-02")
	require.NoError(t, err)
	require.Equal(t, "stock_strategy:4", req.StrategyKey)
	require.Equal(t, "run:82", req.UniverseID)
	require.Equal(t, uint(4), req.StrategyID)
	require.Equal(t, uint(82), req.StrategyRunID)
	require.Len(t, req.StockCodes, 2)
}
