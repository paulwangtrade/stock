package data

import (
	"fmt"
	"testing"
)

func TestSignalScanBatchSpan_DefaultStaysChunked(t *testing.T) {
	if signalScanBatchSpan("default", 5000) != signalScanJSChunkSize {
		t.Fatalf("default span = %d", signalScanBatchSpan("default", 5000))
	}
	if signalScanBatchSpan("", 5000) != signalScanJSChunkSize {
		t.Fatalf("empty strategy span = %d", signalScanBatchSpan("", 5000))
	}
	if signalScanBatchSpan("stock_strategy:4", 5000) != signalScanJSChunkSize {
		t.Fatalf("other strategy span = %d", signalScanBatchSpan("stock_strategy:4", 5000))
	}
}

func TestSignalScanBatchSpan_XsMomRanksFullPreparedSet(t *testing.T) {
	if extXsMomV1StrategyID == signalScanDefaultStrategyID {
		t.Fatal("momentum strategy id must not be default")
	}
	if signalScanBatchSpan(extXsMomV1StrategyID, 5000) != 5000 {
		t.Fatalf("xsmom span = %d, want full prepared set", signalScanBatchSpan(extXsMomV1StrategyID, 5000))
	}
	// prepared=0 must not produce span 0 (that would spin the chunk loop).
	if signalScanBatchSpan(extXsMomV1StrategyID, 0) != signalScanJSChunkSize {
		t.Fatalf("empty prepared span = %d", signalScanBatchSpan(extXsMomV1StrategyID, 0))
	}
}

func xsMomSeries(n int, prev, last float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = prev
	}
	if n > 0 {
		out[n-1] = last
	}
	return out
}

func xsMomStock(code, name string, closes []float64) signalScanStockInput {
	n := len(closes)
	flat := make([]float64, n)
	days := make([]string, n)
	vols := make([]float64, n)
	for i := 0; i < n; i++ {
		flat[i] = closes[i]
		vols[i] = 1
		days[i] = fmt.Sprintf("2026-04-%02d", (i%28)+1)
	}
	last := n - 1
	if last < 0 {
		last = 0
	}
	return signalScanStockInput{
		Code: code, Name: name, Secucode: code,
		Closes: closes, Opens: flat, Highs: flat, Lows: flat, Volumes: vols,
		DayKeys: days, LastBarIndex: last,
	}
}

