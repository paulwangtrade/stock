package data

import (
	"encoding/json"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// SignalSnapshotRepo 只读查询 signal_scan_snapshots（不触发扫描、不算分）。
type SignalSnapshotRepo struct{}

func NewSignalSnapshotRepo() *SignalSnapshotRepo { return &SignalSnapshotRepo{} }

// GetSnapshotByID returns a snapshot by primary key (nil if missing).
func (r *SignalSnapshotRepo) GetSnapshotByID(id uint) (*models.SignalScanSnapshot, error) {
	if db.Dao == nil || id == 0 {
		return nil, nil
	}
	var snap models.SignalScanSnapshot
	if err := db.Dao.First(&snap, id).Error; err != nil {
		return nil, nil
	}
	return &snap, nil
}

// GetUniverseSnapshotByRun finds a directed universe snapshot for (tradeDate, session, strategyKey, universeID).
func (r *SignalSnapshotRepo) GetUniverseSnapshotByRun(
	tradeDate, session, strategyKey, universeID string,
) (*models.SignalScanSnapshot, error) {
	if db.Dao == nil {
		return nil, nil
	}
	tradeDate = strings.TrimSpace(tradeDate)
	session = strings.TrimSpace(session)
	strategyKey = strings.TrimSpace(strategyKey)
	universeID = strings.TrimSpace(universeID)
	if tradeDate == "" || strategyKey == "" || universeID == "" {
		return nil, nil
	}
	if session == "" {
		session = models.SignalScanSessionClose
	}
	var snap models.SignalScanSnapshot
	err := db.Dao.Where(
		"trade_date = ? AND session = ? AND scope = ? AND strategy_id = ? AND status = ? AND message LIKE ?",
		tradeDate, session, models.SignalScanScopeUniverse, strategyKey, "done",
		"%universeId="+universeID+"%",
	).Order("id DESC").First(&snap).Error
	if err != nil {
		return nil, nil
	}
	return &snap, nil
}

// GetLatestCloseSnapshotByStrategy returns the latest close/done snapshot before asOfDate for strategy+scope.
func (r *SignalSnapshotRepo) GetLatestCloseSnapshotByStrategy(
	asOfDate, session, strategyKey, scope string,
) (*models.SignalScanSnapshot, error) {
	if db.Dao == nil {
		return nil, nil
	}
	asOfDate = strings.TrimSpace(asOfDate)
	strategyKey = strings.TrimSpace(strategyKey)
	scope = strings.TrimSpace(scope)
	if strategyKey == "" {
		return nil, nil
	}
	if asOfDate == "" {
		asOfDate = time.Now().Format("2006-01-02")
	}
	if session == "" {
		session = models.SignalScanSessionClose
	}
	if scope == "" {
		scope = models.SignalScanScopeUniverse
	}
	var snap models.SignalScanSnapshot
	q := db.Dao.Where(
		"session = ? AND status = ? AND trade_date < ? AND strategy_id = ? AND scope = ?",
		session, "done", asOfDate, strategyKey, scope,
	).Order("trade_date DESC, id DESC")
	if err := q.First(&snap).Error; err != nil {
		return nil, nil
	}
	return &snap, nil
}

// GetLatestCloseSignalSnapshot 取 asOfDate 之前最近一条 close/done 快照。
// 规则：session=close AND status=done AND trade_date < asOfDate
// ORDER BY trade_date DESC, id DESC LIMIT 1
// 无结果返回 (nil, nil)，不报错（周末/节假日自然落到更早交易日）。
func (r *SignalSnapshotRepo) GetLatestCloseSignalSnapshot(asOfDate string) (*models.SignalScanSnapshot, error) {
	if db.Dao == nil {
		return nil, nil
	}
	asOfDate = strings.TrimSpace(asOfDate)
	if asOfDate == "" {
		asOfDate = time.Now().Format("2006-01-02")
	}

	var snap models.SignalScanSnapshot
	err := db.Dao.Where(
		"session = ? AND status = ? AND trade_date < ?",
		models.SignalScanSessionClose, "done", asOfDate,
	).Order("trade_date DESC, id DESC").First(&snap).Error
	if err != nil {
		return nil, nil
	}
	return &snap, nil
}

// GetCloseSnapshotByTradeDate returns the latest close/done snapshot on tradeDate.
func (r *SignalSnapshotRepo) GetCloseSnapshotByTradeDate(tradeDate string) (*models.SignalScanSnapshot, error) {
	if db.Dao == nil {
		return nil, nil
	}
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return nil, nil
	}
	var snap models.SignalScanSnapshot
	err := db.Dao.Where(
		"session = ? AND status = ? AND trade_date = ?",
		models.SignalScanSessionClose, "done", tradeDate,
	).Order("id DESC").First(&snap).Error
	if err != nil {
		return nil, nil
	}
	return &snap, nil
}

// ParseSnapshotHits 解析 result_json 中的 hits（只解析，不算分）。
// 支持：{items:[]}、顶层数组。results[] / code 映射由 research_candidate_pool 灵活解析。
func (r *SignalSnapshotRepo) ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit {
	if snap == nil || strings.TrimSpace(snap.ResultJSON) == "" {
		return nil
	}
	raw := []byte(snap.ResultJSON)
	var payload models.SignalScanResultPayload
	if err := json.Unmarshal(raw, &payload); err == nil && len(payload.Items) > 0 {
		return NormalizeSignalScanHits(payload.Items)
	}
	var items []models.SignalScanHit
	if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
		return NormalizeSignalScanHits(items)
	}
	var wrap struct {
		Items []models.SignalScanHit `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Items) > 0 {
		return NormalizeSignalScanHits(wrap.Items)
	}
	return nil
}
