package main

import (
	"encoding/json"
	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) emitSignalScanProgress(p data.SignalScanProgress) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "signalScanProgress", map[string]any{
		"phase":   p.Phase,
		"done":    p.Done,
		"total":   p.Total,
		"session": p.Session,
	})
}

// RunSignalScanSnapshot synchronously runs a full-market signal snapshot (legacy / tests).
// Prefer StartSignalScanSnapshot for UI so the frontend is not blocked for tens of minutes.
func (a *App) RunSignalScanSnapshot(session string, signalParamsJson string, strategyID string, strategyName string) (*models.SignalScanSnapshot, error) {
	api := data.NewSignalScanApi()
	snap, err := api.RunFullMarketSnapshot(session, signalParamsJson, strategyID, strategyName, func(p data.SignalScanProgress) {
		a.emitSignalScanProgress(p)
	})
	if err != nil {
		return nil, err
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "signalScanDone", map[string]any{
			"id":        snap.ID,
			"tradeDate": snap.TradeDate,
			"session":   snap.Session,
			"hitTotal":  snap.HitTotal,
		})
	}
	return snap, nil
}

// StartSignalScanSnapshot starts a full-market snapshot job and returns immediately.
func (a *App) StartSignalScanSnapshot(session string, signalParamsJson string, strategyID string, strategyName string) (*data.SignalScanTaskView, error) {
	api := data.NewSignalScanApi()
	task, err := api.StartFullMarketSnapshotAsync(session, signalParamsJson, strategyID, strategyName,
		func(p data.SignalScanProgress) {
			a.emitSignalScanProgress(p)
		},
		func(snap *models.SignalScanSnapshot, runErr error) {
			if a.ctx == nil {
				return
			}
			payload := map[string]any{"ok": runErr == nil}
			if runErr != nil {
				payload["error"] = runErr.Error()
			}
			if snap != nil {
				payload["id"] = snap.ID
				payload["tradeDate"] = snap.TradeDate
				payload["session"] = snap.Session
				payload["hitTotal"] = snap.HitTotal
			}
			runtime.EventsEmit(a.ctx, "signalScanDone", payload)
		},
	)
	return task, err
}

func (a *App) GetSignalScanTask(taskID string) *data.SignalScanTaskView {
	return data.GetSignalScanTask(taskID)
}

func (a *App) GetLatestSignalScanTask() *data.SignalScanTaskView {
	return data.GetLatestSignalScanTask()
}

func (a *App) IsSignalScanRunning() bool {
	return data.NewSignalScanApi().IsRunning()
}

func (a *App) GetLatestSignalScanSnapshot(tradeDate, session string) *models.SignalScanSnapshot {
	snap, err := data.NewSignalScanApi().GetLatestSnapshot(tradeDate, session)
	if err != nil {
		return nil
	}
	return snap
}

func (a *App) GetLatestSignalScanSnapshotByStrategy(tradeDate, session string, strategyID string) *models.SignalScanSnapshot {
	snap, err := data.NewSignalScanApi().GetLatestSnapshotByStrategy(tradeDate, session, strategyID)
	if err != nil {
		return nil
	}
	return snap
}

func (a *App) ListSignalScanSnapshots(query *models.SignalScanSnapshotQuery) *models.SignalScanSnapshotPageResp {
	return data.NewSignalScanApi().ListSnapshots(query)
}

func (a *App) GetSignalScanSnapshotDetail(id uint) *models.SignalScanResultPayload {
	snap, err := data.NewSignalScanApi().GetSnapshotByID(id)
	if err != nil || snap == nil || snap.ResultJSON == "" {
		return nil
	}
	var payload models.SignalScanResultPayload
	if json.Unmarshal([]byte(snap.ResultJSON), &payload) != nil {
		return nil
	}
	return &payload
}

func (a *App) ParseSignalScanSnapshotPayload(snap *models.SignalScanSnapshot) *models.SignalScanResultPayload {
	if snap == nil || snap.ResultJSON == "" {
		return nil
	}
	var payload models.SignalScanResultPayload
	if json.Unmarshal([]byte(snap.ResultJSON), &payload) != nil {
		return nil
	}
	return &payload
}
