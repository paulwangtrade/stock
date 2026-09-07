package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"go-stock/backend/models"
)

func sampleDecision(code, tradeDate, actionCode, label string, asOf time.Time) *models.QuantDecision {
	d := &models.QuantDecision{
		AsOf:      asOf,
		TradeDate: tradeDate,
		Purpose:   models.QuantPurposeWatchlist,
		Instrument: models.QuantInstrument{
			StockCode: code,
			StockName: "T",
		},
		Regime: models.QuantRegime{Level: 3, Key: "level3", ExposureCap: 0.2},
		Signal: models.QuantSignal{Tag: "强", TagKind: models.QuantTagKindEntry},
		EntryZone: &models.QuantEntryZone{
			Low: 10, High: 10.2, InstantPrice: 10.1,
			Mode: models.QuantZoneModeNear, DeferMode: models.QuantDeferSame, Text: "10.00~10.20",
		},
		Gate:   models.QuantGate{Score: 0.9, Ready: true, RequiredPassed: true, ReadyThreshold: 0.85},
		Risk:   models.QuantRiskSlice{Passed: true, Code: "APPROVED"},
		Size:   models.QuantSize{OK: true, TargetShares: 1000, AddShares: 1000, PositionPct: 10},
		Action: models.QuantAction{Code: actionCode, Label: label, AllowDraft: true, Side: "buy"},
		Meta: models.QuantDecisionMeta{
			SchemaVersion: models.QuantDecisionSchemaVersion,
			Producer:      models.QuantProducerJSLegacy,
		},
	}
	EnsureDecisionID(d)
	return d
}

func TestTraceRoundTrip_ItemToRegistryToDecision(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)

	reg := New()
	id, err := reg.Put(d)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" || !reg.Has(id) {
		t.Fatalf("put failed id=%q", id)
	}

	items := []models.CandidatePoolItem{
		{StockCode: "sz000001", TradeDate: "2026-07-21", Rank: 1, Score: 0.91},
		{StockCode: "sh600519", TradeDate: "2026-07-21", Rank: 2, Score: 0.80},
	}
	models.EnrichCandidatePoolDecisionIDs(items, map[string]string{d.Instrument.StockCode: id})

	if items[0].Rank != 1 || items[0].Score != 0.91 {
		t.Fatalf("Score/Rank mutated during bind: %+v", items[0])
	}
	if items[0].DecisionID != id {
		t.Fatalf("bind DecisionID=%q want %q", items[0].DecisionID, id)
	}

	tr := reg.TraceItem(items[0])
	if !tr.Bound || !tr.Found || tr.Stale {
		t.Fatalf("trace not OK: %+v", tr)
	}
	if tr.Decision == nil || tr.Decision.ID != id {
		t.Fatalf("decision mismatch: %+v", tr.Decision)
	}
	if tr.Decision.Action.Code != models.QuantActionEnter {
		t.Fatalf("action=%s", tr.Decision.Action.Code)
	}
	if items[0].Rank != 1 || items[0].Score != 0.91 {
		t.Fatalf("Score/Rank mutated during trace: %+v", items[0])
	}
	if tr.ItemRank != 1 || tr.ItemScore != 0.91 {
		t.Fatalf("trace rank/score snapshot wrong: %+v", tr)
	}

	tr2 := reg.TraceItem(items[1])
	if tr2.Bound || tr2.Found || tr2.Stale {
		t.Fatalf("unbound peer unexpected: %+v", tr2)
	}
}

func TestStaleDecision_MissingFromRegistry(t *testing.T) {
	reg := New()
	item := models.CandidatePoolItem{
		StockCode:  "sz000001",
		TradeDate:  "2026-07-21",
		Rank:       3,
		Score:      0.5,
		DecisionID: "qd1:js_legacy:sz000001:2026-07-21T00:00:00Z:ENTER:可买",
	}
	tr := reg.TraceItem(item)
	if !tr.Bound || tr.Found || !tr.Stale || tr.StaleReason != StaleReasonMissingRegistry {
		t.Fatalf("expected missing_from_registry: %+v", tr)
	}
	if item.Rank != 3 || item.Score != 0.5 {
		t.Fatalf("Score/Rank mutated: %+v", item)
	}
}

