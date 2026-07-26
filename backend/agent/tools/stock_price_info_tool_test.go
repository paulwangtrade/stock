package tools

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
	"go-stock/backend/marketdata/adapter"
)

type stubQuoteService struct {
	quotes []marketdata.Quote
	err    error
	calls  [][]string
}

func (s *stubQuoteService) GetQuote(code string) (*marketdata.Quote, error) {
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

func (s *stubQuoteService) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	s.calls = append(s.calls, append([]string(nil), codes...))
	if s.err != nil {
		return nil, s.err
	}
	return s.quotes, nil
}

func TestQueryStockPriceInfoJSON_UsesQuoteService(t *testing.T) {
	stub := &stubQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Name: "平安银行",
		Price: 11.2, Open: 11.0, High: 11.5, Low: 10.9,
		Volume: 1e6, Amount: 2e7, PreClose: 11.1,
		Date: "2026-07-27", Time: "14:30:00",
		FetchedAt: time.Date(2026, 7, 27, 14, 30, 0, 0, time.Local),
	}}}
	out, err := QueryStockPriceInfoJSON(stub, []string{"sz000001"})
	if err != nil {
		t.Fatalf("QueryStockPriceInfoJSON: %v", err)
	}
	if len(stub.calls) != 1 || len(stub.calls[0]) != 1 || stub.calls[0][0] != "sz000001" {
		t.Fatalf("expected GetQuotes([sz000001]), got %+v", stub.calls)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json: %v body=%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d (%s)", len(rows), out)
	}
	if rows[0]["股票代码"] != "sz000001" || rows[0]["股票名称"] != "平安银行" {
		t.Fatalf("unexpected row: %+v", rows[0])
	}
	if rows[0]["当前价格"] != "11.2" {
		t.Fatalf("price mismatch: %+v", rows[0])
	}
	for _, forbidden := range []string{"成本", "持仓", "盈亏", "关注价", "报警", "订单", "计划", "风控"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("output leaked non-market field %q: %s", forbidden, out)
		}
	}
}

func TestQueryStockPriceInfoJSON_NilService(t *testing.T) {
	_, err := QueryStockPriceInfoJSON(nil, []string{"sz000001"})
	if err == nil || !strings.Contains(err.Error(), "QuoteService 未初始化") {
		t.Fatalf("expected uninitialized error, got %v", err)
	}
}

func TestRenderGetStockInfo_UsesQuoteService(t *testing.T) {
	stub := &stubQuoteService{quotes: []marketdata.Quote{{
		Code: "sh600519", Name: "贵州茅台",
		Price: 1600, Open: 1590, High: 1610, Low: 1580,
		Volume: 10000, Amount: 1.6e7, PreClose: 1595,
		ChangeValue: 5, ChangePercent: 0.31,
		Date: "2026-07-27", Time: "09:35:00",
	}}}
	out := RenderGetStockInfo(stub, []string{"sh600519"})
	if len(stub.calls) != 1 {
		t.Fatalf("expected one GetQuotes call, got %+v", stub.calls)
	}
	if !strings.Contains(out, "贵州茅台") || !strings.Contains(out, "1600.00") {
		t.Fatalf("markdown missing quote fields: %s", out)
	}
	if !strings.Contains(out, "2026-07-27") {
		t.Fatalf("markdown missing timestamp: %s", out)
	}
	for _, forbidden := range []string{"成本价", "持仓数量", "盈亏", "关注价"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("markdown leaked %q: %s", forbidden, out)
		}
	}
}

func TestRenderGetStockInfo_NoData(t *testing.T) {
	stub := &stubQuoteService{err: marketdata.ErrNoData}
	out := RenderGetStockInfo(stub, []string{"sz000858"})
	if !strings.Contains(out, "未找到股票信息") {
		t.Fatalf("expected not-found message, got %s", out)
	}
}

func TestGetQuoteService_FakeInjectNoDB(t *testing.T) {
	// 勿在 SetQuoteService 前调用 GetQuoteService：会触发 wire 工厂 → NewStockDataApi → DB。
	t.Cleanup(func() {
		data.SetQuoteService(nil)
	})

	stub := &stubQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Name: "平安银行", Price: 1, Open: 1, High: 1, Low: 1,
		Date: "2026-07-27", Time: "10:00:00",
	}}}
	data.SetQuoteService(stub)

	got := data.GetQuoteService()
	if got == nil {
		t.Fatal("GetQuoteService returned nil after SetQuoteService")
	}
	out, err := QueryStockPriceInfoJSON(got, []string{"sz000001"})
	if err != nil {
		t.Fatalf("injected service failed: %v", err)
	}
	if !strings.Contains(out, "平安银行") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestLegacyQuoteAdapter_DelegatesLegacyAPI_NoDB(t *testing.T) {
	fake := &fakeRealtimeFetcher{result: &[]data.StockInfo{{
		Code: "sz000001", Name: "平安银行",
		Price: "11.2", Open: "11.0", High: "11.5", Low: "10.9",
		Volume: "1000000", Amount: "20000000",
		PreClose: "11.1", Date: "2026-07-27", Time: "14:30:00",
		CostPrice: 99, Profit: 88, // must NOT map into Quote
	}}}
	var svc marketdata.QuoteService = adapter.NewLegacyQuoteAdapterWith(fake)
	quotes, err := svc.GetQuotes([]string{"sz000001"})
	if err != nil {
		t.Fatalf("GetQuotes: %v", err)
	}
	if len(quotes) != 1 {
		t.Fatalf("want 1 quote, got %d", len(quotes))
	}
	q := quotes[0]
	if q.Price != 11.2 || q.Open != 11.0 || q.High != 11.5 || q.Low != 10.9 {
		t.Fatalf("OHLC mismatch: %+v", q)
	}
	if q.Volume != 1e6 || q.Amount != 2e7 {
		t.Fatalf("volume/amount mismatch: %+v", q)
	}
	if q.Date != "2026-07-27" || q.Time != "14:30:00" {
		t.Fatalf("timestamp mismatch: %+v", q)
	}
	if q.FetchedAt.IsZero() {
		t.Fatal("FetchedAt should be set")
	}
	if len(fake.calls) != 1 || fake.calls[0][0] != "sz000001" {
		t.Fatalf("adapter did not delegate expected codes: %+v", fake.calls)
	}
}

type fakeRealtimeFetcher struct {
	result *[]data.StockInfo
	err    error
	calls  [][]string
}

func (f *fakeRealtimeFetcher) GetStockCodeRealTimeData(stockCodes ...string) (*[]data.StockInfo, error) {
	f.calls = append(f.calls, append([]string(nil), stockCodes...))
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestQueryStockPriceInfoJSON_PropagatesServiceError(t *testing.T) {
	stub := &stubQuoteService{err: errors.New("upstream boom")}
	_, err := QueryStockPriceInfoJSON(stub, []string{"sz000001"})
	if err == nil || err.Error() != "upstream boom" {
		t.Fatalf("expected upstream error, got %v", err)
	}
}
