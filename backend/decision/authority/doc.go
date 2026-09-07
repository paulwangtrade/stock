// Package authority freezes QuantDecision Authority Boundary (Phase3-D).
//
// Decision is Fact / Explanation / Shadow compare object — NOT sort input,
// trade authorization, or Execution command.
//
// Boundaries:
//   - Does NOT wire Decision → TradePlan / Order / Broker
//   - Does NOT mutate Candidate Rank / Score / Pool lifecycle
//   - Does NOT change js_legacy producer / deriveQuantAction / UI copy
package authority
