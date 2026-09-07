package strategy

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// runUniverseSignalHook is overridable in tests (M1-B).
var runUniverseSignalHook = func(uni universeBuildResult, tradeDate, sourceDate string) (*models.SignalScanSnapshot, error) {
	return runUniverseSignalForPool(uni, tradeDate, sourceDate)
}

func runUniverseSignalForPool(uni universeBuildResult, tradeDate, sourceDate string) (*models.SignalScanSnapshot, error) {
	snapTradeDate := strings.TrimSpace(sourceDate)
	if snapTradeDate == "" {
		snapTradeDate = strings.TrimSpace(tradeDate)
	}
	req, err := universeSignalRequestFromCollect(uni, snapTradeDate)
	if err != nil {
		return nil, err
	}
	return data.NewSignalScanApi().RunUniverseSignalSnapshot(req, nil)
}

// applyUniverseSignalSnapshot runs M1 directed scan after rank freeze.
// Failures are Warn-only; returns nil snap without blocking the pool.
func applyUniverseSignalSnapshot(uni universeBuildResult, tradeDate string, config map[string]any) *models.SignalScanSnapshot {
	if config == nil {
		config = map[string]any{}
	}
	mergeUniverseIdentityIntoConfig(uni, tradeDate, config)

	sourceDate, _ := config["source_date"].(string)
	snap, err := runUniverseSignalHook(uni, tradeDate, sourceDate)
	if err != nil {
		logger.SugaredLogger.Warnf(
			"universe signal snapshot failed (pool continues): source=%s ref=%s err=%v",
			uni.Source, uni.SourceRef, err,
		)
		config["universeScanOk"] = false
		return nil
	}
	if snap == nil {
		config["universeScanOk"] = false
		return nil
	}
	config["universeScanOk"] = true
	config["universeSnapshotId"] = snap.ID
	config["universeSnapshotHits"] = snap.HitTotal
	config["universeSnapshotScanned"] = snap.ScannedTotal
	logger.SugaredLogger.Infof(
		"universe signal snapshot ready: snapId=%d hits=%d scanned=%d strategyKey=%s universeId=%v",
		snap.ID, snap.HitTotal, snap.ScannedTotal, config["strategyKey"], config["universeId"],
	)
	return snap
}

func mergeUniverseIdentityIntoConfig(uni universeBuildResult, tradeDate string, config map[string]any) {
	if config == nil {
		return
	}
	switch uni.Source {
	case models.CandidatePoolSourceStrategyRun:
		if uni.StrategyID > 0 {
			config["strategyId"] = uni.StrategyID
			config["strategyKey"] = data.StrategyKeyFromID(uni.StrategyID)
		}
		if uni.StrategyRunID > 0 {
			config["strategyRunId"] = uni.StrategyRunID
			config["universeId"] = data.UniverseIDFromRun(uni.StrategyRunID)
		}
		if uni.StrategyName != "" {
			config["strategyName"] = uni.StrategyName
		}
	case models.CandidatePoolSourceFollow:
		config["strategyKey"] = "follow"
		config["universeId"] = data.UniverseIDFromFollow(tradeDate)
		config["strategyName"] = "follow"
	}
	if _, ok := config["universeId"]; !ok && uni.SourceRef != "" {
		config["universeId"] = fmt.Sprintf("ref:%s", uni.SourceRef)
	}
}
