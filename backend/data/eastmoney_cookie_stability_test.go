package data

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClampEastMoneyCookieTimeout(t *testing.T) {
	if got := clampEastMoneyCookieTimeout(0); got != eastMoneyCookieChromedpMinTimeout {
		t.Fatalf("min clamp: got %v want %v", got, eastMoneyCookieChromedpMinTimeout)
	}
	if got := clampEastMoneyCookieTimeout(5 * time.Minute); got != eastMoneyCookieChromedpMaxTimeout {
		t.Fatalf("max clamp: got %v want %v", got, eastMoneyCookieChromedpMaxTimeout)
	}
	mid := 15 * time.Second
	if got := clampEastMoneyCookieTimeout(mid); got != mid {
		t.Fatalf("mid pass-through: got %v want %v", got, mid)
	}
}

func TestDoEastMoneyCookieSingleflight_CoalescesConcurrentCalls(t *testing.T) {
	var calls atomic.Int32
	var wg sync.WaitGroup
	const n = 8
	results := make([]string, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			h, err := doEastMoneyCookieSingleflight("test-key", func() (string, error) {
				calls.Add(1)
				time.Sleep(80 * time.Millisecond)
				return "a=b", nil
			})
			results[idx] = h
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected singleflight once, got %d calls", got)
	}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d err=%v", i, errs[i])
		}
		if results[i] != "a=b" {
			t.Fatalf("worker %d header=%q", i, results[i])
		}
	}
}

func TestDoEastMoneyCookieSingleflight_PropagatesError(t *testing.T) {
	want := errors.New("boom")
	_, err := doEastMoneyCookieSingleflight("err-key", func() (string, error) {
		return "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("got %v want %v", err, want)
	}
}

func TestRealtimeQuoteHTTPTimeout(t *testing.T) {
	if got := realtimeQuoteHTTPTimeout(nil); got != realtimeQuoteHTTPDefaultTimeout {
		t.Fatalf("nil config: got %v", got)
	}
	if got := realtimeQuoteHTTPTimeout(&SettingConfig{}); got != realtimeQuoteHTTPDefaultTimeout {
		t.Fatalf("empty config: got %v", got)
	}
	if got := realtimeQuoteHTTPTimeout(&SettingConfig{Settings: &Settings{CrawlTimeOut: 0}}); got != realtimeQuoteHTTPDefaultTimeout {
		t.Fatalf("zero crawl: got %v", got)
	}
	if got := realtimeQuoteHTTPTimeout(&SettingConfig{Settings: &Settings{CrawlTimeOut: 60}}); got != realtimeQuoteHTTPMaxTimeout {
		t.Fatalf("large crawl capped: got %v want %v", got, realtimeQuoteHTTPMaxTimeout)
	}
	if got := realtimeQuoteHTTPTimeout(&SettingConfig{Settings: &Settings{CrawlTimeOut: 10}}); got != 10*time.Second {
		t.Fatalf("normal crawl: got %v", got)
	}
}
