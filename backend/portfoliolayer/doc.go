// Package portfoliolayer is the Phase12-F.4–F.6 Portfolio Selection / Allocation skeleton.
//
// Shadow only: types, constraint resolve, pure Select/Allocate, Observe (off by default),
// F.5 observation diffs, and F.6 evaluation deltas (record-only, no auto decision).
// It does not wire into CandidatePool, PlanFilter, TradePlan, FixedAmountSizer,
// morning materialization, Freeze, or Execution.
package portfoliolayer
