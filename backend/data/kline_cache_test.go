package data

import (
	"testing"
	"time"

	"go-stock/backend/db"
)

func TestNormalizeKLineCacheEndKey(t *testing.T) {
	if normalizeKLineCacheEndKey("") != "latest" {
		t.Fatal("empty end")
	}
	if normalizeKLineCacheEndKey("20500101") != "latest" {
		t.Fatal("20500101 end")
	}
	if normalizeKLineCacheEndKey("20260101") != "20260101" {
		t.Fatal("historical end")
	}
}

func TestKLineCachePutGet(t *testing.T) {
	db.Init("file::memory:?cache=shared&_busy_timeout=10000")
	ensureKLineCacheTable()

	bars := []KLineData{
		{Day: "2026-06-01", Open: "10", Close: "10.5", High: "11", Low: "9.8", Volume: "100"},
		{Day: "2026-06-02", Open: "10.5", Close: "11", High: "11.2", Low: "10.2", Volume: "120"},
	}
	data := &bars
	klineCachePut("0.000001", "101", "", "20500101", 2, data)

	got := klineCacheGet("0.000001", "101", "", "20500101", 2)
	if got == nil || len(*got) != 2 {
		t.Fatalf("cache miss or wrong len: %v", got)
	}
	if (*got)[1].Day != "2026-06-02" {
		t.Fatalf("unexpected last day: %s", (*got)[1].Day)
	}

	one := klineCacheGet("0.000001", "101", "", "20500101", 1)
	if one == nil || len(*one) != 1 || (*one)[0].Day != "2026-06-02" {
		t.Fatalf("trim limit failed: %+v", one)
	}
}

func TestKLineCacheTTLExpired(t *testing.T) {
	db.Init("file::memory:?cache=shared&_busy_timeout=10000")
	ensureKLineCacheTable()

	bars := []KLineData{{Day: "2026-06-01", Close: "1"}}
	klineCachePut("1.600000", "101", "", "latest", 1, &bars)

	var row KLineCacheRecord
	if err := db.Dao.Where("stock_sec_id = ?", "1.600000").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	// Beyond IDLE day TTL (6h / 24h) so miss regardless of LIVE/IDLE.
	row.FetchedAt = time.Now().Add(-25 * time.Hour)
	if err := db.Dao.Save(&row).Error; err != nil {
		t.Fatal(err)
	}
	if got := klineCacheGet("1.600000", "101", "", "latest", 1); got != nil {
		t.Fatal("expected ttl miss")
	}
	stale := klineCacheGetStale("1.600000", "101", "", "latest", 1)
	if stale == nil || len(*stale) != 1 || (*stale)[0].Day != "2026-06-01" {
		t.Fatalf("expected stale cache hit, got %+v", stale)
	}
}

func TestKLineCacheGetLatestAllowsFewerBarsThanLimit(t *testing.T) {
	db.Init("file::memory:?cache=shared&_busy_timeout=10000")
	ensureKLineCacheTable()

	bars := []KLineData{
		{Day: "2026-06-01", Close: "10"},
		{Day: "2026-06-02", Close: "11"},
		{Day: "2026-06-03", Close: "12"},
		{Day: "2026-06-04", Close: "13"},
		{Day: "2026-06-05", Close: "14"},
	}
	klineCachePut("0.000002", "101", "", "latest", 5, &bars)

	got := klineCacheGet("0.000002", "101", "", "latest", 800)
	if got == nil || len(*got) != 5 {
		t.Fatalf("expected hit with 5 bars when limit=800, got %+v", got)
	}

	got250 := klineCacheGet("0.000002", "101", "", "latest", 2)
	if got250 == nil || len(*got250) != 2 || (*got250)[1].Day != "2026-06-05" {
		t.Fatalf("expected trim to last 2, got %+v", got250)
	}
}

func TestKlineCacheTTLAtLiveVsIdle(t *testing.T) {
	// Monday 10:00 Shanghai ≈ LIVE
	liveAt := time.Date(2026, 9, 7, 10, 0, 0, 0, chinaLocPrefer())
	idleAt := time.Date(2026, 9, 6, 22, 0, 0, 0, chinaLocPrefer()) // Saturday night
	if klineCacheTTLAt("101", "latest", liveAt) != 60*time.Second {
		t.Fatalf("LIVE day ttl: %v", klineCacheTTLAt("101", "latest", liveAt))
	}
	if klineCacheTTLAt("101", "latest", idleAt) != 24*time.Hour {
		t.Fatalf("weekend IDLE day ttl: %v", klineCacheTTLAt("101", "latest", idleAt))
	}
	if !klineMarketSessionLive(liveAt) {
		t.Fatal("expected LIVE at Mon 10:00")
	}
	if klineMarketSessionLive(idleAt) {
		t.Fatal("expected IDLE on Saturday")
	}
}

func TestMergeKLineDataByDay(t *testing.T) {
	base := []KLineData{
		{Day: "2026-06-01", Close: "10"},
		{Day: "2026-06-02", Close: "11"},
		{Day: "2026-06-03", Close: "12"},
	}
	latest := []KLineData{
		{Day: "2026-06-03", Close: "12.5"},
		{Day: "2026-06-04", Close: "13"},
	}
	got := mergeKLineDataByDay(&base, &latest, 3)
	if got == nil || len(*got) != 3 {
		t.Fatalf("unexpected len: %+v", got)
	}
	if (*got)[1].Day != "2026-06-03" || (*got)[1].Close != "12.5" {
		t.Fatalf("expected duplicate day to be replaced, got %+v", *got)
	}
	if (*got)[2].Day != "2026-06-04" {
		t.Fatalf("expected latest day appended, got %+v", *got)
	}
}

