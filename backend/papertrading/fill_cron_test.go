package papertrading_test

import (
	"testing"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestNormalizeFillMode_DefaultA(t *testing.T) {
	require.Equal(t, papertrading.FillModeA, papertrading.NormalizeFillMode(""))
	require.Equal(t, papertrading.FillModeA, papertrading.NormalizeFillMode("a"))
	require.Equal(t, papertrading.FillModeA, papertrading.NormalizeFillMode("A"))
	require.Equal(t, papertrading.FillModeA, papertrading.NormalizeFillMode("both"))
	require.Equal(t, papertrading.FillModeA, papertrading.NormalizeFillMode("unknown"))
	require.Equal(t, papertrading.FillModeB, papertrading.NormalizeFillMode("b"))
	require.Equal(t, papertrading.FillModeB, papertrading.NormalizeFillMode(" B "))
}

func TestFillCronExclusive_A_RegistersOpenOnly(t *testing.T) {
	open, sessionB := papertrading.FillCronExclusive(papertrading.FillModeA)
	require.True(t, open)
	require.False(t, sessionB)

	open, sessionB = papertrading.FillCronExclusive("")
	require.True(t, open, "default empty mode must keep Session A open cron")
	require.False(t, sessionB)
}

func TestFillCronExclusive_B_RegistersSessionBOnly(t *testing.T) {
	open, sessionB := papertrading.FillCronExclusive(papertrading.FillModeB)
	require.False(t, open)
	require.True(t, sessionB)
}

func TestEffectiveFillMode_FromConfigCache(t *testing.T) {
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true})
	t.Cleanup(papertrading.ResetConfigCache)
	require.Equal(t, papertrading.FillModeA, papertrading.EffectiveFillMode())

	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, FillMode: "B"})
	require.Equal(t, papertrading.FillModeB, papertrading.EffectiveFillMode())
}
