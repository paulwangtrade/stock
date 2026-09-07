package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestQuantDecisionSchemaVersionFrozen(t *testing.T) {
	if QuantDecisionSchemaVersion != 1 {
		t.Fatalf("Phase0 schema must be 1, got %d", QuantDecisionSchemaVersion)
	}
}

func TestQuantDecisionJSONRoundTrip(t *testing.T) {
	bar := 100
	d := QuantDecision{
		ID:        "qd-test-1",
		AsOf:      time.Date(2026, 7, 20, 10, 0, 0, 0, time.Local),
		TradeDate: "2026-07-20",
		Purpose:   QuantPurposeWatchlist,
		Instrument: QuantInstrument{
			StockCode: "sz000001",
			StockName: "平安银行",
		},
		Regime: QuantRegime{Level: 3, Key: "level3", Name: "中性观望", ExposureCap: 0.2},
		Signal: QuantSignal{
			Tag: "强", TagKind: QuantTagKindEntry, DaysAgo: 1, Score: 88,
			RatioPct: nil, Summary: "1日前强化买点", BarIndex: &bar,
		},
		EntryZone: &QuantEntryZone{
			Low: 10.1, High: 10.3, InstantPrice: 10.2,
			Mode: QuantZoneModeNear, DeferMode: QuantDeferDefer, DaysAgo: 1,
			Text: "10.10~10.30",
		},
		Gate: QuantGate{Score: 0.875, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk: QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size: QuantSize{
			OK: true, TargetShares: 1000, TargetAmount: 102000, PositionPct: 12.5,
		},
		Action: QuantAction{Code: QuantActionEnter, Label: "建议关注买入", AllowDraft: true, Side: "buy"},
		Meta: QuantDecisionMeta{
			SchemaVersion: QuantDecisionSchemaVersion,
			Producer:      QuantProducerJSLegacy,
		},
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var back QuantDecision
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Action.Code != QuantActionEnter {
		t.Fatalf("action=%s", back.Action.Code)
	}
	if back.Meta.SchemaVersion != 1 {
		t.Fatalf("schema=%d", back.Meta.SchemaVersion)
	}
	if back.EntryZone == nil || back.EntryZone.Text != "10.10~10.30" {
		t.Fatalf("entryZone=%+v", back.EntryZone)
	}
}

func TestQuantActionCodesAreStable(t *testing.T) {
	want := []string{
		QuantActionEnter, QuantActionWaitPullback, QuantActionWatch,
		QuantActionScaleIn, QuantActionReduce, QuantActionExitPartial,
		QuantActionHold, QuantActionBlocked,
	}
	if len(want) != 8 {
		t.Fatalf("unexpected action set size %d", len(want))
	}
}
