package papertrading_test

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestProjectSecurityDisplayName_NoMaster(t *testing.T) {
	d := papertrading.ProjectSecurityDisplayName("万业企业", "")
	require.Equal(t, "万业企业", d.SnapshotName)
	require.Equal(t, "UNKNOWN", d.CurrentName)
	require.Equal(t, "万业企业", d.DisplayName)
	require.False(t, d.NameChanged)
}

func TestProjectSecurityDisplayName_Changed(t *testing.T) {
	d := papertrading.ProjectSecurityDisplayName("万业企业", "先导基电")
	require.Equal(t, "先导基电", d.DisplayName)
	require.True(t, d.NameChanged)
}

func TestProjectSecurityDisplayName_Empty(t *testing.T) {
	d := papertrading.ProjectSecurityDisplayName("", "")
	require.Equal(t, "UNKNOWN", d.DisplayName)
	require.Equal(t, "UNKNOWN", d.CurrentName)
}
