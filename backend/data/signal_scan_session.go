package data

import (
	"strings"
	"time"

	"go-stock/backend/tradingcalendar"
)

var shanghaiLoc = time.FixedZone("CST", 8*3600)

func shanghaiNow() time.Time {
	return time.Now().In(shanghaiLoc)
}

func chinaTodayKey(t time.Time) string {
	return t.In(shanghaiLoc).Format("2006-01-02")
}

func isWeekend(t time.Time) bool {
	wd := t.In(shanghaiLoc).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

func isBeforeAShareMarketOpen(t time.Time) bool {
	if isWeekend(t) {
		return false
	}
	hm := t.In(shanghaiLoc).Hour()*60 + t.In(shanghaiLoc).Minute()
	return hm < 9*60+30
}

func isAShareMarketOpenNow(t time.Time) bool {
	if isWeekend(t) {
		return false
	}
	hm := t.In(shanghaiLoc).Hour()*60 + t.In(shanghaiLoc).Minute()
	return (hm >= 9*60+30 && hm < 11*60+30) || (hm >= 13*60 && hm < 15*60)
}

func normalizeScanDayKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ResolveSignalLastBarIndex 与 frontend tradingSession.js 一致
func ResolveSignalLastBarIndex(dayKeys []string, now time.Time) int {
	if len(dayKeys) == 0 {
		return -1
	}
	last := len(dayKeys) - 1
	today := chinaTodayKey(now)
	lastKey := normalizeScanDayKey(dayKeys[last])
	if lastKey != today {
		return last
	}
	if isBeforeAShareMarketOpen(now) && last > 0 {
		return last - 1
	}
	return last
}

// EffectiveSignalTradeDate returns the business trade_date for a signal snapshot.
// Phase16.25: always a real A-share trading day (tradingcalendar), never a weekend/holiday calendar day.
// - If now falls on a trading day → that day
// - Otherwise → previous trading day (PrevTradingDay)
// created_at remains the wall-clock generation time and must not be used as trade_date.
func EffectiveSignalTradeDate(session string, now time.Time) string {
	_ = session
	now = now.In(shanghaiLoc)
	cal := tradingcalendar.Calendar{Location: shanghaiLoc}
	if cal.IsTradingDay(now) {
		return chinaTodayKey(now)
	}
	prev, err := cal.PrevTradingDay(now)
	if err != nil {
		return chinaTodayKey(now)
	}
	return prev.In(shanghaiLoc).Format(tradingcalendar.DateLayout)
}

// SignalTradeDateAdjusted reports whether EffectiveSignalTradeDate differs from the calendar day of now
// (i.e. generation happened on a non-trading day and trade_date was rolled back).
func SignalTradeDateAdjusted(session string, now time.Time) bool {
	return EffectiveSignalTradeDate(session, now) != chinaTodayKey(now)
}
