package models

import (
	"testing"
	"time"
)

func TestTradePlanLifecycleHelpers(t *testing.T) {
	t.Parallel()

	now := time.Now()
	zero := time.Time{}

	cases := []struct {
		name               string
		plan               TradePlan
		wantDraft          bool
		wantReady          bool
		wantFrozen         bool
		wantExecutable     bool
		wantTerminal       bool
	}{
		{
			name:           "empty",
			plan:           TradePlan{},
			wantExecutable: false,
		},
		{
			name:           "draft",
			plan:           TradePlan{Status: TradePlanStatusDraft},
			wantDraft:      true,
			wantExecutable: false,
		},
		{
			name:           "legacy morning ready (FreezeAt nil)",
			plan:           TradePlan{Status: TradePlanStatusReady},
			wantReady:      true,
			wantFrozen:     false,
			wantExecutable: true,
		},
		{
			name:           "frozen ready",
			plan:           TradePlan{Status: TradePlanStatusReady, FreezeAt: &now},
			wantReady:      true,
			wantFrozen:     true,
			wantExecutable: true,
		},
		{
			name:           "ready with zero FreezeAt is not frozen",
			plan:           TradePlan{Status: TradePlanStatusReady, FreezeAt: &zero},
			wantReady:      true,
			wantFrozen:     false,
			wantExecutable: true,
		},
		{
			name:           "executing",
			plan:           TradePlan{Status: TradePlanStatusExecuting},
			wantExecutable: false,
			wantTerminal:   false,
		},
		{
			name:         "done terminal",
			plan:         TradePlan{Status: TradePlanStatusDone},
			wantTerminal: true,
		},
		{
			name:         "partial terminal",
			plan:         TradePlan{Status: TradePlanStatusPartial},
			wantTerminal: true,
		},
		{
			name:         "skipped terminal",
			plan:         TradePlan{Status: TradePlanStatusSkipped},
			wantTerminal: true,
		},
		{
			name:         "failed terminal",
			plan:         TradePlan{Status: TradePlanStatusFailed},
			wantTerminal: true,
		},
		{
			name:         "superseded terminal",
			plan:         TradePlan{Status: TradePlanStatusSuperseded},
			wantTerminal: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.plan.IsDraft(); got != tc.wantDraft {
				t.Fatalf("IsDraft=%v want %v", got, tc.wantDraft)
			}
			if got := tc.plan.IsReady(); got != tc.wantReady {
				t.Fatalf("IsReady=%v want %v", got, tc.wantReady)
			}
			if got := tc.plan.IsFrozen(); got != tc.wantFrozen {
				t.Fatalf("IsFrozen=%v want %v", got, tc.wantFrozen)
			}
			if got := tc.plan.IsExecutableStatus(); got != tc.wantExecutable {
				t.Fatalf("IsExecutableStatus=%v want %v", got, tc.wantExecutable)
			}
			if got := tc.plan.IsTerminal(); got != tc.wantTerminal {
				t.Fatalf("IsTerminal=%v want %v", got, tc.wantTerminal)
			}
		})
	}
}

func TestTradePlanSourceSessionConstants(t *testing.T) {
	t.Parallel()
	if TradePlanSourceAfterClose != "after_close" {
		t.Fatalf("TradePlanSourceAfterClose=%q", TradePlanSourceAfterClose)
	}
	if TradePlanSourceMorningRebuild != "morning_rebuild" {
		t.Fatalf("TradePlanSourceMorningRebuild=%q", TradePlanSourceMorningRebuild)
	}
	if TradePlanSourceCashRescale != "cash_rescale" {
		t.Fatalf("TradePlanSourceCashRescale=%q", TradePlanSourceCashRescale)
	}
}
