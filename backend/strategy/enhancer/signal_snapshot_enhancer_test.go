package enhancer

import (
	"testing"

	"go-stock/backend/models"
)

func TestSignalScoreFromTag(t *testing.T) {
	cases := map[string]float64{
		"强": 1.0, "买": 0.85, "突": 0.75, "趋": 0.65, "转": 0.55, "弹": 0.45, "": 0, "x": 0,
	}
	for tag, want := range cases {
		if got := SignalScoreFromTag(tag); got != want {
			t.Fatalf("tag=%q got=%v want=%v", tag, got, want)
		}
	}
}

func TestComposeCandidateScore(t *testing.T) {
	cs := ComposeCandidateScore(0.8, 1.0)
	want := StrategyWeight*0.8 + SignalWeight*1.0
	if cs.StrategyScore != 0.8 || cs.SignalScore != 1.0 || cs.FinalScore != want {
		t.Fatalf("got %+v want final=%v", cs, want)
	}
	item := CandidateItem{}
	item.ApplyScore(cs)
	if item.Score != cs.FinalScore || item.SignalScore != 1.0 || item.StrategyScore != 0.8 {
		t.Fatalf("ApplyScore failed: %+v", item)
	}
}

type snapStub struct {
	snap *models.SignalScanSnapshot
	hits []models.SignalScanHit
}

func (s *snapStub) GetLatestCloseSignalSnapshot(asOfDate string) (*models.SignalScanSnapshot, error) {
	return s.snap, nil
}

func (s *snapStub) ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit {
	if snap == nil {
		return nil
	}
	return s.hits
}

func TestEnhanceWithoutSnapshotSetsZeroSignal(t *testing.T) {
	e := &SignalSnapshotEnhancer{Source: &snapStub{snap: nil}}
	items := []CandidateItem{
		{StockCode: "sz000001", StrategyScore: 0.9},
		{StockCode: "sh600519", StrategyScore: 0.8},
	}
	out, err := e.Enhance(items, EnhanceContext{TradeDate: "2026-07-17"})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range out {
		if it.SignalTag != "" || it.SignalScore != 0 || it.SignalSnapshotID != 0 {
			t.Fatalf("expected empty signal, got %+v", it)
		}
		want := StrategyWeight * it.StrategyScore
		if it.Score != want {
			t.Fatalf("score=%v want=%v", it.Score, want)
		}
	}
}

func TestEnhanceWithSnapshotRaisesStrongTag(t *testing.T) {
	// A: strategy 0.8 + 强(1.0) → 0.6*0.8+0.4*1=0.88
	// B: strategy 0.9 + 无信号 → 0.54
	e := &SignalSnapshotEnhancer{Source: &snapStub{
		snap: &models.SignalScanSnapshot{ID: 123},
		hits: []models.SignalScanHit{
			{SECUCODE: "000001.SZ", Tag: "强"},
		},
	}}
	items := []CandidateItem{
		{StockCode: "sz000001", StrategyScore: 0.8},
		{StockCode: "sh600519", StrategyScore: 0.9},
	}
	out, err := e.Enhance(items, EnhanceContext{TradeDate: "2026-07-17"})
	if err != nil {
		t.Fatal(err)
	}
	var a, b CandidateItem
	for _, it := range out {
		switch it.StockCode {
		case "sz000001":
			a = it
		case "sh600519":
			b = it
		}
	}
	if a.SignalTag != "强" || a.SignalSnapshotID != 123 {
		t.Fatalf("A signal meta: %+v", a)
	}
	wantA := StrategyWeight*0.8 + SignalWeight*1.0
	if a.Score != wantA {
		t.Fatalf("A score=%v want=%v", a.Score, wantA)
	}
	if b.SignalScore != 0 {
		t.Fatalf("B should have no signal")
	}
	wantB := StrategyWeight * 0.9
	if b.Score != wantB {
		t.Fatalf("B score=%v want=%v", b.Score, wantB)
	}
	if a.Score <= b.Score {
		t.Fatalf("A(%v) should outrank B(%v) after strong tag boost", a.Score, b.Score)
	}
}

func TestSortTopNChangesAfterEnhance(t *testing.T) {
	items := []CandidateItem{
		{StockCode: "sh600519", StrategyScore: 1.0},
		{StockCode: "sz000001", StrategyScore: 0.5},
	}
	e := &SignalSnapshotEnhancer{Source: &snapStub{
		snap: &models.SignalScanSnapshot{ID: 1},
		hits: []models.SignalScanHit{
			{SECUCODE: "sz000001", Tag: "强"},
		},
	}}
	out, err := e.Enhance(items, EnhanceContext{TradeDate: "2026-07-17"})
	if err != nil {
		t.Fatal(err)
	}
	sortByScore(out)
	if out[0].StockCode != "sz000001" {
		t.Fatalf("expected sz000001 first after enhance, got %s (scores 000001=%.2f 600519=%.2f)",
			out[0].StockCode, scoreOf(out, "sz000001"), scoreOf(out, "sh600519"))
	}
}

func sortByScore(items []CandidateItem) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Score > items[i].Score {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func scoreOf(items []CandidateItem, code string) float64 {
	for _, it := range items {
		if it.StockCode == code {
			return it.Score
		}
	}
	return 0
}
