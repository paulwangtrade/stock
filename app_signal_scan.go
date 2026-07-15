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

// RunSignalScanSnapshot manually runs a full-market signal snapshot (midday | close).
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
