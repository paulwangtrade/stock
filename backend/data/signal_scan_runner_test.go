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
