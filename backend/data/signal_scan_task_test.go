package data

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignalScanTaskRegistry_GetLatest(t *testing.T) {
	signalScanTaskMu.Lock()
	prevByID := signalScanTaskByID
	prevLatest := signalScanLatestTask
	signalScanTaskByID = map[string]*signalScanTaskState{}
	signalScanLatestTask = ""
	signalScanTaskMu.Unlock()
	t.Cleanup(func() {
		signalScanTaskMu.Lock()
		signalScanTaskByID = prevByID
		signalScanLatestTask = prevLatest
		signalScanTaskMu.Unlock()
	})

	require.Nil(t, GetLatestSignalScanTask())
	require.Nil(t, GetSignalScanTask("missing"))

	const id = "phase67g-test-task"
	signalScanTaskMu.Lock()
	signalScanTaskByID[id] = &signalScanTaskState{view: SignalScanTaskView{
		TaskID: id, Status: SignalScanTaskRunning, HitTotal: 0, Message: "running",
	}}
	signalScanLatestTask = id
	signalScanTaskMu.Unlock()

	got := GetSignalScanTask(id)
	require.NotNil(t, got)
	require.Equal(t, SignalScanTaskRunning, got.Status)
	require.Equal(t, id, got.TaskID)

	latest := GetLatestSignalScanTask()
	require.NotNil(t, latest)
	require.Equal(t, id, latest.TaskID)
}
