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
	row.FetchedAt = time.Now().Add(-11 * time.Minute)
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
