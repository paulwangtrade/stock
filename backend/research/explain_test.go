package research

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMakeAndParseExplainID(t *testing.T) {
	cid := "rc:signal:2026-08-17:sz000001"
	eid := MakeExplainID(cid)
	require.Equal(t, "rx:rc:signal:2026-08-17:sz000001", eid)
	got, ok := ParseExplainID(eid)
	require.True(t, ok)
	require.Equal(t, cid, got)
	_, ok = ParseExplainID("bad")
	require.False(t, ok)
}

func TestDeriveExplain_DeterministicHashAndNoIntent(t *testing.T) {
	score := 92.0
	snap := uint(7)
	c := Candidate{
		ID:               "rc:signal:2026-08-17:sz000001",
		TradeDate:        "2026-08-17",
		StockCode:        "sz000001",
		StockName:        "平安银行",
		Source:           SourceSignalSnapshot,
		SourceRef:        "snapshot_id=7",
		SignalTag:        "强",
		SignalScore:      &score,
		SignalSnapshotID: &snap,
		Direction:        "看多",
		Reason:           "今日强化买点",
		Price:            "11",
	}
	now := time.Date(2026, 8, 17, 16, 0, 0, 0, time.UTC)
	a := DeriveExplain(c, now)
	b := DeriveExplain(c, now)
	require.True(t, a.Available)
	require.Equal(t, ExplainTypeSignalDerived, a.ExplainType)
	require.Equal(t, ExplainSchemaVersion, a.SchemaVersion)
	require.Equal(t, a.Evidence.EvidenceHash, b.Evidence.EvidenceHash)
	require.NotEmpty(t, a.Summary)
	require.Nil(t, a.StrategyIntentRef)
	require.NotNil(t, a.RiskNote)
	require.Equal(t, ReasonKindSignalText, a.ResearchReason.Kind)
}

func TestUpdateExplain_OverlayHybrid(t *testing.T) {
	SetStoreForTest(NewMemoryStoreForTest())
	SetExplainStoreForTest(NewExplainMemoryStoreForTest())
	t.Cleanup(func() {
		ResetStoreForTest()
		ResetExplainStoreForTest()
	})

	// Without DB candidate load will fail — unit-test merge helpers instead.
	score := 80.0
	c := Candidate{
		ID:          "rc:signal:2026-08-17:sz000001",
		TradeDate:   "2026-08-17",
		StockCode:   "sz000001",
		StockName:   "平安银行",
		Source:      SourceSignalSnapshot,
		SignalTag:   "趋",
		SignalScore: &score,
		Direction:   "看多",
		Reason:      "趋势",
	}
	ex := DeriveExplain(c, time.Now().UTC())
	sum := "覆盖摘要"
	ov := ExplainOverlay{HasSummary: true, Summary: sum, UpdatedAt: time.Now().UTC()}
	mergeExplainOverlay(&ex, ov)
	require.Equal(t, sum, ex.Summary)
	require.Equal(t, ExplainTypeHybrid, ex.ExplainType)
	require.Nil(t, ex.StrategyIntentRef)
}

func TestTruncateExplainSummary(t *testing.T) {
	require.Equal(t, "短", TruncateExplainSummary("短", 80))
	long := stringsRepeat("测", 100)
	out := TruncateExplainSummary(long, 80)
	require.True(t, len([]rune(out)) <= 81) // 80 + ellipsis char
}

func stringsRepeat(s string, n int) string {
	out := make([]rune, 0, n)
	r := []rune(s)
	for i := 0; i < n; i++ {
		out = append(out, r...)
	}
	return string(out)
}
