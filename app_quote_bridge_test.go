package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"go-stock/backend/cache"
	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

type stubAppQuoteService struct {
	quotes []marketdata.Quote
	err    error
	calls  [][]string
}

func (s *stubAppQuoteService) GetQuote(code string) (*marketdata.Quote, error) {
	qs, err := s.GetQuotes([]string{code})
	if err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, marketdata.ErrNoData
	}
	out := qs[0]
	return &out, nil
}

func (s *stubAppQuoteService) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	s.calls = append(s.calls, append([]string(nil), codes...))
	if s.err != nil {
		return nil, s.err
	}
	return s.quotes, nil
}

func TestQuoteToStockInfo_MarketFieldsOnly(t *testing.T) {
	q := marketdata.Quote{
		Code: "sz000001", Name: "平安银行",
		Price: 11.1, Open: 11.09, High: 11.18, Low: 11.09,
		Volume: 1e6, Amount: 2e7, PreClose: 11.0,
		ChangePercent: 0.9, ChangeValue: 0.1,
		Date: "2026-07-27", Time: "14:30:00",
		FetchedAt: time.Now(),
	}
	info := quoteToStockInfo(q)
	if info.Price != "11.1" || info.Open != "11.09" || info.High != "11.18" || info.Low != "11.09" {
		t.Fatalf("OHLC mapping mismatch: %+v", info)
	}
	if info.Volume != "1000000" || info.Amount != "20000000" {
		t.Fatalf("volume/amount mismatch: %+v", info)
	}
	if info.Date != "2026-07-27" || info.Time != "14:30:00" {
		t.Fatalf("timestamp mismatch: %+v", info)
	}
	if info.CostPrice != 0 || info.Profit != 0 || info.CostVolume != 0 || info.AlarmPrice != 0 {
		t.Fatalf("non-market fields leaked into quote mapping: %+v", info)
	}
}

