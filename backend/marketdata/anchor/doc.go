// Package anchor provides Execution Intent price-anchor resolution for after-close
// TradePlan Intent populate (Phase10-B.0).
//
// Boundary:
//   - Read-only market/local price projections; no TradePlan state machine,
//     Approve/Freeze, Execution, or fabricated prices.
//   - B.0 ships FollowedStockAnchorProvider only (parity with legacy
//     strategy.defaultAfterCloseAnchor). Kline / Candidate Snapshot are Phase10-B.1+.
package anchor
