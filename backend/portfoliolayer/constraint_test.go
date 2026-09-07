package portfoliolayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolve_UserTightensNamesOverStrategy(t *testing.T) {
	t.Parallel()
	got := ConstraintSet{
		User:     PreferenceLayer{MaxNewNames: i(3)},
		Strategy: PreferenceLayer{MaxNewNames: i(8)},
	}.Resolve()
	require.Equal(t, 3, got.MaxNewNames)
}

func TestResolve_RiskCeilingBlocksUserWiden(t *testing.T) {
	t.Parallel()
	got := ConstraintSet{
		User: PreferenceLayer{MaxSingleWeight: f(0.30), MaxGrossExposurePct: f(0.99)},
		Risk: RiskLayer{MaxSingleNamePct: f(0.20), MaxGrossExposurePct: f(0.85)},
	}.Resolve()
	require.InDelta(t, 0.20, got.MaxSingleWeight, 1e-9)
	require.InDelta(t, 0.85, got.MaxGrossExposurePct, 1e-9)
}

func TestResolve_UserCanTightenWithinRiskCeiling(t *testing.T) {
	t.Parallel()
	got := ConstraintSet{
		User: PreferenceLayer{MaxSingleWeight: f(0.10)},
		Risk: RiskLayer{MaxSingleNamePct: f(0.20)},
	}.Resolve()
	require.InDelta(t, 0.10, got.MaxSingleWeight, 1e-9)
}

func TestResolve_SkipHoldingForcesAllowAddFalse(t *testing.T) {
	t.Parallel()
	got := ConstraintSet{
		User:     PreferenceLayer{SkipAlreadyHolding: b(true), AllowAddToHolding: b(true)},
		Strategy: PreferenceLayer{SkipAlreadyHolding: b(false)},
	}.Resolve()
	require.True(t, got.SkipAlreadyHolding)
	require.False(t, got.AllowAddToHolding)
}

func TestResolve_DefaultMaxNewNames(t *testing.T) {
	t.Parallel()
	got := ConstraintSet{}.Resolve()
	require.Equal(t, DefaultMaxNewNames, got.MaxNewNames)
	require.InDelta(t, DefaultMaxGrossExposurePct, got.MaxGrossExposurePct, 1e-9)
	require.InDelta(t, DefaultMaxSingleNamePct, got.MaxSingleWeight, 1e-9)
}

func TestResolve_HigherReserveIsTighter(t *testing.T) {
	t.Parallel()
	got := ConstraintSet{
		User:      PreferenceLayer{ReserveCashRatio: f(0.10)},
		Portfolio: PreferenceLayer{ReserveCashRatio: f(0.20)},
	}.Resolve()
	require.InDelta(t, 0.20, got.ReserveCashRatio, 1e-9)
}
