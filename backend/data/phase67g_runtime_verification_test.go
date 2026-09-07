package data

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPhase67GRuntimeVerification(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TEST") != "1" {
		t.Skip("skip long-running phase67g runtime verification (set RUN_INTEGRATION_TEST=1)")
	}
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(`D:\stock\build\bin`))
	defer func() { _ = os.Chdir(originalWD) }()

	db.Init(`D:\stock\build\bin\data\stock.db?_busy_timeout=60000&_journal_mode=WAL&_synchronous=NORMAL`)
	require.NotNil(t, db.Dao)
	logger.Logger = zap.NewNop()
	logger.SugaredLogger = logger.Logger.Sugar()

	api := NewSignalScanApi()
	before, _ := api.GetLatestSnapshotByStrategy("", models.SignalScanSessionClose, "default")
	if before != nil {
		fmt.Printf("VERIFY_BEFORE snapshot_id=%d trade_date=%s scanned=%d hit=%d duration_ms=%d\n",
			before.ID, before.TradeDate, before.ScannedTotal, before.HitTotal, before.DurationMs)
	}

	lastPhase := ""
	lastReported := 0
	var progressMu sync.Mutex
	started := time.Now()
	task, err := api.StartFullMarketSnapshotAsync(
		models.SignalScanSessionClose,
		"",
		"default",
		"默认策略",
		func(p SignalScanProgress) {
			progressMu.Lock()
			defer progressMu.Unlock()
			if p.Phase != lastPhase || p.Done-lastReported >= 500 || (p.Total > 0 && p.Done == p.Total) {
				fmt.Printf("VERIFY_PROGRESS phase=%s done=%d total=%d at=%s\n",
					p.Phase, p.Done, p.Total, time.Now().Format(time.RFC3339))
				lastPhase = p.Phase
				lastReported = p.Done
			}
		},
		nil,
	)
	returnedIn := time.Since(started)
	require.NoError(t, err)
	require.NotNil(t, task)
	fmt.Printf("VERIFY_TASK_CREATED task_id=%s status=%s start_time=%s return_ms=%d\n",
		task.TaskID, task.Status, task.StartTime, returnedIn.Milliseconds())
	require.Less(t, returnedIn, 2*time.Second)
	require.Equal(t, SignalScanTaskPending, task.Status)

	deadline := time.Now().Add(55 * time.Minute)
	lastStatus := task.Status
	lastHeartbeat := time.Now()
	var final *SignalScanTaskView
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		current := GetSignalScanTask(task.TaskID)
		require.NotNil(t, current)
		if current.Status != lastStatus {
			fmt.Printf("VERIFY_TASK_STATUS task_id=%s from=%s to=%s phase=%s done=%d total=%d\n",
				task.TaskID, lastStatus, current.Status, current.Phase, current.Done, current.Total)
			lastStatus = current.Status
		}
		if time.Since(lastHeartbeat) >= time.Minute {
			fmt.Printf("VERIFY_HEARTBEAT status=%s phase=%s done=%d total=%d elapsed_ms=%d\n",
				current.Status, current.Phase, current.Done, current.Total, time.Since(started).Milliseconds())
			lastHeartbeat = time.Now()
		}
		if current.Status == SignalScanTaskCompleted || current.Status == SignalScanTaskFailed {
			final = current
			break
		}
	}
	require.NotNil(t, final, "task timed out")
	fmt.Printf("VERIFY_TASK_FINAL task_id=%s status=%s snapshot_id=%d start=%s end=%s duration_ms=%d scanned=%d hit=%d error=%q\n",
		final.TaskID, final.Status, final.SnapshotID, final.StartTime, final.EndTime,
		final.DurationMs, final.ScannedTotal, final.HitTotal, final.Error)
	require.Equal(t, SignalScanTaskCompleted, final.Status)
	require.Empty(t, final.Error)
	require.NotZero(t, final.SnapshotID)

	snap, err := api.GetSnapshotByID(final.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, models.SignalScanSessionClose, snap.Session)
	require.Equal(t, "done", snap.Status)
	require.NotEmpty(t, snap.TradeDate)
	require.Equal(t, final.ScannedTotal, snap.ScannedTotal)
	require.Equal(t, final.HitTotal, snap.HitTotal)

	var payload models.SignalScanResultPayload
	require.NoError(t, json.Unmarshal([]byte(snap.ResultJSON), &payload))
	require.Equal(t, snap.TradeDate, payload.TradeDate)
	require.Equal(t, snap.Session, payload.Session)
	require.Equal(t, snap.HitTotal, payload.HitTotal)
	require.Len(t, payload.Items, snap.HitTotal)

	sampleCount := 5
	if len(payload.Items) < sampleCount {
		sampleCount = len(payload.Items)
	}
	for i := 0; i < sampleCount; i++ {
		hit := payload.Items[i]
		fmt.Printf("VERIFY_SAMPLE index=%d code=%s signal=%s score=%d strategy_result=%q\n",
			i, hit.SECUCODE, hit.Tag, hit.SortRank, hit.StatusText)
		require.NotEmpty(t, hit.SECUCODE)
		require.NotEmpty(t, hit.Tag)
	}

	var cacheCountBefore, cacheCountAfter int64
	var cacheMaxBefore, cacheMaxAfter time.Time
	_ = db.Dao.Table("kline_cache").Count(&cacheCountBefore).Error
	_ = db.Dao.Table("kline_cache").Select("MAX(fetched_at)").Scan(&cacheMaxBefore).Error

	queryStarted := time.Now()
	queried, err := api.GetSnapshotByID(snap.ID)
	require.NoError(t, err)
	var queriedPayload models.SignalScanResultPayload
	require.NoError(t, json.Unmarshal([]byte(queried.ResultJSON), &queriedPayload))
	tag := ""
	code := ""
	if len(payload.Items) > 0 {
		tag = payload.Items[0].Tag
		code = payload.Items[0].SECURITY_CODE
	}
	filtered := make([]models.SignalScanHit, 0)
	for _, hit := range queriedPayload.Items {
		if tag != "" && hit.Tag != tag {
			continue
		}
		if code != "" && !strings.Contains(hit.SECURITY_CODE, code) {
			continue
		}
		filtered = append(filtered, hit)
	}
	queryElapsed := time.Since(queryStarted)

	_ = db.Dao.Table("kline_cache").Count(&cacheCountAfter).Error
	_ = db.Dao.Table("kline_cache").Select("MAX(fetched_at)").Scan(&cacheMaxAfter).Error
	fmt.Printf("VERIFY_SEARCH source=signal_scan_snapshots snapshot_id=%d signal=%s stock_code=%s result_count=%d elapsed_ms=%d scan_running=%v kline_cache_count_before=%d after=%d kline_max_before=%s after=%s\n",
		snap.ID, tag, code, len(filtered), queryElapsed.Milliseconds(), api.IsRunning(),
		cacheCountBefore, cacheCountAfter, cacheMaxBefore.Format(time.RFC3339Nano), cacheMaxAfter.Format(time.RFC3339Nano))
	require.False(t, api.IsRunning())
	require.Equal(t, cacheCountBefore, cacheCountAfter)
	require.Equal(t, cacheMaxBefore, cacheMaxAfter)

	require.Equal(t, payload.Items[:sampleCount], queriedPayload.Items[:sampleCount],
		"snapshot query must preserve signal/score/strategy result")
	fmt.Printf("VERIFY_SNAPSHOT id=%d trade_date=%s session=%s status=%s scanned=%d hit=%d result_json_bytes=%d\n",
		snap.ID, snap.TradeDate, snap.Session, snap.Status, snap.ScannedTotal, snap.HitTotal, len(snap.ResultJSON))
}

