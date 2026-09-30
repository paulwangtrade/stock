package data

import (
	"testing"
)

func TestRunSignalScanBatchJS_Empty(t *testing.T) {
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		Stocks:     []signalScanStockInput{},
		IndexClose: map[string]float64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal != 0 {
		t.Fatalf("expected 0 hits, got %d", out.HitTotal)
	}
}

func TestRunSignalScanBatchJS_ObservationBreakoutDoesNotUseIcePath(t *testing.T) {
	n := 21
	closes := make([]float64, n)
	opens := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = 10
		opens[i] = 10
		highs[i] = 10
		lows[i] = 9.9
		volumes[i] = 100
	}
	closes[n-1] = 11
	opens[n-1] = 10.2
	highs[n-1] = 12
	lows[n-1] = 10.1
	volumes[n-1] = 200

	stock := signalScanStockInput{
		Code: "sh600036", Name: "招商银行",
		Closes: closes, Opens: opens, Highs: highs, Lows: lows, Volumes: volumes,
	}
	hit, err := RunSignalScanBatchJS(signalScanBatchInput{
		Stocks:     []signalScanStockInput{stock},
		StrategyID: StrategyIDVolBreakout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hit.HitTotal != 1 {
		t.Fatalf("expected 1 observation hit, got %d", hit.HitTotal)
	}
	if hit.Items[0]["strategy_id"] != StrategyIDVolBreakout {
		t.Fatalf("strategy_id = %v", hit.Items[0]["strategy_id"])
	}
	if hit.Items[0]["tag"] != "买" {
		t.Fatalf("tag = %v", hit.Items[0]["tag"])
	}

	quiet := stock
	quiet.Volumes = append([]float64(nil), volumes...)
	quiet.Volumes[n-1] = 100
	miss, err := RunSignalScanBatchJS(signalScanBatchInput{
		Stocks:     []signalScanStockInput{quiet},
		StrategyID: StrategyIDVolBreakout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if miss.HitTotal != 0 {
		t.Fatalf("expected volume fail-closed, got %d", miss.HitTotal)
	}
}

func TestRunSignalScanBatchJS_VMInit(t *testing.T) {
	_, err := RunSignalScanBatchJS(signalScanBatchInput{
		Stocks: []signalScanStockInput{
			{
				Code: "sz000001", Name: "平安银行",
				Closes: []float64{10, 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9,
					11, 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8, 11.9, 12},
				Opens:   []float64{10, 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9, 11, 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8, 11.9, 12},
				Highs:   []float64{10, 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9, 11, 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8, 11.9, 12},
				Lows:    []float64{10, 10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9, 11, 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8, 11.9, 12},
				Volumes: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
				DayKeys: []string{"2026-01-01", "2026-01-02", "2026-01-03", "2026-01-06", "2026-01-07",
					"2026-01-08", "2026-01-09", "2026-01-10", "2026-01-13", "2026-01-14",
					"2026-01-15", "2026-01-16", "2026-01-17", "2026-01-20", "2026-01-21",
					"2026-01-22", "2026-01-23", "2026-01-24", "2026-01-27", "2026-01-28", "2026-01-29"},
				LastBarIndex: 20,
			},
		},
		IndexClose: map[string]float64{"2026-01-29": 3000},
	})
	if err != nil {
		t.Fatal(err)
	}
}
