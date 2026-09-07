/**
 * Phase9-C.3 Observation Bundle selftest.
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/utils/riskintel/phase9C3ObservationBundle.selftest.mjs
 */
import assert from 'node:assert/strict'
import {
  evaluateHealthyWindowEligibility,
  getPhase9C3ObservationBundle,
  exportPhase9C3ObservationBundleJSON,
  buildPhase9C3ObservationCheckpoint,
  getLastPhase9C3ObservationCheckpoint,
  rememberPhase9C3ObservationCheckpoint,
} from './phase9C3ObservationBundle.js'
import {
  getControlledSwitchStatus,
  enableHoldingDecisionControlledSwitch,
  getHoldingDecision,
  resetControlledSwitchObservations,
  HOLDING_DECISION_SOURCE_PROJECTION,
} from './holdingDecisionAdapter.js'
import { resetHoldingDecisionAdapterMetrics } from './holdingDecisionAdapterMetrics.js'
import { resetHoldingMigrationGateHistory } from './holdingMigrationGate.js'
import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import { buildRiskAdviceProjection } from './riskAdviceProjection.js'

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

resetHoldingDecisionAdapterMetrics()
resetControlledSwitchObservations()
resetHoldingMigrationGateHistory()
rememberPhase9C3ObservationCheckpoint(null)

// --- vacuum / insufficient ---
{
  const hint = evaluateHealthyWindowEligibility({
    status: { totalDecisions: 0, status: 'HEALTHY', source: HOLDING_DECISION_SOURCE_PROJECTION },
    metrics: { totalDecisions: 0 },
  })
  assert.equal(hint.label, 'INSUFFICIENT SAMPLE')
  assert.equal(hint.countsTowardHealthyWindow, false)
}

{
  const bundle = getPhase9C3ObservationBundle({ recordGate: false, meta: { trigger: 'manual_refresh' } })
  for (const key of ['status', 'metrics', 'gate', 'shadowObservation', 'healthyWindowHint', 'capturedAt', 'meta']) {
    assert.ok(key in bundle, `missing key ${key}`)
  }
  assert.equal(bundle.meta.observationOnly, true)
  assert.equal(bundle.healthyWindowHint.label, 'INSUFFICIENT SAMPLE')
  const json = exportPhase9C3ObservationBundleJSON({ recordGate: false })
  assert.ok(json.includes('"status"'))
  assert.ok(json.includes('"healthyWindowHint"'))
}

// --- fill samples then eligibility ---
enableHoldingDecisionControlledSwitch({ record: false })
const p3 = mkProj(3)
const legacyHold = { action: 'hold', marketLevel: 3 }
for (let i = 0; i < 30; i++) {
  getHoldingDecision({
    holdingAdvice: legacyHold,
    riskAdviceProjection: p3,
    symbol: `c3obs${i}`,
  })
}
const status = getControlledSwitchStatus()
assert.equal(status.totalDecisions, 30)
assert.equal(status.status, 'HEALTHY')

{
  const hint = evaluateHealthyWindowEligibility({ status, metrics: {}, meta: { trigger: 'scan_success' } })
  assert.equal(hint.label, 'HEALTHY')
  assert.equal(hint.countsTowardHealthyWindow, true)
}

{
  const bundle = getPhase9C3ObservationBundle({ recordGate: true, meta: { trigger: 'manual_refresh' } })
  assert.equal(bundle.healthyWindowHint.label, 'HEALTHY')
  assert.ok(bundle.gate.sampleCount >= 30)
  assert.equal(bundle.meta.recordGate, true)
}

{
  const cp = buildPhase9C3ObservationCheckpoint({ meta: { scannedCount: 3 } })
  assert.equal(cp.meta.trigger, 'scan_success')
  assert.equal(getLastPhase9C3ObservationCheckpoint()?.meta?.scannedCount, 3)
}

// authority unchanged by bundle
assert.equal(getControlledSwitchStatus().source, HOLDING_DECISION_SOURCE_PROJECTION)

console.log('phase9C3ObservationBundle.selftest: OK')
