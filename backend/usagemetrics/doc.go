// Package usagemetrics records Shell / product feature usage for commercial insight.
//
// C4: local UsageEvent capture + aggregate counts.
// Phase13-E: Usage Analytics Read Model (AnalyticsReader) → Product API / Dashboard.
// Forbidden: cloud sync, data sale, ad systems, trading-sensitive payloads
// (orders, fills, positions, stock codes, account balances, broker credentials).
package usagemetrics
