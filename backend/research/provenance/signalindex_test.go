package provenance

import (
	"testing"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestBuildHitIndex_NormalizesSinaCode(t *testing.T) {
	hits := []models.SignalScanHit{
		{SECUCODE: "000001.SZ", Tag: "强"},
		{SECURITY_CODE: "600519", Tag: "买"},
	}
	idx := BuildHitIndex(hits)
	require.Contains(t, idx, "sz000001")
	require.Contains(t, idx, "sh600519")
	require.Equal(t, "强", idx["sz000001"].Tag)
	require.Equal(t, "买", idx["sh600519"].Tag)
}

func TestBuildHitIndex_Empty(t *testing.T) {
	require.Empty(t, BuildHitIndex(nil))
}
