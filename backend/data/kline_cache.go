package data

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"gorm.io/gorm"
)

// KLineCacheRecord 日/周/月/分钟 K 线本地缓存（JSON 序列化）
type KLineCacheRecord struct {
	ID         uint      `gorm:"primaryKey"`
	StockSecID string    `gorm:"column:stock_sec_id;size:32;uniqueIndex:idx_kline_cache_key"`
	Klt        string    `gorm:"column:klt;size:8;uniqueIndex:idx_kline_cache_key"`
	AdjustFlag string    `gorm:"column:adjust_flag;size:8;uniqueIndex:idx_kline_cache_key"`
	EndKey     string    `gorm:"column:end_key;size:32;uniqueIndex:idx_kline_cache_key"`
	Payload    string    `gorm:"column:payload;type:text"`
	BarCount   int       `gorm:"column:bar_count"`
	LastBarDay string    `gorm:"column:last_bar_day;size:16"`
	FetchedAt  time.Time `gorm:"column:fetched_at;index"`
}

func (KLineCacheRecord) TableName() string {
	return "kline_cache"
}

var (
	klineCacheInitOnce  sync.Once
	klineCachePruneOnce sync.Once
)

func ensureKLineCacheTable() {
	klineCacheInitOnce.Do(func() {
		if db.Dao == nil {
			return
		}
		if err := db.Dao.AutoMigrate(&KLineCacheRecord{}); err != nil {
			logger.SugaredLogger.Warnf("kline cache migrate: %v", err)
		}
	})
}

func normalizeKLineCacheEndKey(end string) string {
	end = strings.TrimSpace(end)
	if end == "" || end == "20500101" {
		return "latest"
	}
	return end
}

func klineCacheTTL(klt, endKey string) time.Duration {
	switch strings.TrimSpace(klt) {
	case "101", "102", "103":
		if endKey == "latest" {
			return 60 * time.Second
		}
		return 24 * time.Hour
	default:
		if endKey == "latest" {
			return 30 * time.Second
		}
		return 30 * time.Minute
	}
}

func trimKLineCachePayload(bars []KLineData, limit int) *[]KLineData {
	if bars == nil {
		empty := []KLineData{}
		return &empty
	}
	if limit <= 0 || len(bars) <= limit {
		out := make([]KLineData, len(bars))
		copy(out, bars)
		return &out
	}
	out := make([]KLineData, limit)
	copy(out, bars[len(bars)-limit:])
	return &out
}

func lastKLineDay(bars []KLineData) string {
	if len(bars) == 0 {
		return ""
	}
	return normalizeKLineDayKey(bars[len(bars)-1].Day)
}

func klineCacheGet(stockSecID, klt, adjustFlag, endKey string, limit int) *[]KLineData {
	if db.Dao == nil || stockSecID == "" || limit <= 0 {
		return nil
	}
	ensureKLineCacheTable()
	endKey = normalizeKLineCacheEndKey(endKey)

	var row KLineCacheRecord
	err := db.Dao.Where(
		"stock_sec_id = ? AND klt = ? AND adjust_flag = ? AND end_key = ?",
		stockSecID, klt, adjustFlag, endKey,
	).First(&row).Error
	if err != nil {
		return nil
	}
	if time.Since(row.FetchedAt) > klineCacheTTL(klt, endKey) {
		return nil
	}
	if row.BarCount < limit {
		return nil
	}
	var bars []KLineData
	if json.Unmarshal([]byte(row.Payload), &bars) != nil || len(bars) < limit {
		return nil
	}
	return trimKLineCachePayload(bars, limit)
}

func klineCacheGetStale(stockSecID, klt, adjustFlag, endKey string, limit int) *[]KLineData {
	if db.Dao == nil || stockSecID == "" || limit <= 0 {
		return nil
	}
	ensureKLineCacheTable()
	endKey = normalizeKLineCacheEndKey(endKey)

	var row KLineCacheRecord
	err := db.Dao.Where(
		"stock_sec_id = ? AND klt = ? AND adjust_flag = ? AND end_key = ?",
		stockSecID, klt, adjustFlag, endKey,
	).First(&row).Error
	if err != nil {
		return nil
	}
	if row.BarCount <= 0 {
		return nil
	}
	var bars []KLineData
	if json.Unmarshal([]byte(row.Payload), &bars) != nil || len(bars) == 0 {
		return nil
	}
	return trimKLineCachePayload(bars, limit)
}

func klineCachePut(stockSecID, klt, adjustFlag, endKey string, limit int, data *[]KLineData) {
	if db.Dao == nil || stockSecID == "" || data == nil || len(*data) == 0 {
		return
	}
	ensureKLineCacheTable()
	endKey = normalizeKLineCacheEndKey(endKey)

	bars := make([]KLineData, len(*data))
	copy(bars, *data)
	for i := range bars {
		bars[i].MA = nil
	}
	payload, err := json.Marshal(bars)
	if err != nil {
		return
	}

	row := KLineCacheRecord{
		StockSecID: stockSecID,
		Klt:        klt,
		AdjustFlag: adjustFlag,
		EndKey:     endKey,
		Payload:    string(payload),
		BarCount:   len(bars),
		LastBarDay: lastKLineDay(bars),
		FetchedAt:  time.Now(),
	}

	var existing KLineCacheRecord
	findErr := db.Dao.Where(
		"stock_sec_id = ? AND klt = ? AND adjust_flag = ? AND end_key = ?",
		stockSecID, klt, adjustFlag, endKey,
	).First(&existing).Error
	if findErr == nil {
		row.ID = existing.ID
		if err := db.Dao.Save(&row).Error; err != nil {
			logger.SugaredLogger.Warnf("kline cache save: %v", err)
		}
		return
	}
	if findErr != nil && findErr != gorm.ErrRecordNotFound {
		logger.SugaredLogger.Warnf("kline cache find: %v", findErr)
	}
	if err := db.Dao.Create(&row).Error; err != nil {
		logger.SugaredLogger.Warnf("kline cache create: %v", err)
	}
}

func maybePruneKLineCache() {
	klineCachePruneOnce.Do(func() {
		if db.Dao == nil {
			return
		}
		cutoff := time.Now().Add(-7 * 24 * time.Hour)
		if err := db.Dao.Where("fetched_at < ?", cutoff).Delete(&KLineCacheRecord{}).Error; err != nil {
			logger.SugaredLogger.Warnf("kline cache prune: %v", err)
		}
	})
}

func saveKLineCacheResult(stockSecID, klt, adjustFlag, end string, limit int, data *[]KLineData) {
	if data == nil || len(*data) == 0 {
		return
	}
	klineCachePut(stockSecID, klt, adjustFlag, end, limit, data)
	maybePruneKLineCache()
}
