package brand_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-stock/backend/brand"

	"github.com/stretchr/testify/require"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Dir(filepath.Dir(wd))
}

func TestPhase1ProductSurfaces_NoLegacyBrandTokens(t *testing.T) {
	t.Parallel()
	repo := repoRoot(t)
	hits, err := brand.ScanProductSurfaces(repo)
	require.NoError(t, err)
	if len(hits) > 0 {
		var b strings.Builder
		for _, h := range hits {
			fmt.Fprintf(&b, "%s:%d token=%q\n", h.Path, h.Line, h.Token)
		}
		t.Fatalf("legacy brand tokens found:\n%s", b.String())
	}
}

func TestPhase1B_AppGo_NoLegacyChannels(t *testing.T) {
	t.Parallel()
	require.NoError(t, brand.ValidateAppGoNoLegacyChannels(repoRoot(t)))
}

func TestPhase1_WailsJSON_OfficialIdentity(t *testing.T) {
	t.Parallel()
	require.NoError(t, brand.ValidateWailsJSON(repoRoot(t)))
}

func TestPhase1_PaymentImagesRemoved(t *testing.T) {
	t.Parallel()
	require.NoError(t, brand.ValidatePaymentImagesRemoved(repoRoot(t)))
}

func TestPhase1_MainGo_NoPaymentEmbed(t *testing.T) {
	t.Parallel()
	require.NoError(t, brand.ValidateMainGoEmbed(repoRoot(t)))
}

func TestPhase1_GetVersionInfo_NoPaymentImages(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "app.go"))
	require.NoError(t, err)
	content := string(b)
	require.NotContains(t, content, "GetImageBase(alipay)")
	require.NotContains(t, content, "GetImageBase(wxpay)")
	require.NotContains(t, content, "GetImageBase(wxgzh)")
}
