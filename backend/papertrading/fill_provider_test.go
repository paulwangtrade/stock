package papertrading_test

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestSelectFillProvider_SessionA_Realtime(t *testing.T) {
	p := papertrading.SelectFillProvider(papertrading.SessionA, nil)
	_, ok := p.(papertrading.RealtimeOpenPriceProvider)
	require.True(t, ok, "Session A must use RealtimeOpenPriceProvider")
}

func TestSelectFillProvider_SessionB_Close(t *testing.T) {
	p := papertrading.SelectFillProvider(papertrading.SessionB, nil)
	_, ok := p.(papertrading.CloseFillProvider)
	require.True(t, ok, "Session B must use CloseFillProvider")
}

func TestSelectFillProvider_SessionC_Missing(t *testing.T) {
	// C/closed should not reach provider in production; selector still fail-closes.
	p := papertrading.SelectFillProvider(papertrading.SessionC, nil)
	_, ok := p.(papertrading.MissingPriceProvider)
	require.True(t, ok)
	p2 := papertrading.SelectFillProvider(papertrading.SessionClosed, nil)
	_, ok = p2.(papertrading.MissingPriceProvider)
	require.True(t, ok)
}

func TestSelectFillProvider_SessionB_OverridesCronDefaultOpen(t *testing.T) {
	cronOpen := papertrading.DefaultOpenPriceProvider()
	_, isRealtime := cronOpen.(papertrading.RealtimeOpenPriceProvider)
	require.True(t, isRealtime)

	p := papertrading.SelectFillProvider(papertrading.SessionB, cronOpen)
	_, ok := p.(papertrading.CloseFillProvider)
	require.True(t, ok, "cron DefaultOpen must not stick on Session B")
}

func TestSelectFillProvider_StaticOverride(t *testing.T) {
	static := papertrading.StaticPriceProvider{Quotes: map[string]papertrading.Quote{"sz000001": {Open: 1}}}
	p := papertrading.SelectFillProvider(papertrading.SessionB, static)
	_, ok := p.(papertrading.StaticPriceProvider)
	require.True(t, ok)
}
