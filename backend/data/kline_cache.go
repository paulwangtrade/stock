package data

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/tradingcalendar"
	"gorm.io/gorm"
)

// chinaLocPrefer returns Asia/Shanghai for A-share session clocks; falls back to Local.
func chinaLocPrefer() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil || loc == nil {
		return time.Local
	}
	return loc
}

// klineMarketSessionLive is Phase17-B.1 LIVE vs IDLE: continuous auction only.
// LIVE = trading day ∧ (09:30–11:30 ∨ 13:00–15:00) Shanghai wall clock.
func klineMarketSessionLive(now time.Time) bool {
	t := now.In(chinaLocPrefer())
	if !tradingcalendar.IsTradingDay(t) {
		return false
	}
	hm := t.Hour()*60 + t.Minute()
	return (hm >= 9*60+30 && hm < 11*60+30) || (hm >= 13*60 && hm < 15*60)
}

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
	return klineCacheTTLAt(klt, endKey, time.Now())
}

// klineCacheTTLAt: LIVE keeps short latest TTL; IDLE lengthens so off-hours opens hit SQLite.
func klineCacheTTLAt(klt, endKey string, now time.Time) time.Duration {
	endKey = normalizeKLineCacheEndKey(endKey)
	live := klineMarketSessionLive(now)
	klt = strings.TrimSpace(klt)
	switch klt {
	case "101", "102", "103":
		if endKey == "latest" {
			if live {
				return 60 * time.Second
			}
			if !tradingcalendar.IsTradingDay(now.In(chinaLocPrefer())) {
				return 24 * time.Hour
			}
			return 6 * time.Hour // 午休 / 盘后
		}
		return 24 * time.Hour
	case "104", "106":
		if endKey == "latest" {
			if live {
				return 5 * time.Minute
			}
			return 24 * time.Hour
		}
		return 24 * time.Hour
	default:
		if endKey == "latest" {
			if live {
				return 30 * time.Second
			}
			if !tradingcalendar.IsTradingDay(now.In(chinaLocPrefer())) {
				return 24 * time.Hour
			}
			return 2 * time.Hour
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

// Phase17.1: after a freshness-driven fetch, allow returning calendar-behind
// payload briefly so open/auction without today's bar does not hammer HTTP.
const klineLatestCalendarGrace = 60 * time.Second

func klineTradingCalendar() tradingcalendar.Calendar {
	return tradingcalendar.Calendar{Location: chinaLocPrefer()}
}

// klineExpectedLatestBarDay is the calendar day the latest series last bar
// should have reached (YYYY-MM-DD, Shanghai).
// Non-trading day / before 09:30 → previous trading day; else → today.
func klineExpectedLatestBarDay(now time.Time) string {
	cal := klineTradingCalendar()
	t := now.In(chinaLocPrefer())
	today := cal.TruncateDay(t)
	if !cal.IsTradingDay(today) {
		prev, err := cal.PrevTradingDay(today)
		if err != nil {
			return today.Format(tradingcalendar.DateLayout)
		}
		return prev.Format(tradingcalendar.DateLayout)
	}
	hm := t.Hour()*60 + t.Minute()
	if hm < 9*60+30 {
		prev, err := cal.PrevTradingDay(today)
		if err != nil {
			return today.Format(tradingcalendar.DateLayout)
		}
		return prev.Format(tradingcalendar.DateLayout)
	}
	return today.Format(tradingcalendar.DateLayout)
}

// klineLatestCalendarFresh reports last_bar_day >= expected trading day.
func klineLatestCalendarFresh(lastBarDay string, now time.Time) bool {
	last := normalizeKLineDayKey(lastBarDay)
	if last == "" {
		return false
	}
	expected := normalizeKLineDayKey(klineExpectedLatestBarDay(now))
	if expected == "" {
		return false
	}
	return last >= expected
}

func klineLatestAllowStaleWithinGrace(fetchedAt, now time.Time) bool {
	if fetchedAt.IsZero() {
		return false
	}
	d := now.Sub(fetchedAt)
	return d >= 0 && d < klineLatestCalendarGrace
}

func klineCacheGet(stockSecID, klt, adjustFlag, endKey string, limit int) *[]KLineData {
	return klineCacheGetAt(stockSecID, klt, adjustFlag, endKey, limit, time.Now())
}

func klineCacheGetAt(stockSecID, klt, adjustFlag, endKey string, limit int, now time.Time) *[]KLineData {
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
	if now.Sub(row.FetchedAt) > klineCacheTTLAt(klt, endKey, now) {
		return nil
	}
	var bars []KLineData
	if json.Unmarshal([]byte(row.Payload), &bars) != nil || len(bars) == 0 {
		return nil
	}
	// Phase17-B.1: latest 允许缓存根数 < request limit（假 miss → 全量 HTTP）。
	// 历史 endKey 仍要求足够根数，避免截断窗口误当完整。
	if endKey != "latest" {
		if row.BarCount < limit || len(bars) < limit {
			return nil
		}
	} else {
		// Phase17.1: TTL hit alone is not enough — calendar freshness for latest.
		last := strings.TrimSpace(row.LastBarDay)
		if last == "" {
			last = lastKLineDay(bars)
		}
		if !klineLatestCalendarFresh(last, now) && !klineLatestAllowStaleWithinGrace(row.FetchedAt, now) {
			return nil
		}
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