func TestPhase67GReadLatestVerification(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TEST") != "1" {
		t.Skip("skip phase67g read-latest verification (set RUN_INTEGRATION_TEST=1)")
	}
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(`D:\stock\build\bin`))
	defer func() { _ = os.Chdir(originalWD) }()
	db.Init(`D:\stock\build\bin\data\stock.db?_busy_timeout=60000&_journal_mode=WAL&_synchronous=NORMAL`)

	api := NewSignalScanApi()
	started := time.Now()
	snap, err := api.GetLatestSnapshotByStrategy("", models.SignalScanSessionClose, "default")
	require.NoError(t, err)
	var payload models.SignalScanResultPayload
	require.NoError(t, json.Unmarshal([]byte(snap.ResultJSON), &payload))
	fmt.Printf("VERIFY_LATEST snapshot_id=%d trade_date=%s session=%s status=%s scanned=%d hit=%d duration_ms=%d query_ms=%d created_at=%s\n",
		snap.ID, snap.TradeDate, snap.Session, snap.Status, snap.ScannedTotal, snap.HitTotal,
		snap.DurationMs, time.Since(started).Milliseconds(), snap.CreatedAt.Format(time.RFC3339Nano))
	for i := 0; i < len(payload.Items) && i < 5; i++ {
		hit := payload.Items[i]
		fmt.Printf("VERIFY_LATEST_SAMPLE index=%d code=%s signal=%s score=%d strategy_result=%q\n",
			i, hit.SECUCODE, hit.Tag, hit.SortRank, hit.StatusText)
	}
}
