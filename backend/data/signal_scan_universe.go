package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

var universeScanRunning atomic.Bool

// UniverseSignalScanRequest is the M1 directed-scan input (strategy universe only).
type UniverseSignalScanRequest struct {
	TradeDate        string                  // signal trade date; empty → EffectiveSignalTradeDate
	Session          string                  // close | midday
	StrategyKey      string                  // stock_strategy:4 | follow
	StrategyName     string                  // display name
	UniverseID       string                  // run:82 | follow:2026-09-03
	StockCodes       []string                // lowercase sina codes
	Items            []UniverseScanItem      // optional richer adapter input
	SignalParamsJSON string                  // empty → settings.SignalParams
	StrategyID       uint                    // numeric StockStrategy.id (config)
	StrategyRunID    uint                    // stock_strategy_runs.id (config)
}

// UniverseScanItem is an optional name/industry overlay for the code list.
type UniverseScanItem struct {
	StockCode string
	StockName string
	Industry  string
}

// StrategyKeyFromID formats the snapshot strategy_id contract.
func StrategyKeyFromID(strategyID uint) string {
	if strategyID == 0 {
		return ""
	}
	return fmt.Sprintf("stock_strategy:%d", strategyID)
}

// UniverseIDFromRun formats the universe_id contract.
func UniverseIDFromRun(runID uint) string {
	if runID == 0 {
		return ""
	}
	return fmt.Sprintf("run:%d", runID)
}

// UniverseIDFromFollow formats the follow-source universe_id contract.
func UniverseIDFromFollow(tradeDate string) string {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return "follow"
	}
	return "follow:" + tradeDate
}

