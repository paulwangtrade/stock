package data

import (
	"fmt"
	"sync"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/google/uuid"
)

// Signal scan async task statuses (UI / GetSignalScanTask).
const (
	SignalScanTaskPending   = "pending"
	SignalScanTaskRunning   = "running"
	SignalScanTaskCompleted = "completed"
	SignalScanTaskFailed    = "failed"
)

// SignalScanTaskView is a read-only snapshot of an async full-market scan job.
// It does not alter strategy computation; the worker still calls RunFullMarketSnapshot.
type SignalScanTaskView struct {
	TaskID       string `json:"taskId"`
	Status       string `json:"status"` // pending|running|completed|failed
	Session      string `json:"session"`
	StrategyID   string `json:"strategyId"`
	StrategyName string `json:"strategyName"`
	StartTime    string `json:"startTime"`
	EndTime      string `json:"endTime,omitempty"`
	DurationMs   int64  `json:"durationMs"`
	SnapshotID   uint   `json:"snapshotId,omitempty"`
	HitTotal     int    `json:"hitTotal"`
	ScannedTotal int    `json:"scannedTotal"`
	Message      string `json:"message"`
	Error        string `json:"error,omitempty"`
	Phase        string `json:"phase,omitempty"`
	Done         int    `json:"done"`
	Total        int    `json:"total"`
	TradeDate    string `json:"tradeDate,omitempty"`
}

type signalScanTaskState struct {
	view SignalScanTaskView
}

var (
	signalScanTaskMu     sync.RWMutex
	signalScanTaskByID   = map[string]*signalScanTaskState{}
	signalScanLatestTask string
)

func cloneTaskView(v SignalScanTaskView) SignalScanTaskView { return v }

func updateSignalScanTask(taskID string, mut func(*SignalScanTaskView)) {
	signalScanTaskMu.Lock()
	defer signalScanTaskMu.Unlock()
	st, ok := signalScanTaskByID[taskID]
	if !ok || st == nil {
		return
	}
	mut(&st.view)
}

// GetSignalScanTask returns a copy of the task view (nil if unknown).
func GetSignalScanTask(taskID string) *SignalScanTaskView {
	signalScanTaskMu.RLock()
	defer signalScanTaskMu.RUnlock()
	st, ok := signalScanTaskByID[taskID]
	if !ok || st == nil {
		return nil
	}
	v := cloneTaskView(st.view)
	return &v
}

// GetLatestSignalScanTask returns the most recently started task, if any.
func GetLatestSignalScanTask() *SignalScanTaskView {
	signalScanTaskMu.RLock()
	defer signalScanTaskMu.RUnlock()
	if signalScanLatestTask == "" {
		return nil
	}
	st, ok := signalScanTaskByID[signalScanLatestTask]
	if !ok || st == nil {
		return nil
	}
	v := cloneTaskView(st.view)
	return &v
}

// StartFullMarketSnapshotAsync creates a task and runs RunFullMarketSnapshot in a goroutine.
// Returns immediately. Strategy/compute path is unchanged.
// onProgress / onComplete are optional (e.g. Wails EventsEmit).
func (a *SignalScanApi) StartFullMarketSnapshotAsync(
	session string,
	signalParamsOverride string,
	strategyID string,
	strategyName string,
	onProgress SignalScanProgressFn,
	onComplete func(snap *models.SignalScanSnapshot, err error),
) (*SignalScanTaskView, error) {
	if a.IsRunning() {
		if latest := GetLatestSignalScanTask(); latest != nil &&
			(latest.Status == SignalScanTaskPending || latest.Status == SignalScanTaskRunning) {
			return latest, fmt.Errorf("信号扫描正在进行中")
		}
		return nil, fmt.Errorf("信号扫描正在进行中")
	}

	taskID := uuid.NewString()
	now := time.Now()
	view := SignalScanTaskView{
		TaskID:       taskID,
		Status:       SignalScanTaskPending,
		Session:      session,
		StrategyID:   strategyID,
		StrategyName: strategyName,
		StartTime:    FormatShanghaiTime(now),
		Message:      "任务已创建，等待执行",
	}

	signalScanTaskMu.Lock()
	signalScanTaskByID[taskID] = &signalScanTaskState{view: view}
	signalScanLatestTask = taskID
	out := cloneTaskView(view)
	signalScanTaskMu.Unlock()

	go func() {
		updateSignalScanTask(taskID, func(v *SignalScanTaskView) {
			v.Status = SignalScanTaskRunning
			v.Message = "全市场扫描进行中"
		})

		prog := func(p SignalScanProgress) {
			updateSignalScanTask(taskID, func(v *SignalScanTaskView) {
				v.Phase = p.Phase
				v.Done = p.Done
				v.Total = p.Total
			})
			if onProgress != nil {
				onProgress(p)
			}
		}

		snap, err := a.RunFullMarketSnapshot(session, signalParamsOverride, strategyID, strategyName, prog)
		end := time.Now()
		if err != nil {
			logger.SugaredLogger.Errorf("StartFullMarketSnapshotAsync failed task=%s: %v", taskID, err)
			updateSignalScanTask(taskID, func(v *SignalScanTaskView) {
				v.Status = SignalScanTaskFailed
				v.EndTime = FormatShanghaiTime(end)
				v.Error = err.Error()
				v.Message = "扫描失败: " + err.Error()
				if v.StartTime != "" {
					if t0, e := time.ParseInLocation("2006-01-02 15:04:05", v.StartTime, shanghaiLoc); e == nil {
						v.DurationMs = end.Sub(t0).Milliseconds()
					}
				}
			})
			if onComplete != nil {
				onComplete(nil, err)
			}
			return
		}

		updateSignalScanTask(taskID, func(v *SignalScanTaskView) {
			v.Status = SignalScanTaskCompleted
			v.EndTime = FormatShanghaiTime(end)
			v.Message = "扫描完成"
			v.Error = ""
			if snap != nil {
				v.SnapshotID = snap.ID
				v.HitTotal = snap.HitTotal
				v.ScannedTotal = snap.ScannedTotal
				v.DurationMs = snap.DurationMs
				v.TradeDate = snap.TradeDate
				v.Session = snap.Session
				if snap.Message != "" {
					v.Message = snap.Message
				}
			}
		})
		if onComplete != nil {
			onComplete(snap, nil)
		}
	}()

	return &out, nil
}
