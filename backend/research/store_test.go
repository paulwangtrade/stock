package research

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeStatus(t *testing.T) {
	s, err := NormalizeStatus("WATCHING")
	require.NoError(t, err)
	require.Equal(t, StatusWatching, s)

	s, err = NormalizeStatus("dismissed")
	require.NoError(t, err)
	require.Equal(t, StatusDiscarded, s)

	_, err = NormalizeStatus("promoted")
	require.Error(t, err)
	_, err = NormalizeStatus("ready")
	require.Error(t, err)
}

func TestMemoryStore_PatchMergeAndPersistFields(t *testing.T) {
	SetStoreForTest(NewMemoryStoreForTest())
	t.Cleanup(ResetStoreForTest)

	id := "rc:signal:2026-08-17:sz000001"
	st := StatusReviewed
	note := "值得跟踪"
	tags := []string{"银行", "观察"}
	ann, err := DefaultStore().Patch(id, UpdatePatch{
		Status: &st,
		Note:   &note,
		Tags:   &tags,
	})
	require.NoError(t, err)
	require.Equal(t, StatusReviewed, ann.Status)
	require.Equal(t, "值得跟踪", ann.Note)
	require.True(t, ann.HasTags)
	require.Equal(t, []string{"银行", "观察"}, ann.Tags)
	require.False(t, ann.UpdatedAt.IsZero())

	got, ok := DefaultStore().Get(id)
	require.True(t, ok)
	require.Equal(t, StatusReviewed, got.Status)

	note2 := "更新备注"
	ann2, err := DefaultStore().Patch(id, UpdatePatch{Note: &note2})
	require.NoError(t, err)
	require.Equal(t, StatusReviewed, ann2.Status)
	require.Equal(t, "更新备注", ann2.Note)
}
