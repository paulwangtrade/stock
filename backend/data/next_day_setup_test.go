package data

import (
	"encoding/json"
	"strings"
	"testing"

	"go-stock/backend/models"
)

func TestMapSliceToSetups_ForcesObservationAndFailClosed(t *testing.T) {
	rows := mapSliceToSetups([]map[string]any{
		{
			"SECUCODE": "000001.SZ", "SECURITY_NAME_ABBR": "平安银行",
			"engine": "breakout_high", "tag": "突", "priceMode": "price",
			"triggerPrice": 10.25, "closeT": 10.1, "distancePct": 0.014,
			"confirmed": true, "orderIntent": true, "gapText": "放量未知",
		},
		{
			"SECUCODE": "000002.SZ", "engine": "trend_next_day", "tag": "趋",
			"priceMode": "gap", "triggerPrice": 99.0, "gapText": "",
		},
		{
			"SECUCODE": "000003.SZ", "engine": "breakout_high", "tag": "突",
			"priceMode": "price", "triggerPrice": 0,
		},
		{"engine": "breakout_high", "tag": "突"},
	})
	if len(rows) != 3 {
		t.Fatalf("kept %d rows, want 3", len(rows))
	}
	if rows[0].Confirmed || rows[0].OrderIntent || !rows[0].ObservationOnly {
		t.Fatalf("observation flags = %+v", rows[0])
	}
	if rows[0].Disclaimer != nextDaySetupDisclaimer {
		t.Fatalf("disclaimer=%q", rows[0].Disclaimer)
	}
	if rows[0].TriggerPrice == nil || *rows[0].TriggerPrice != 10.25 {
		t.Fatalf("trigger=%v", rows[0].TriggerPrice)
	}
	if rows[1].PriceMode != "gap" || rows[1].TriggerPrice != nil {
		t.Fatalf("gap row leaked a price: %+v", rows[1])
	}
	if rows[1].GapText != nextDaySetupUnavailable {
		t.Fatalf("gap text=%q", rows[1].GapText)
	}
	if rows[2].PriceMode != "unavailable" || rows[2].TriggerPrice != nil {
		t.Fatalf("zero price should fail closed: %+v", rows[2])
	}
}

func TestParseSnapshotHits_IgnoresNextDaySetups(t *testing.T) {
	raw := `{
		"items":[{"SECUCODE":"600036.SH","SECURITY_CODE":"600036","SECURITY_NAME_ABBR":"招商银行","tag":"强","statusText":"强化买点"}],
		"hitTotal":1,
		"nextDaySetups":[{"SECUCODE":"000001.SZ","engine":"breakout_high","tag":"突","priceMode":"price","triggerPrice":11.2,"confirmed":false,"orderIntent":false,"observationOnly":true,"disclaimer":"若触及可能形成，不保证，非买卖指令"}]
	}`
	hits := NewSignalSnapshotRepo().ParseSnapshotHits(&models.SignalScanSnapshot{ResultJSON: raw})
	if len(hits) != 1 {
		t.Fatalf("hits=%d, setups must not become confirmed hits", len(hits))
	}
	if hits[0].Tag != "强" || hits[0].SECUCODE != "600036.SH" {
		t.Fatalf("hit=%+v", hits[0])
	}
	var payload models.SignalScanResultPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.NextDaySetups) != 1 || payload.NextDaySetups[0].Tag != "突" {
		t.Fatalf("setup payload=%+v", payload.NextDaySetups)
	}
	if payload.NextDaySetups[0].OrderIntent || payload.NextDaySetups[0].Confirmed {
		t.Fatal("stored flags must stay non-order")
	}
}

func TestRunSignalScanBatchJS_SetupsSeparateFromHits(t *testing.T) {
	n := 25
	closes := make([]float64, n)
	opens := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)
	days := make([]string, n)
	for i := 0; i < n; i++ {
		closes[i] = 10
		opens[i] = 9.95
		highs[i] = 10.2
		lows[i] = 9.9
		volumes[i] = 1000
		days[i] = "2026-01-01"
	}
	closes[n-1] = 10.15
	out, err := RunSignalScanBatchJS(signalScanBatchInput{
		Stocks: []signalScanStockInput{{
			Code: "sz000001", Name: "平台股", Secucode: "000001.SZ",
			Closes: closes, Opens: opens, Highs: highs, Lows: lows, Volumes: volumes, DayKeys: days,
			Row: map[string]any{"SECUCODE": "000001.SZ", "SECURITY_NAME_ABBR": "平台股"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.SetupTotal == 0 || len(out.Setups) == 0 {
		t.Fatalf("expected a breakout setup, hitTotal=%d setupTotal=%d", out.HitTotal, out.SetupTotal)
	}
	found := false
	for _, raw := range out.Setups {
		if raw["engine"] == "breakout_high" && raw["tag"] == "突" {
			found = true
			if raw["confirmed"] == true || raw["orderIntent"] == true {
				t.Fatalf("setup must not be an order: %+v", raw)
			}
			px, _ := finiteFloat(raw["triggerPrice"])
			if px <= 10.15 {
				t.Fatalf("trigger=%v", raw["triggerPrice"])
			}
		}
	}
	if !found {
		t.Fatalf("setups=%v", out.Setups)
	}
	for _, hit := range out.Items {
		blob := strings.ToLower(mustJSON(hit))
		if strings.Contains(blob, "breakout_high") || strings.Contains(blob, "orderintent") {
			t.Fatalf("confirmed hit absorbed a setup: %s", blob)
		}
	}
	mapped := mapSliceToSetups(out.Setups)
	if len(mapped) == 0 || mapped[0].OrderIntent || mapped[0].Confirmed == true {
		t.Fatalf("mapped=%+v", mapped)
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
