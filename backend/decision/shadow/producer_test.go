package shadow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"go-stock/backend/models"
)

func sampleBuyReadyInput() Input {
	return Input{
		Code:       "sz000001",
		Name:       "PingAnBank",
		Purpose:    models.QuantPurposeWatchlist,
		TradeDate:  "2026-07-21",
		AsOf:       "2026-07-21T00:00:00.000Z",
		Tag:        "\u5f3a", // ?
		DaysAgo:    0,
		StatusText: "today-strong",
		EntryZone: &models.QuantEntryZone{
			Low: 10, High: 10.2, InstantPrice: 10.1,
			Mode: models.QuantZoneModeNear, DeferMode: models.QuantDeferSame,
			DaysAgo: 0, Text: "10.00~10.20", Note: "near",
		},
		HasGate: true,
		Gate: models.QuantGate{
			Score: 0.875, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85,
			Items: []models.QuantGateItem{{ID: "buy_signal", Label: "buy_signal", Passed: true}},
		},
		HasSize: true,
		Size: models.QuantSize{
			OK: true, StopPrice: 9.85, TargetShares: 1000, AddShares: 1000,
			PositionPct: 12.5, Reason: "buy-1000",
		},
		MarketLevel:    3,
		ChecklistReady: true,
		ExistingVolume: 0,
	}
}

func TestBuildShadowDecision_ProducerAndSchema(t *testing.T) {
	d, err := BuildShadowDecision(sampleBuyReadyInput())
	if err != nil {
		t.Fatal(err)
	}
	if d.Meta.Producer != models.QuantProducerGoEngine {
		t.Fatalf("producer=%s", d.Meta.Producer)
	}
	if d.Meta.SchemaVersion != models.QuantDecisionSchemaVersion {
		t.Fatalf("schema=%d", d.Meta.SchemaVersion)
	}
	if d.Action.Code != models.QuantActionEnter {
		t.Fatalf("action.code=%s", d.Action.Code)
	}
	if d.Action.Label != "\u53ef\u4e70" { // ??
		t.Fatalf("action.label=%q", d.Action.Label)
	}
	if !d.Action.AllowDraft {
		t.Fatal("allowDraft want true")
	}
	if d.ID == "" {
		t.Fatal("id required")
	}
}

func TestShadowJSON_Output(t *testing.T) {
	d, err := BuildShadowDecision(sampleBuyReadyInput())
	if err != nil {
		t.Fatal(err)
	}
	b, err := MarshalDecisionJSON(d)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatal("invalid json")
	}
	var back models.QuantDecision
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Meta.Producer != models.QuantProducerGoEngine {
		t.Fatalf("producer=%s", back.Meta.Producer)
	}
	path := filepath.Join(testdataDir(t), "go_engine_buy_ready.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompare_SameShadowNoDiff(t *testing.T) {
	d, err := BuildShadowDecision(sampleBuyReadyInput())
	if err != nil {
		t.Fatal(err)
	}
	clone := *d
	clone.ID = "other-id"
	clone.AsOf = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	clone.Meta.Producer = models.QuantProducerJSLegacy
	r := CompareDecisions(d, &clone)
	if !r.Equal {
		t.Fatalf("want equal, got %s diffs=%v", r.Summary, r.Diffs)
	}
}

func TestCompare_ActionDiffDetectable(t *testing.T) {
	left, err := BuildShadowDecision(sampleBuyReadyInput())
	if err != nil {
		t.Fatal(err)
	}
	left.Meta.Producer = models.QuantProducerJSLegacy

	right := *left
	right.Meta.Producer = models.QuantProducerGoEngine
	right.Action = models.QuantAction{
		Code: models.QuantActionWaitPullback, Label: "\u7b49\u56de\u8e29", Side: "none", AllowDraft: false, // ???
	}
	r := CompareDecisions(left, &right)
	if r.Equal {
		t.Fatal("want action diff")
	}
	actionSlice := r.BySlice["action"]
	if actionSlice.Equal {
		t.Fatal("action slice should differ")
	}
	foundLabel := false
	for _, d := range r.Diffs {
		if d.Path == "action.label" {
			foundLabel = true
			if d.Left != "\u53ef\u4e70" || d.Right != "\u7b49\u56de\u8e29" {
				t.Fatalf("label diff=%+v", d)
			}
		}
	}
	if !foundLabel {
		t.Fatalf("missing action.label diff: %+v", r.Diffs)
	}
	b, err := MarshalReportJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testdataDir(t), "shadow_diff_report_action.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompare_EntryZoneDiffLocatable(t *testing.T) {
	left, err := BuildShadowDecision(sampleBuyReadyInput())
	if err != nil {
		t.Fatal(err)
	}
	left.Meta.Producer = models.QuantProducerJSLegacy
	right := *left
	right.Meta.Producer = models.QuantProducerGoEngine
	z := *right.EntryZone
	z.Mode = models.QuantZoneModeAbove
	z.DeferMode = models.QuantDeferWait
	right.EntryZone = &z
	r := CompareDecisions(left, &right)
	zoneSlice := r.BySlice["entryZone"]
	if zoneSlice.Equal {
		t.Fatal("entryZone should differ")
	}
	ok := false
	for _, d := range r.Diffs {
		if d.Path == "entryZone.mode" && d.Slice == "entryZone" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("want entryZone.mode diff, got %+v", r.Diffs)
	}
}

func TestShadow_DoesNotTouchTradePlanTypes(t *testing.T) {
	d, err := BuildShadowDecision(sampleBuyReadyInput())
	if err != nil {
		t.Fatal(err)
	}
	if d.Meta.TradePlanID != 0 {
		t.Fatal("shadow must not bind TradePlanID")
	}
}

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Join(filepath.Dir(file), "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