func TestStaleDecision_CodeMismatch(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	reg := New()
	id, _ := reg.Put(d)

	item := models.CandidatePoolItem{
		StockCode:  "sh600519",
		TradeDate:  "2026-07-21",
		Rank:       1,
		Score:      0.99,
		DecisionID: id,
	}
	tr := reg.TraceItem(item)
	if !tr.Found || !tr.Stale || tr.StaleReason != StaleReasonCodeMismatch {
		t.Fatalf("expected code_mismatch: %+v", tr)
	}
	if item.Rank != 1 || item.Score != 0.99 {
		t.Fatalf("Score/Rank mutated: %+v", item)
	}
}

func TestStaleDecision_TradeDateMismatch(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d := sampleDecision("sz000001", "2026-07-21", models.QuantActionWatch, "观察", asOf)
	reg := New()
	id, _ := reg.Put(d)

	item := models.CandidatePoolItem{
		StockCode:  "sz000001",
		TradeDate:  "2026-07-20",
		Rank:       2,
		Score:      0.7,
		DecisionID: id,
	}
	tr := reg.TraceItem(item)
	if !tr.Found || !tr.Stale || tr.StaleReason != StaleReasonTradeDateMismatch {
		t.Fatalf("expected trade_date_mismatch: %+v", tr)
	}
}

type goldenTracePair struct {
	Code  string      `json:"code"`
	Rank  int         `json:"rank"`
	Score float64     `json:"score"`
	Trace TraceResult `json:"trace"`
}

func TestRegistryGolden(t *testing.T) {
	asOf := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	d1 := sampleDecision("sz000001", "2026-07-21", models.QuantActionEnter, "可买", asOf)
	d2 := sampleDecision("sz000002", "2026-07-21", models.QuantActionWatch, "历史参考", asOf)

	reg := New()
	id1, _ := reg.Put(d1)
	id2, _ := reg.Put(d2)

	items := []models.CandidatePoolItem{
		{StockCode: "sz000001", TradeDate: "2026-07-21", Rank: 1, Score: 0.91, DecisionID: id1},
		{StockCode: "sz000002", TradeDate: "2026-07-21", Rank: 2, Score: 0.75, DecisionID: id2},
		{StockCode: "sz000003", TradeDate: "2026-07-21", Rank: 3, Score: 0.60, DecisionID: "qd1:js_legacy:sz000003:missing:ENTER:_"},
		{StockCode: "sz000004", TradeDate: "2026-07-21", Rank: 4, Score: 0.50},
	}

	traces := make([]goldenTracePair, 0, len(items))
	for _, it := range items {
		tr := reg.TraceItem(it)
		compact := tr
		if compact.Decision != nil {
			compact.Decision = &models.QuantDecision{
				ID:         tr.Decision.ID,
				TradeDate:  tr.Decision.TradeDate,
				Instrument: models.QuantInstrument{StockCode: tr.Decision.Instrument.StockCode},
				Action: models.QuantAction{
					Code:  tr.Decision.Action.Code,
					Label: tr.Decision.Action.Label,
				},
				Meta: models.QuantDecisionMeta{
					SchemaVersion: tr.Decision.Meta.SchemaVersion,
					Producer:      tr.Decision.Meta.Producer,
				},
			}
		}
		traces = append(traces, goldenTracePair{Code: it.StockCode, Rank: it.Rank, Score: it.Score, Trace: compact})
	}

	ids := reg.IDs()
	sort.Strings(ids)

	golden := map[string]any{
		"phase":   "Phase3-B",
		"harness": "quant-decision-registry",
		"count":   reg.Count(),
		"ids":     ids,
		"traces":  traces,
		"summary": map[string]int{
			"ok":      countGolden(traces, func(p goldenTracePair) bool { return p.Trace.Found && !p.Trace.Stale }),
			"stale":   countGolden(traces, func(p goldenTracePair) bool { return p.Trace.Stale }),
			"unbound": countGolden(traces, func(p goldenTracePair) bool { return !p.Trace.Bound }),
		},
	}

	got, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "registry_trace_golden.json")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote initial golden %s", path)
		return
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch\nWANT:\n%s\nGOT:\n%s\n(set UPDATE_GOLDEN=1 to refresh)", want, got)
	}

	for i, p := range traces {
		if p.Rank != items[i].Rank || p.Score != items[i].Score {
			t.Fatalf("golden pair altered Score/Rank: %+v", p)
		}
	}
}

func countGolden(traces []goldenTracePair, fn func(goldenTracePair) bool) int {
	n := 0
	for _, p := range traces {
		if fn(p) {
			n++
		}
	}
	return n
}