// RunUniverseSignalSnapshot runs a directed signal batch on the given universe and persists a snapshot.
// It does not call fetchAllMarketStocks. Failures return (nil, err); callers may Warn and continue.
func (a *SignalScanApi) RunUniverseSignalSnapshot(
	req UniverseSignalScanRequest,
	onProgress SignalScanProgressFn,
) (*models.SignalScanSnapshot, error) {
	if a == nil {
		a = NewSignalScanApi()
	}
	session := strings.TrimSpace(req.Session)
	if session == "" {
		session = models.SignalScanSessionClose
	}
	if session != models.SignalScanSessionMid && session != models.SignalScanSessionClose {
		return nil, fmt.Errorf("invalid session: %s", session)
	}
	strategyKey := strings.TrimSpace(req.StrategyKey)
	if strategyKey == "" {
		return nil, fmt.Errorf("strategyKey required")
	}
	universeID := strings.TrimSpace(req.UniverseID)
	if universeID == "" {
		return nil, fmt.Errorf("universeId required")
	}

	codes := normalizeUniverseCodes(req)
	if len(codes) == 0 {
		return nil, fmt.Errorf("universe stock codes empty")
	}

	if !universeScanRunning.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("universe signal scan already running")
	}
	defer universeScanRunning.Store(false)

	start := time.Now()
	now := shanghaiNow()
	tradeDate := strings.TrimSpace(req.TradeDate)
	if tradeDate == "" {
		tradeDate = EffectiveSignalTradeDate(session, now)
	}

	signalParams := strings.TrimSpace(req.SignalParamsJSON)
	if signalParams == "" {
		if cfg := GetSettingConfig(); cfg != nil {
			signalParams = strings.TrimSpace(cfg.SignalParams)
		}
	}
	strategyName := strings.TrimSpace(req.StrategyName)

	byCode := map[string]UniverseScanItem{}
	for _, it := range req.Items {
		c := strings.ToLower(strings.TrimSpace(it.StockCode))
		if c == "" {
			continue
		}
		byCode[c] = it
	}

	rows := make([]models.StockInfo, 0, len(codes))
	for _, code := range codes {
		it := byCode[code]
		name, industry := it.StockName, it.Industry
		rows = append(rows, AdaptUniverseCodeToStockInfo(code, name, industry))
	}

	if onProgress != nil {
		onProgress(SignalScanProgress{Phase: "fetch", Done: len(rows), Total: len(rows), Session: session})
	}

	indexClose := a.buildIndexCloseMap()
	prepared := make([]preparedScanStock, 0, len(rows))
	var prepMu sync.Mutex
	var prepDone int32
	total := len(rows)

	sem := make(chan struct{}, signalScanKlineConcurrency)
	var wg sync.WaitGroup
	for _, row := range rows {
		row := row
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			p := a.prepareStockBars(row, now)
			if p != nil {
				prepMu.Lock()
				prepared = append(prepared, *p)
				prepMu.Unlock()
			}
			done := int(atomic.AddInt32(&prepDone, 1))
			if onProgress != nil && (done%5 == 0 || done == total) {
				onProgress(SignalScanProgress{Phase: "scan", Done: done, Total: total, Session: session})
			}
		}()
	}
	wg.Wait()

	allItems := make([]map[string]any, 0, 64)
	for i := 0; i < len(prepared); i += signalScanJSChunkSize {
		end := i + signalScanJSChunkSize
		if end > len(prepared) {
			end = len(prepared)
		}
		chunk := make([]signalScanStockInput, 0, end-i)
		for _, p := range prepared[i:end] {
			chunk = append(chunk, p.Input)
		}
		out, err := RunSignalScanBatchJS(signalScanBatchInput{
			Stocks:           chunk,
			IndexClose:       indexClose,
			SignalParamsJSON: signalParams,
			IncludeSell:      true,
		})
		if err != nil {
			logger.SugaredLogger.Errorf("universe signal scan js chunk %d: %v", i/signalScanJSChunkSize, err)
			return nil, err
		}
		allItems = append(allItems, out.Items...)
		if onProgress != nil {
			onProgress(SignalScanProgress{Phase: "compute", Done: end, Total: len(prepared), Session: session})
		}
	}

	cfg := &models.UniverseSignalSnapshotConfig{
		StrategyID:    req.StrategyID,
		StrategyRunID: req.StrategyRunID,
		UniverseID:    universeID,
		Scope:         models.SignalScanScopeUniverse,
		StrategyKey:   strategyKey,
	}
	payload := models.SignalScanResultPayload{
		Items:        mapSliceToHits(allItems),
		ScannedTotal: len(rows),
		HitTotal:     len(allItems),
		TradeDate:    tradeDate,
		Session:      session,
		StrategyID:   strategyKey,
		StrategyName: strategyName,
		CompletedAt:  FormatShanghaiTime(time.Now()),
		Config:       cfg,
	}
	resultJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	msg := formatUniverseSnapshotMessage(cfg)
	snap := &models.SignalScanSnapshot{
		TradeDate:        tradeDate,
		Session:          session,
		Scope:            models.SignalScanScopeUniverse,
		StrategyID:       strategyKey,
		StrategyName:     strategyName,
		SignalParamsJSON: signalParams,
		ScannedTotal:     len(rows),
		HitTotal:         len(allItems),
		Status:           "done",
		Message:          msg,
		ResultJSON:       string(resultJSON),
		DurationMs:       time.Since(start).Milliseconds(),
	}

	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	// Idempotent replace for same trade_date+session+scope+strategy+universe (message marker).
	deleteQuery := db.Dao.Where(
		"trade_date = ? AND session = ? AND scope = ? AND strategy_id = ? AND message LIKE ?",
		tradeDate, session, models.SignalScanScopeUniverse, strategyKey, "%universeId="+universeID+"%",
	)
	if err := deleteQuery.Delete(&models.SignalScanSnapshot{}).Error; err != nil {
		return nil, err
	}
	if err := db.Dao.Create(snap).Error; err != nil {
		return nil, err
	}
	if onProgress != nil {
		onProgress(SignalScanProgress{Phase: "done", Done: len(rows), Total: len(rows), Session: session})
	}
	logger.SugaredLogger.Infof(
		"universe signal snapshot: id=%d trade_date=%s strategyKey=%s universeId=%s scanned=%d hits=%d",
		snap.ID, snap.TradeDate, strategyKey, universeID, snap.ScannedTotal, snap.HitTotal,
	)
	return snap, nil
}

func normalizeUniverseCodes(req UniverseSignalScanRequest) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(req.StockCodes)+len(req.Items))
	appendCode := func(raw string) {
		c := strings.ToLower(strings.TrimSpace(raw))
		if c == "" || seen[c] {
			return
		}
		if !isScannableAShareCode(c) {
			return
		}
		seen[c] = true
		out = append(out, c)
	}
	for _, c := range req.StockCodes {
		appendCode(c)
	}
	for _, it := range req.Items {
		appendCode(it.StockCode)
	}
	return out
}

func formatUniverseSnapshotMessage(cfg *models.UniverseSignalSnapshotConfig) string {
	if cfg == nil {
		return "universe snapshot"
	}
	return fmt.Sprintf(
		"universe snapshot; scope=%s; strategyId=%d; strategyRunId=%d; universeId=%s",
		cfg.Scope, cfg.StrategyID, cfg.StrategyRunID, cfg.UniverseID,
	)
}

// ParseUniverseSnapshotConfig extracts M1 config from a snapshot result_json.
func ParseUniverseSnapshotConfig(snap *models.SignalScanSnapshot) *models.UniverseSignalSnapshotConfig {
	if snap == nil || strings.TrimSpace(snap.ResultJSON) == "" {
		return nil
	}
	var payload models.SignalScanResultPayload
	if err := json.Unmarshal([]byte(snap.ResultJSON), &payload); err != nil {
		return nil
	}
	return payload.Config
}
