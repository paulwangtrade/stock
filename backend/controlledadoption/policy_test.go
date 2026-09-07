package controlledadoption_test

import (
	"testing"

	"go-stock/backend/controlledadoption"

	"github.com/stretchr/testify/require"
)

func TestDefaultPolicy_IsOff(t *testing.T) {
	t.Parallel()
	controlledadoption.ResetActive()
	p := controlledadoption.DefaultPolicy()
	require.Equal(t, controlledadoption.AdoptionOff, p.Adoption)
	require.False(t, p.KillSwitch)
	require.Empty(t, p.AccountIDs)
	got := controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 1, StrategyName: "s", TradeDate: "2026-08-22",
	})
	require.False(t, got.Matched)
	require.Equal(t, controlledadoption.ReasonAdoptionOff, got.Reason)
}

func TestResolveMatch_ThreeWayIntersection(t *testing.T) {
	t.Parallel()
	p := controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		AccountIDs:    []uint{7},
		StrategyNames: []string{"alpha"},
		TradeDates:    []string{"2026-08-22"},
	}
	ok := controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 7, StrategyName: "alpha", TradeDate: "2026-08-22",
	})
	require.True(t, ok.Matched)

	require.False(t, controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 8, StrategyName: "alpha", TradeDate: "2026-08-22",
	}).Matched)
	require.False(t, controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 7, StrategyName: "beta", TradeDate: "2026-08-22",
	}).Matched)
	require.False(t, controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 7, StrategyName: "alpha", TradeDate: "2026-08-23",
	}).Matched)
}

func TestResolveMatch_EmptyWhitelistFailClosed(t *testing.T) {
	t.Parallel()
	p := controlledadoption.ControlledProviderPolicy{
		Adoption:   controlledadoption.AdoptionControlled,
		AccountIDs: []uint{1},
		// strategy / dates empty
	}
	got := controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 1, StrategyName: "a", TradeDate: "2026-08-22",
	})
	require.False(t, got.Matched)
	require.Equal(t, controlledadoption.ReasonEmptyWhitelist, got.Reason)
}

func TestResolveMatch_KillSwitch(t *testing.T) {
	t.Parallel()
	p := controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		KillSwitch:    true,
		AccountIDs:    []uint{1},
		StrategyNames: []string{"a"},
		TradeDates:    []string{"2026-08-22"},
	}
	got := controlledadoption.ResolveMatch(p, controlledadoption.MatchInput{
		AccountID: 1, StrategyName: "a", TradeDate: "2026-08-22",
	})
	require.False(t, got.Matched)
	require.Equal(t, controlledadoption.ReasonKillSwitch, got.Reason)
}

func TestSetActive_Reset(t *testing.T) {
	controlledadoption.ResetActive()
	t.Cleanup(controlledadoption.ResetActive)
	controlledadoption.SetActive(controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		AccountIDs:    []uint{1},
		StrategyNames: []string{"a"},
		TradeDates:    []string{"2026-08-22"},
	})
	require.True(t, controlledadoption.ResolveActive(controlledadoption.MatchInput{
		AccountID: 1, StrategyName: "a", TradeDate: "2026-08-22",
	}).Matched)
	controlledadoption.ResetActive()
	require.False(t, controlledadoption.ResolveActive(controlledadoption.MatchInput{
		AccountID: 1, StrategyName: "a", TradeDate: "2026-08-22",
	}).Matched)
}
