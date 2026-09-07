# riskintel (Phase8-0)

Watchlist **Context / Advice** module skeleton.

- Contract: `PHASE8_0_RISKINTEL_OBJECT_CONTRACT.md`
- Types: `types.js` (JSDoc) — `RiskContextBundle`, `RiskAdvice`, `RiskAdviceReason`
- Constants: `constants.js` — `POLICY_VERSION`, `ADVICE_CATEGORY`, `FORBIDDEN_FIELDS`

**Not** `backend/risk` PlanFilter / ApproveGate / Execution / Broker.

- Step0: types + constants
- Step1: `buildRiskContextBundle`
- Step2: `buildRiskAdvice`
- Step3-A: `scanRiskAdviceShadow` dual-write after holding adjust (legacy UI unchanged)
- Step3-B: `scanRiskAdviceShadowMetrics` observation only (generated/failed/level/tags)
- Step3-C: `scanRiskAdviceShadowEvaluation` legacy vs RiskAdvice compare (match/mismatch/delta)
- Phase8.5: observation taxonomy + `getShadowMigrationReadiness()` (decision support only)
- Phase8.6: `riskAdviceProjection` read-only holding-facing shape (not a decision)
- Phase9-A: `holdingDecisionAdapter` unified reader (source locked to legacy)
- Phase9-B: shadow validation taxonomy + `simulateRiskAdviceSource` + holding migration readiness
- Phase9-B.5: migration observation accumulation + `getHoldingMigrationStability()`
- Phase9-B.6: `holdingMigrationGate` snapshot / trend / text report (no switch)
- Phase9-C.1: `resolveAuthorityHoldingDecision` consumer choke
- Phase9-C.2: controlled switch ON — preferred authority `riskAdvice_projection` (fallback legacy; rollback API)
- Phase9-C.3: `getControlledSwitchMetrics` / `getControlledSwitchStatus` (HEALTHY|DEGRADED|ROLLBACK_RECOMMENDED)