func TestRunSignalScanBatchJS_XsMomTopNAndSkips(t *testing.T) {
	const n = 41
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       extXsMomV1StrategyID,
		SignalParamsJSON: `{"xsmomLookback":20,"xsmomTopN":2}`,
		Stocks: []signalScanStockInput{
			xsMomStock("sz000003", "高", xsMomSeries(n, 10, 15)),   // +50%
			xsMomStock("sz000001", "中", xsMomSeries(n, 10, 12)),   // +20%
			xsMomStock("sz000002", "低", xsMomSeries(n, 10, 10.5)), // +5%
			xsMomStock("sz000004", "跌", xsMomSeries(n, 10, 8)),    // negative
			xsMomStock("sz000005", "短", xsMomSeries(8, 10, 20)),   // < lookback
			xsMomStock("sz000006", "零", xsMomSeries(n, 10, 0)),    // bad price
			{Code: "sz000007", Name: "空"},                         // no bars
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 2 || len(out.Items) != 2 {
		t.Fatalf("hits=%d items=%d, want 2", out.HitTotal, len(out.Items))
	}
	if got := out.Items[0]["SECUCODE"]; got != "sz000003" {
		t.Fatalf("rank1 = %v, want sz000003", got)
	}
	if got := out.Items[1]["SECUCODE"]; got != "sz000001" {
		t.Fatalf("rank2 = %v, want sz000001", got)
	}
	for _, item := range out.Items {
		if item["tag"] != "XS_MOM_TOP" {
			t.Fatalf("tag = %v", item["tag"])
		}
		if item["signal_price"] == nil {
			t.Fatal("missing signal_price")
		}
	}
}

func TestRunSignalScanBatchJS_XsMomTieBreakByCode(t *testing.T) {
	const n = 41
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       extXsMomV1StrategyID,
		SignalParamsJSON: `{"xsmomTopN":2}`,
		Stocks: []signalScanStockInput{
			xsMomStock("sz000002", "乙", xsMomSeries(n, 10, 12)),
			xsMomStock("sz000001", "甲", xsMomSeries(n, 10, 12)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("hits=%d", len(out.Items))
	}
	if out.Items[0]["SECUCODE"] != "sz000001" || out.Items[1]["SECUCODE"] != "sz000002" {
		t.Fatalf("tie order = %v, %v", out.Items[0]["SECUCODE"], out.Items[1]["SECUCODE"])
	}
}

func TestRunSignalScanBatchJS_DefaultStrategyIgnoresMomentumMode(t *testing.T) {
	const n = 41
	stocks := []signalScanStockInput{
		xsMomStock("sz000003", "高", xsMomSeries(n, 10, 15)),
		xsMomStock("sz000001", "中", xsMomSeries(n, 10, 12)),
	}
	iceTags := map[string]bool{"强": true, "趋": true, "转": true, "突": true, "弹": true, "买": true}
	for _, sid := range []string{"", "default"} {
		out, err := RunSignalScanBatchJS(signalScanBatchInput{
			StrategyID:       sid,
			SignalParamsJSON: `{"scanMode":"xsmom","xsmomTopN":1}`,
			Stocks:           stocks,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range out.Items {
			tag, _ := item["tag"].(string)
			if tag == "XS_MOM_TOP" || !iceTags[tag] {
				t.Fatalf("strategy %q emitted tag %q", sid, tag)
			}
		}
	}
}

func TestRunSignalScanBatchJS_ExplicitScanModeOnNonDefault(t *testing.T) {
	const n = 41
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       "lab_observe",
		SignalParamsJSON: `{"scanMode":"ext_xsmom_v1","xsmomTopN":1}`,
		Stocks: []signalScanStockInput{
			xsMomStock("sz000003", "高", xsMomSeries(n, 10, 15)),
			xsMomStock("sz000001", "中", xsMomSeries(n, 10, 12)),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 1 || out.Items[0]["tag"] != "XS_MOM_TOP" || out.Items[0]["SECUCODE"] != "sz000003" {
		t.Fatalf("explicit mode result = %+v", out.Items)
	}
}

func TestRunSignalScanBatchJS_XsMomRsiMaxIsOptIn(t *testing.T) {
	const n = 41
	stock := xsMomStock("sz000003", "高", xsMomSeries(n, 10, 15))
	open, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extXsMomV1StrategyID,
		Stocks:     []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if open.HitTotal != 1 {
		t.Fatalf("default gate hits=%d, want 1 (RSI must not veto momentum)", open.HitTotal)
	}
	closed, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       extXsMomV1StrategyID,
		SignalParamsJSON: `{"xsmomRsiMax":85}`,
		Stocks:           []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if closed.HitTotal != 0 {
		t.Fatalf("rsi max hits=%d, want 0", closed.HitTotal)
	}
}

func TestRunSignalScanBatchJS_XsMomLookbackOverride(t *testing.T) {
	short := xsMomStock("sz000001", "短", xsMomSeries(15, 10, 13))
	miss, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extXsMomV1StrategyID,
		Stocks:     []signalScanStockInput{short},
	})
	if err != nil {
		t.Fatal(err)
	}
	if miss.HitTotal != 0 {
		t.Fatalf("default lookback 20 should skip 15 bars, hits=%d", miss.HitTotal)
	}
	hit, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       extXsMomV1StrategyID,
		SignalParamsJSON: `{"xsmomLookback":10}`,
		Stocks:           []signalScanStockInput{short},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit.HitTotal != 1 || hit.Items[0]["tag"] != "XS_MOM_TOP" {
		t.Fatalf("lookback 10 hits=%d", hit.HitTotal)
	}
}
