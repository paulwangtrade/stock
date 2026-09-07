// Package portfolioreplay is the Phase12-F.11 independent replay runner (extended).
//
// For each ReplayCase it runs Legacy Simulation + Portfolio Simulation via
// portfoliosim.Simulate, then aggregates a PortfolioReplayReport of decision
// behavior (candidate / allocation / cash / risk hits / concentration / sector).
//
// It does not query DB, create TradePlans, place orders, compute returns,
// run backtests, or auto-tune parameters.
package portfolioreplay
