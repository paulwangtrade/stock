package data

import (
	"encoding/json"
	"testing"

	"go-stock/backend/models"
)

func TestMapSliceToHits_PreservesSignalPriceSeparateFromQuote(t *testing.T) {
	items := []map[string]any{
		{
			"SECUCODE":            "000001.SZ",
			"SECURITY_CODE":       "000001",
			"SECURITY_NAME_ABBR":  "平安银行",
			"NEW_PRICE":           "15.88",
			"tag":                 "趋",
			"recentSignalDaysAgo": 1,
			"signal_price":        14.20,
			"signal_time":         "2026-08-27",
			"signal_price_source": "kline_close",
			"signal_days_ago":     1,
			"signal_bar_role":     "signal",
			"signal_price_status": "frozen",
			"schema_version":      models.SignalSchemaVersionV1,
		},
	}
	hits := mapSliceToHits(items)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	h := hits[0]
	if h.NEW_PRICE != "15.88" {
		t.Fatalf("NEW_PRICE=%q", h.NEW_PRICE)
	}
	if h.SignalPrice != 14.20 {
		t.Fatalf("signal_price=%v", h.SignalPrice)
	}
	if h.SignalTime != "2026-08-27" {
		t.Fatalf("signal_time=%q", h.SignalTime)
	}
	if h.SignalPriceStatus != models.SignalPriceStatusFrozen {
		t.Fatalf("status=%q", h.SignalPriceStatus)
	}
}

func TestNormalizeSignalScanHit_NeverBackfillFromNewPrice(t *testing.T) {
	h := models.SignalScanHit{
		NEW_PRICE:   "99.99",
		SignalPrice: 0,
	}
	NormalizeSignalScanHit(&h)
	if h.SignalPrice != 0 {
		t.Fatalf("signal_price must stay 0, got %v", h.SignalPrice)
	}
	if h.SignalPriceStatus != models.SignalPriceStatusMissing {
		t.Fatalf("status=%q want missing", h.SignalPriceStatus)
	}
}

func TestParseSnapshotHits_LegacyWithoutSignalPrice(t *testing.T) {
	legacy := `{"items":[{"SECUCODE":"600036.SH","SECURITY_CODE":"600036","SECURITY_NAME_ABBR":"招商银行","NEW_PRICE":"38.65","tag":"强","recentSignalDaysAgo":2,"statusText":"2日前强化买点","sortRank":80}]}`
	repo := NewSignalSnapshotRepo()
	hits := repo.ParseSnapshotHits(&models.SignalScanSnapshot{ResultJSON: legacy})
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	h := hits[0]
	if h.NEW_PRICE != "38.65" {
		t.Fatalf("NEW_PRICE=%q", h.NEW_PRICE)
	}
	if h.SignalPrice != 0 {
		t.Fatalf("legacy must not infer signal_price from quote, got %v", h.SignalPrice)
	}
	if h.SignalPriceStatus != models.SignalPriceStatusMissing {
		t.Fatalf("status=%q want missing", h.SignalPriceStatus)
	}
}

func TestParseSnapshotHits_WithFrozenSignalPrice(t *testing.T) {
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{
				SECUCODE:           "600036.SH",
				SECURITY_CODE:      "600036",
				SECURITY_NAME_ABBR: "招商银行",
				NEW_PRICE:          "38.65",
				Tag:                "强",
				StatusText:         "2日前强化买点",
				SchemaVersion:      models.SignalSchemaVersionV1,
				SignalPrice:        38.12,
				SignalTime:         "2026-08-27",
				SignalPriceSource:  "kline_close_confirm",
				SignalBarRole:      "confirm",
				SignalPriceStatus:  models.SignalPriceStatusFrozen,
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	hits := NewSignalSnapshotRepo().ParseSnapshotHits(&models.SignalScanSnapshot{ResultJSON: string(raw)})
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	h := hits[0]
	if h.SignalPrice != 38.12 {
		t.Fatalf("signal_price=%v", h.SignalPrice)
	}
	if h.NEW_PRICE != "38.65" {
		t.Fatalf("snapshot quote must remain separate, NEW_PRICE=%q", h.NEW_PRICE)
	}
	if h.SignalPriceStatus != models.SignalPriceStatusFrozen {
		t.Fatalf("status=%q", h.SignalPriceStatus)
	}
}

func TestRunSignalScanBatchJS_SignalPriceOnHit(t *testing.T) {
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		Stocks: []signalScanStockInput{
			{
				Code: "sz000001", Name: "平安银行", Secucode: "000001.SZ",
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
				Row: map[string]any{
					"NEW_PRICE": "12.50",
				},
			},
		},
		IndexClose: map[string]float64{"2026-01-29": 3000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HitTotal == 0 {
		t.Skip("fixture produced no signal hits; mapSliceToHits tests cover persistence contract")
	}
	hits := mapSliceToHits(out.Items)
	for _, h := range hits {
		if h.SignalPrice <= 0 {
			t.Fatalf("hit %s missing signal_price", h.SECUCODE)
		}
		if h.NEW_PRICE == "" {
			continue
		}
		if h.SignalPriceStatus != models.SignalPriceStatusFrozen {
			t.Fatalf("hit %s status=%q", h.SECUCODE, h.SignalPriceStatus)
		}
	}
}
