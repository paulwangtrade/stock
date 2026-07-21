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

// ParseSnapshotHits 解析 result_json 中的 hits（只解析，不算分）。
// 支持：{items:[]}、顶层数组。results[] / code 映射由 research_candidate_pool 灵活解析。
func (r *SignalSnapshotRepo) ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit {
	if snap == nil || strings.TrimSpace(snap.ResultJSON) == "" {
		return nil
	}
	raw := []byte(snap.ResultJSON)
	var payload models.SignalScanResultPayload
	if err := json.Unmarshal(raw, &payload); err == nil && len(payload.Items) > 0 {
		return payload.Items
	}
	var items []models.SignalScanHit
	if err := json.Unmarshal(raw, &items); err == nil && len(items) > 0 {
		return items
	}
	var wrap struct {
		Items []models.SignalScanHit `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Items) > 0 {
		return wrap.Items
	}
	return nil
}