func TestFetchRealtimeStockInfos_FakeInject(t *testing.T) {
	stub := &stubAppQuoteService{quotes: []marketdata.Quote{{
		Code: "sh600519", Name: "贵州茅台", Price: 1600, Open: 1590, High: 1610, Low: 1580,
		Volume: 100, Amount: 160000, Date: "2026-07-27", Time: "09:35:00",
	}}}
	got, err := fetchRealtimeStockInfos(stub, []string{"sh600519"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(stub.calls) != 1 || stub.calls[0][0] != "sh600519" {
		t.Fatalf("expected GetQuotes call, got %+v", stub.calls)
	}
	if len(got) != 1 || got[0].Price != "1600" || got[0].Name != "贵州茅台" {
		t.Fatalf("unexpected stock info: %+v", got)
	}
}

func TestFetchRealtimeStockInfos_NilService(t *testing.T) {
	_, err := fetchRealtimeStockInfos(nil, []string{"sz000001"})
	if !errors.Is(err, errQuoteServiceUninitialized) {
		t.Fatalf("expected uninitialized, got %v", err)
	}
}

func TestGetStockInfosWithQuoteService_ConsumerParity(t *testing.T) {
	// 使用无市场前缀码，避免闭市时 GetStockInfos 时段门控跳过取数（门控语义保持不变）。
	q := marketdata.Quote{
		Code: "TEST001", Name: "测试股",
		Price: 11.1, Open: 11.09, High: 11.18, Low: 11.09,
		Volume: 123, Amount: 456, PreClose: 11.0,
		Date: "2026-07-24", Time: "16:14:36",
	}
	legacyDisplay := data.StockInfo{
		Code: q.Code, Name: q.Name,
		Price: formatQuoteFloat(q.Price), Open: formatQuoteFloat(q.Open),
		High: formatQuoteFloat(q.High), Low: formatQuoteFloat(q.Low),
		Volume: formatQuoteFloat(q.Volume), Amount: formatQuoteFloat(q.Amount),
		Date: q.Date, Time: q.Time,
	}
	stub := &stubAppQuoteService{quotes: []marketdata.Quote{q}}
	follows := []data.FollowedStock{{StockCode: "TEST001", Name: "测试股", CostPrice: 99, Volume: 100}}
	out, err := getStockInfosWithQuoteService(stub, false, follows...)
	if err != nil {
		t.Fatalf("getStockInfos: %v", err)
	}
	if len(*out) != 1 {
		t.Fatalf("want 1, got %d (calls=%+v)", len(*out), stub.calls)
	}
	got := (*out)[0]
	if got.Price != legacyDisplay.Price || got.Open != legacyDisplay.Open ||
		got.High != legacyDisplay.High || got.Low != legacyDisplay.Low ||
		got.Volume != legacyDisplay.Volume || got.Amount != legacyDisplay.Amount ||
		got.Date != legacyDisplay.Date || got.Time != legacyDisplay.Time {
		t.Fatalf("display field mismatch\n legacy=%+v\n got=%+v", legacyDisplay, got)
	}
	if got.CostPrice != 0 || got.Profit != 0 {
		t.Fatalf("follow-derived fields should not appear without applyFollow: %+v", got)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("QuoteService not used: %+v", stub.calls)
	}
}

func TestGetStockInfosRealtimeBatchWithQuoteService_UsesCacheAndService(t *testing.T) {
	code := "sz000858"
	// 清空该码缓存条目：写入过期不可能；用唯一码避免与其它测试冲突
	stub := &stubAppQuoteService{quotes: []marketdata.Quote{{
		Code: code, Name: "五粮液", Price: 73.57, Open: 74.6, High: 75.3, Low: 73.47,
		Volume: 10, Amount: 20, Date: "2026-07-24", Time: "16:14:42",
	}}}
	follows := []data.FollowedStock{{StockCode: code, Name: "五粮液"}}

	// 确保 miss：写入后再用极短 TTL 不便；直接用 Partition — 若已有全局缓存 hit，仍应返回正确价。
	// 先 SetBatch 再读验证 cache path 不二次调用 service。
	first := getStockInfosRealtimeBatchWithQuoteService(stub, false, follows...)
	if len(*first) != 1 || (*first)[0].Price != "73.57" {
		t.Fatalf("first batch unexpected: %+v calls=%+v", first, stub.calls)
	}
	callsAfterFirst := len(stub.calls)

	second := getStockInfosRealtimeBatchWithQuoteService(stub, false, follows...)
	if len(*second) != 1 || (*second)[0].Price != "73.57" {
		t.Fatalf("second batch unexpected: %+v", second)
	}
	// 缓存命中时不应再次 GetQuotes
	if len(stub.calls) != callsAfterFirst {
		t.Fatalf("cache should prevent second GetQuotes: calls before=%d after=%d %+v",
			callsAfterFirst, len(stub.calls), stub.calls)
	}

	// 确认缓存层仍为 FollowRealtimePriceCache（未改 TTL 语义，仅验证可读写）
	hits, misses := cache.FollowRealtimePriceCache.Partition([]string{code})
	if len(hits) == 0 || len(misses) != 0 {
		t.Fatalf("expected cache hit for %s, hits=%d misses=%d", code, len(hits), len(misses))
	}
}

func TestGetQuoteService_AccessInjectForApp(t *testing.T) {
	t.Cleanup(func() {
		data.SetQuoteService(nil)
	})
	stub := &stubAppQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Name: "平安银行", Price: 1, Open: 1, High: 1, Low: 1,
	}}}
	data.SetQuoteService(stub)
	got := data.GetQuoteService()
	if got == nil {
		t.Fatal("GetQuoteService nil after inject")
	}
	infos, err := fetchRealtimeStockInfos(got, []string{"sz000001"})
	if err != nil || len(infos) != 1 || !strings.Contains(infos[0].Name, "平安") {
		t.Fatalf("injected service failed: err=%v infos=%+v", err, infos)
	}
}
