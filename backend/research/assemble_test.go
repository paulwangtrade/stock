package research

import (
	"testing"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestMakeAndParseCandidateID(t *testing.T) {
	id := MakeCandidateID("2026-08-17", "SZ000001")
	require.Equal(t, "rc:signal:2026-08-17:sz000001", id)
	td, code, ok := ParseCandidateID(id)
	require.True(t, ok)
	require.Equal(t, "2026-08-17", td)
	require.Equal(t, "sz000001", code)

	_, _, ok = ParseCandidateID("bad")
	require.False(t, ok)
	_, _, ok = ParseCandidateID("rc:signal:2026-8-17:sz000001")
	require.False(t, ok)
}

func TestAssembleFromSnapshot_StableIDAndDefaults(t *testing.T) {
	src := &models.ResearchSnapshotCandidateList{
		SnapshotID:   123,
		SnapshotTime: "2026-08-17T15:05:00Z",
		TradeDate:    "2026-08-17",
		MinScore:     60,
		Items: []models.ResearchSnapshotCandidate{
			{
				StockCode:   "sz000001",
				StockName:   "平安银行",
				SignalScore: 92,
				SignalTag:   "强",
				Direction:   "看多",
				StatusText:  "今日强化买点",
				Price:       "11.2",
				Industry:    "银行",
			},
		},
	}
	out := AssembleFromSnapshot(src)
	require.Equal(t, "2026-08-17", out.TradeDate)
	require.Len(t, out.Items, 1)
	c := out.Items[0]
	require.Equal(t, "rc:signal:2026-08-17:sz000001", c.ID)
	require.Equal(t, StatusNew, c.Status)
	require.Equal(t, SourceSignalSnapshot, c.Source)
	require.Equal(t, "snapshot_id=123", c.SourceRef)
	require.NotNil(t, c.SignalScore)
	require.Equal(t, 92.0, *c.SignalScore)
	require.NotNil(t, c.SignalSnapshotID)
	require.Equal(t, uint(123), *c.SignalSnapshotID)
	require.Equal(t, []string{"银行"}, c.Tags)
	require.NotNil(t, c.Rank)
	require.Equal(t, 1, *c.Rank)
}

func TestValidateTradeDate(t *testing.T) {
	require.NoError(t, ValidateTradeDate(""))
	require.NoError(t, ValidateTradeDate("2026-08-17"))
	require.Error(t, ValidateTradeDate("2026/08/17"))
	require.Error(t, ValidateTradeDate("not-a-date"))
}
