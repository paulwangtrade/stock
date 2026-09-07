package strategy

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/models"
)

// UniverseSignalSnapshotFromCollect builds a directed universe signal snapshot
// from collectUniverse() output. M1-A: snapshot only — not wired into BuildCandidatePool.
func UniverseSignalSnapshotFromCollect(tradeDate string) (*models.SignalScanSnapshot, error) {
	uni := collectUniverse()
	req, err := universeSignalRequestFromCollect(uni, tradeDate)
	if err != nil {
		return nil, err
	}
	return data.NewSignalScanApi().RunUniverseSignalSnapshot(req, nil)
}

// UniverseSignalSnapshotFromRunID builds a directed snapshot for an explicit strategy run
// (same item parser as collectUniverse / parseStrategyRunItems).
func UniverseSignalSnapshotFromRunID(runID uint, tradeDate string) (*models.SignalScanSnapshot, error) {
	if runID == 0 {
		return nil, fmt.Errorf("run id required")
	}
	api := data.NewStockStrategyApi()
	run, err := api.GetRunByID(runID)
	if err != nil || run == nil {
		return nil, fmt.Errorf("strategy run %d not found: %w", runID, err)
	}
	strat, err := api.GetByID(run.StrategyID)
	if err != nil || strat == nil {
		return nil, fmt.Errorf("strategy %d not found: %w", run.StrategyID, err)
	}
	items := parseStrategyRunItems(strat, run)
	if len(items) == 0 {
		return nil, fmt.Errorf("strategy run %d universe empty", runID)
	}
	uni := universeBuildResult{
		Source:        models.CandidatePoolSourceStrategyRun,
		SourceRef:     fmt.Sprintf("strategyId=%d;runId=%d", strat.ID, run.ID),
		Items:         items,
		Message:       fmt.Sprintf("from strategy %q run=%d count=%d", strat.Name, run.ID, len(items)),
		StrategyID:    strat.ID,
		StrategyRunID: run.ID,
		StrategyName:  strat.Name,
	}
	req, err := universeSignalRequestFromCollect(uni, tradeDate)
	if err != nil {
		return nil, err
	}
	return data.NewSignalScanApi().RunUniverseSignalSnapshot(req, nil)
}

func universeSignalRequestFromCollect(uni universeBuildResult, tradeDate string) (data.UniverseSignalScanRequest, error) {
	tradeDate = normalizeTradeDate(tradeDate)
	items := make([]data.UniverseScanItem, 0, len(uni.Items))
	codes := make([]string, 0, len(uni.Items))
	for _, it := range uni.Items {
		code := strings.ToLower(strings.TrimSpace(it.StockCode))
		if code == "" {
			continue
		}
		codes = append(codes, code)
		items = append(items, data.UniverseScanItem{
			StockCode: code,
			StockName: it.StockName,
			Industry:  it.Industry,
		})
	}
	if len(codes) == 0 {
		return data.UniverseSignalScanRequest{}, fmt.Errorf("universe empty: %s", uni.Message)
	}

	req := data.UniverseSignalScanRequest{
		TradeDate:     tradeDate,
		Session:       models.SignalScanSessionClose,
		StockCodes:    codes,
		Items:         items,
		StrategyName:  uni.StrategyName,
		StrategyID:    uni.StrategyID,
		StrategyRunID: uni.StrategyRunID,
	}

	switch uni.Source {
	case models.CandidatePoolSourceStrategyRun:
		if uni.StrategyID == 0 || uni.StrategyRunID == 0 {
			return data.UniverseSignalScanRequest{}, fmt.Errorf("strategy_run missing ids: %s", uni.SourceRef)
		}
		req.StrategyKey = data.StrategyKeyFromID(uni.StrategyID)
		req.UniverseID = data.UniverseIDFromRun(uni.StrategyRunID)
		if req.StrategyName == "" {
			req.StrategyName = fmt.Sprintf("strategy_%d", uni.StrategyID)
		}
	case models.CandidatePoolSourceFollow:
		req.StrategyKey = "follow"
		req.UniverseID = data.UniverseIDFromFollow(tradeDate)
		if req.StrategyName == "" {
			req.StrategyName = "follow"
		}
	default:
		return data.UniverseSignalScanRequest{}, fmt.Errorf("unsupported universe source: %s", uni.Source)
	}
	return req, nil
}