func TestKlineExpectedLatestBarDayAndFresh(t *testing.T) {
	loc := chinaLocPrefer()
	// Scene1: Mon 10:00 trading session — expect today; Fri last bar is stale.
	mon1000 := time.Date(2026, 9, 7, 10, 0, 0, 0, loc)
	if got := klineExpectedLatestBarDay(mon1000); got != "2026-09-07" {
		t.Fatalf("Mon 10:00 expected day=%s", got)
	}
	if klineLatestCalendarFresh("2026-09-04", mon1000) {
		t.Fatal("scene1: last=09-04 at Mon 10:00 should be fresh=false")
	}
	if !klineLatestCalendarFresh("2026-09-07", mon1000) {
		t.Fatal("scene1: last=09-07 should be fresh")
	}

	// Scene2: weekend — expect prior trading day (Fri); no force to Sat/Sun.
	sat := time.Date(2026, 9, 5, 15, 0, 0, 0, loc)
	if got := klineExpectedLatestBarDay(sat); got != "2026-09-04" {
		t.Fatalf("Sat expected day=%s want 2026-09-04", got)
	}
	if !klineLatestCalendarFresh("2026-09-04", sat) {
		t.Fatal("scene2: Fri bar on weekend should be fresh")
	}

	// Before open on trading day — expect previous close day.
	mon0900 := time.Date(2026, 9, 7, 9, 0, 0, 0, loc)
	if got := klineExpectedLatestBarDay(mon0900); got != "2026-09-04" {
		t.Fatalf("Mon 09:00 expected day=%s", got)
	}
	if !klineLatestCalendarFresh("2026-09-04", mon0900) {
		t.Fatal("pre-open Fri bar should be fresh")
	}

	// Lunch IDLE still expects today.
	mon1200 := time.Date(2026, 9, 7, 12, 0, 0, 0, loc)
	if klineMarketSessionLive(mon1200) {
		t.Fatal("expected IDLE at lunch")
	}
	if got := klineExpectedLatestBarDay(mon1200); got != "2026-09-07" {
		t.Fatalf("lunch expected day=%s", got)
	}
	if klineLatestCalendarFresh("2026-09-04", mon1200) {
		t.Fatal("lunch: Fri bar must be stale")
	}
}

func TestKlineCacheGetLatestRejectsCalendarStale(t *testing.T) {
	db.Init("file::memory:?cache=shared&_busy_timeout=10000")
	ensureKLineCacheTable()

	bars := []KLineData{
		{Day: "2026-09-03", Close: "10"},
		{Day: "2026-09-04", Close: "11"},
	}
	klineCachePut("0.300085", "101", "", "latest", 2, &bars)

	var row KLineCacheRecord
	if err := db.Dao.Where("stock_sec_id = ?", "0.300085").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	// Outside grace window but inside 6h IDLE TTL.
	row.FetchedAt = time.Date(2026, 9, 7, 9, 45, 0, 0, chinaLocPrefer())
	row.LastBarDay = "2026-09-04"
	if err := db.Dao.Save(&row).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 7, 10, 0, 0, 0, chinaLocPrefer())
	if got := klineCacheGetAt("0.300085", "101", "", "latest", 2, now); got != nil {
		t.Fatalf("scene4: calendar-stale latest must miss, got %+v", got)
	}
	stale := klineCacheGetStale("0.300085", "101", "", "latest", 2)
	if stale == nil || len(*stale) != 2 {
		t.Fatalf("GetStale should still read rows, got %+v", stale)
	}
	if klineLatestCalendarFresh(lastKLineDay(*stale), now) {
		t.Fatal("IDLE short-circuit must not treat this as fresh")
	}
}

func TestKlineCacheGetHistoricalEndSkipsFreshness(t *testing.T) {
	db.Init("file::memory:?cache=shared&_busy_timeout=10000")
	ensureKLineCacheTable()

	bars := []KLineData{
		{Day: "2026-08-01", Close: "1"},
		{Day: "2026-08-02", Close: "2"},
	}
	end := "20260802"
	klineCachePut("0.300086", "101", "", end, 2, &bars)

	now := time.Date(2026, 9, 7, 10, 0, 0, 0, chinaLocPrefer())
	got := klineCacheGetAt("0.300086", "101", "", end, 2, now)
	if got == nil || len(*got) != 2 {
		t.Fatalf("scene3: historical end should hit without calendar freshness, got %+v", got)
	}
}

func TestKlineCacheGetGraceAllowsBriefBehind(t *testing.T) {
	db.Init("file::memory:?cache=shared&_busy_timeout=10000")
	ensureKLineCacheTable()

	bars := []KLineData{{Day: "2026-09-04", Close: "11"}}
	klineCachePut("0.300087", "101", "", "latest", 1, &bars)

	var row KLineCacheRecord
	if err := db.Dao.Where("stock_sec_id = ?", "0.300087").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, chinaLocPrefer())
	row.FetchedAt = now.Add(-30 * time.Second)
	row.LastBarDay = "2026-09-04"
	if err := db.Dao.Save(&row).Error; err != nil {
		t.Fatal(err)
	}
	if got := klineCacheGetAt("0.300087", "101", "", "latest", 1, now); got == nil {
		t.Fatal("within 60s grace, calendar-behind latest may still return")
	}
}
