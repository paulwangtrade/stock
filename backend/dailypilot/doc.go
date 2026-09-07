// Package dailypilot assembles DailyPilotReport — a read-only daily envelope for
// Controlled Pilot observation (Phase13).
//
// Inputs: Shadow records, Validation, ControlledMonitor, PortfolioHistory, and
// optional policy snapshots (supplied by callers; this package never calls
// controlledadoption.SetActive or writes TradePlan).
//
// DefaultEnabled is false. BuildDailyPilotReport returns a skipped ticket unless
// Input.Enabled=true.
package dailypilot
