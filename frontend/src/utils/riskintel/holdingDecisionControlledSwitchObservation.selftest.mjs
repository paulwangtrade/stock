/**
 * Phase9-C.3 Controlled Switch Observation selftest.
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/utils/riskintel/holdingDecisionControlledSwitchObservation.selftest.mjs
 */
import assert from 'node:assert/strict'
import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import { buildRiskAdviceProjection } from './riskAdviceProjection.js'
import {
  getHoldingDecision,
  getControlledSwitchMetrics,
  getControlledSwitchStatus,
  enableHoldingDecisionControlledSwitch,
  rollbackHoldingDecisionToLegacy,
  getHoldingDecisionPreferredSource,
  resetControlledSwitchObservations,
  HOLDING_DECISION_SOURCE_LEGACY,
  HOLDING_DECISION_SOURCE_PROJECTION,
} from './holdingDecisionAdapter.js'
import { resetHoldingDecisionAdapterMetrics } from './holdingDecisionAdapterMetrics.js'

function mkProj(level) {
  const key = `level${level}`
  const shadow = buildScanRiskAdviceShadow({
    stockCode: 'sz000001',
    marketModeKey: key,
    effectiveMarketMode: { key, level },
    asOf: '2026-07-30T00:00:00.000Z',
  })
  return buildRiskAdviceProjection(shadow.riskAdvice, {
    symbol: 'sz000001',
    position: { costPrice: 10, costVolume: 100 },
    riskContext: shadow.riskContext,
  }).projection
}

const legacyHold = { action: 'hold', marketLevel: 3 }
const legacyDiff = { action: 'reduce', marketLevel: 1, suggestPct: 1 }

resetHoldingDecisionAdapterMetrics()
resetControlledSwitchObservations()
enableHoldingDecisionControlledSwitch({ record: false })
assert.equal(getHoldingDecisionPreferredSource(), HOLDING_DECISION_SOURCE_PROJECTION)

const p3 = mkProj(3)

// --- projection normal (aligned) ---
for (let i = 0; i < 20; i++) {
  getHoldingDecision({
    holdingAdvice: legacyHold,
    riskAdviceProjection: p3,
    symbol: `ok${i}`,
  })
}
let metrics = getControlledSwitchMetrics()
assert.equal(metrics.projectionSelectedCount, 20, 'projection selected')
assert.equal(metrics.legacyFallbackCount, 0)
assert.ok(metrics.projectionUsageRate >= 0.99)
assert.ok(typeof metrics.activeSince === 'string')

let status = getControlledSwitchStatus()
assert.equal(status.source, HOLDING_DECISION_SOURCE_PROJECTION)
assert.equal(status.status, 'HEALTHY', `expected HEALTHY got ${status.status}`)
assert.equal(status.totalDecisions, 20)
assert.equal(status.rolledBack, false)

// --- projection fallback ---
for (let i = 0; i < 5; i++) {
  getHoldingDecision({
    holdingAdvice: legacyHold,
    riskAdviceProjection: null,
    symbol: 'fb',
    shadowFailure: 'exception',
  })
}
metrics = getControlledSwitchMetrics()
assert.equal(metrics.legacyFallbackCount, 5, 'fallback count')
assert.ok(metrics.fallbackRate > 0)
assert.ok(metrics.projectionUnavailableReasons.shadow_exception >= 1
  || metrics.projectionUnavailableReasons.missing_projection >= 1)
assert.ok(metrics.decisionDiffRate >= 0)

status = getControlledSwitchStatus()
assert.ok(['HEALTHY', 'DEGRADED', 'ROLLBACK_RECOMMENDED'].includes(status.status), status.status)
// 5/25 = 0.2 fallback → DEGRADED (>=0.1) but < 0.25 rollback
assert.equal(status.status, 'DEGRADED', `fallback path status=${status.status}`)

// --- mixed decision history (diffs) ---
for (let i = 0; i < 20; i++) {
  getHoldingDecision({
    holdingAdvice: legacyDiff,
    riskAdviceProjection: p3,
    symbol: 'diff',
  })
}
metrics = getControlledSwitchMetrics()
assert.ok(metrics.decisionDiffRate > 0.2, `diffRate=${metrics.decisionDiffRate}`)
status = getControlledSwitchStatus()
assert.ok(
  status.status === 'DEGRADED' || status.status === 'ROLLBACK_RECOMMENDED',
  `mixed status=${status.status}`,
)

// --- rollback legacy ---
rollbackHoldingDecisionToLegacy({ record: false })
assert.equal(getHoldingDecisionPreferredSource(), HOLDING_DECISION_SOURCE_LEGACY)
getHoldingDecision({
  holdingAdvice: legacyHold,
  riskAdviceProjection: p3,
  symbol: 'after-rollback',
})
status = getControlledSwitchStatus()
assert.equal(status.source, HOLDING_DECISION_SOURCE_LEGACY)
assert.equal(status.status, 'ROLLBACK_RECOMMENDED')
assert.equal(status.rolledBack, true)

const metricsAfterRb = getControlledSwitchMetrics()
assert.ok(metricsAfterRb.preferredLegacyCount >= 1, 'preferred legacy counted')

// --- restore projection authority (C.3 end state) ---
enableHoldingDecisionControlledSwitch({ record: false })
assert.equal(getHoldingDecisionPreferredSource(), HOLDING_DECISION_SOURCE_PROJECTION)
status = getControlledSwitchStatus()
assert.equal(status.source, HOLDING_DECISION_SOURCE_PROJECTION)
assert.ok(typeof status.activeSince === 'string')
assert.ok(status.totalDecisions >= 45)

console.log('holdingDecisionControlledSwitchObservation.selftest: PASS')
console.log('final status:', status)
console.log('final metrics:', getControlledSwitchMetrics())
