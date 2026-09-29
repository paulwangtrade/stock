package data

import "testing"

func TestSignalScanBatchSpan_PerStockEnginesStayChunked(t *testing.T) {
	for _, id := range []string{extMaTrendV1StrategyID, extBreakoutV1StrategyID} {
		if id == signalScanDefaultStrategyID || id == extXsMomV1StrategyID {
			t.Fatalf("id %s collides with default or momentum", id)
		}
		if signalScanBatchSpan(id, 5000) != signalScanJSChunkSize {
			t.Fatalf("%s span = %d, want chunked", id, signalScanBatchSpan(id, 5000))
		}
	}
}

func TestRunSignalScanBatchJS_MaTrendRule(t *testing.T) {
	up := xsMomStock("sz000001", "升", xsMomSeries(80, 10, 20))
	flat := xsMomStock("sz000002", "平", xsMomSeries(80, 10, 10))
	short := xsMomStock("sz000003", "短", xsMomSeries(30, 10, 20))
	bounce := xsMomStock("sz000004", "反抽", maTrendBounceCloses())

	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extMaTrendV1StrategyID,
		Stocks:     []signalScanStockInput{up, flat, short, bounce, {Code: "sz000005", Name: "空"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 1 || len(out.Items) != 1 {
		t.Fatalf("hits=%d, want only the rising name", out.HitTotal)
	}
	if out.Items[0]["SECUCODE"] != "sz000001" || out.Items[0]["tag"] != "MA_TREND" {
		t.Fatalf("item = %v %v", out.Items[0]["SECUCODE"], out.Items[0]["tag"])
	}
}

func TestRunSignalScanBatchJS_BreakoutPriorHigh(t *testing.T) {
	hit := breakoutStock("sz000001", "破", 30, 10, 12)
	inside := breakoutStock("sz000002", "内", 30, 10, 10)
	capped := breakoutStock("sz000003", "未破高", 30, 15, 12)
	// prior window uses highs; last close 12 does not clear prior high 15.
	capped.Closes[len(capped.Closes)-1] = 12
	short := breakoutStock("sz000004", "短", 10, 10, 20)

	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extBreakoutV1StrategyID,
		Stocks:     []signalScanStockInput{hit, inside, capped, short},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 1 || out.Items[0]["tag"] != "BREAKOUT_N" || out.Items[0]["SECUCODE"] != "sz000001" {
		t.Fatalf("items=%v", tagsOf(out))
	}
}

func TestRunSignalScanBatchJS_ObserveEnginesDistinctAndIsolated(t *testing.T) {
	// Last bar jumps, so the same name can satisfy both MA20/MA60 and a 20-day high break.
	stock := xsMomStock("sz000008", "双条件", xsMomSeries(80, 10, 20))
	ma, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extMaTrendV1StrategyID,
		Stocks:     []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	bo, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extBreakoutV1StrategyID,
		Stocks:     []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ma.HitTotal != 1 || ma.Items[0]["tag"] != "MA_TREND" {
		t.Fatalf("ma = %v", tagsOf(ma))
	}
	if bo.HitTotal != 1 || bo.Items[0]["tag"] != "BREAKOUT_N" {
		t.Fatalf("breakout = %v", tagsOf(bo))
	}
	for _, sid := range []string{"", "default"} {
		ice, err := RunSignalScanBatchJS(signalScanBatchInput{
			StrategyID:       sid,
			SignalParamsJSON: `{"scanMode":"ext_ma_trend_v1"}`,
			Stocks:           []signalScanStockInput{stock},
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range ice.Items {
			tag, _ := item["tag"].(string)
			if tag == "MA_TREND" || tag == "BREAKOUT_N" || tag == "XS_MOM_TOP" {
				t.Fatalf("strategy %q leaked tag %s", sid, tag)
			}
		}
	}
}

func TestRunSignalScanBatchJS_PlannedEngineEmitsNothing(t *testing.T) {
	stock := xsMomStock("sz000008", "双条件", xsMomSeries(80, 10, 20))
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: "ext_vol_mom_v1",
		Stocks:     []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 0 || len(out.Items) != 0 {
		t.Fatalf("planned hits=%d tags=%v", out.HitTotal, tagsOf(out))
	}
}

func TestRunSignalScanBatchJS_ExplicitObserveScanMode(t *testing.T) {
	stock := xsMomStock("sz000008", "双条件", xsMomSeries(80, 10, 20))
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       "lab_observe",
		SignalParamsJSON: `{"scanMode":"ext_breakout_v1","breakoutLookback":20}`,
		Stocks:           []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 1 || out.Items[0]["tag"] != "BREAKOUT_N" {
		t.Fatalf("explicit mode = %v", tagsOf(out))
	}
}

func TestRunSignalScanBatchJS_BreakoutLookbackOverride(t *testing.T) {
	stock := breakoutStock("sz000001", "短样本", 12, 10, 13)
	miss, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID: extBreakoutV1StrategyID,
		Stocks:     []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if miss.HitTotal != 0 {
		t.Fatalf("default N=20 should skip 12 bars, hits=%d", miss.HitTotal)
	}
	hit, err := RunSignalScanBatchJS(signalScanBatchInput{
		StrategyID:       extBreakoutV1StrategyID,
		SignalParamsJSON: `{"breakoutLookback":5}`,
		Stocks:           []signalScanStockInput{stock},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit.HitTotal != 1 || hit.Items[0]["tag"] != "BREAKOUT_N" {
		t.Fatalf("lookback 5 hits=%d", hit.HitTotal)
	}
}

// maTrendBounceCloses is a rally that dies: close can sit above MA20 while MA20 stays below MA60.
func maTrendBounceCloses() []float64 {
	out := make([]float64, 80)
	for i := 0; i < 60; i++ {
		out[i] = 30
	}
	for i := 60; i < 79; i++ {
		out[i] = 10
	}
	out[79] = 12
	return out
}

func breakoutStock(code, name string, n int, prior, last float64) signalScanStockInput {
	closes := make([]float64, n)
	highs := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = prior
		highs[i] = prior
	}
	if n > 0 {
		closes[n-1] = last
		highs[n-1] = last
	}
	stock := xsMomStock(code, name, closes)
	stock.Highs = highs
	return stock
}

func tagsOf(out *signalScanBatchOutput) []any {
	if out == nil {
		return nil
	}
	tags := make([]any, 0, len(out.Items))
	for _, item := range out.Items {
		tags = append(tags, item["tag"])
	}
	return tags
}
