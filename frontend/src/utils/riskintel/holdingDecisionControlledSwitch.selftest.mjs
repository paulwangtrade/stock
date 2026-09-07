/**
 * Phase9-C.2 Controlled Switch selftest.
 * Run: node --import ./frontend/scripts/loaders/register-js-ext.mjs frontend/src/utils/riskintel/holdingDecisionControlledSwitch.selftest.mjs
 */
import assert from 'node:assert/strict'
import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import { buildRiskAdviceProjection } from './riskAdviceProjection.js'
import {
  getHoldingDecision,
  resolveAuthorityHoldingDecision,
  getHoldingDecisionPreferredSource,
  setHoldingDecisionAuthoritySource,
  enableHoldingDecisionControlledSwitch,
  rollbackHoldingDecisionToLegacy,
  isHoldingDecisionControlledSwitchEnabled,
  getControlledSwitchObservations,
  resetControlledSwitchObservations,
  HOLDING_DECISION_SOURCE_LEGACY,
  HOLDING_DECISION_SOURCE_PROJECTION,
  mapProjectionToSimulatedDecision,
} from './holdingDecisionAdapter.js'
import { resetHoldingDecisionAdapterMetrics } from './holdingDecisionAdapterMetrics.js'
import { assembleWatchlistDecision } from '../quantDecisionAssemble.js'

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

const legacyAdvice = {
  action: 'add',
  actionLabel: '倾向加仓',
  summaryLine: '量价偏强',
  suggestPct: 0.2,
  suggestPctDisplay: 20,
  score: 0.6,
  factors: [{ impact: 1, label: 'x', detail: 'y' }],
  marketLevel: 4,
}

resetHoldingDecisionAdapterMetrics()
resetControlledSwitchObservations()
enableHoldingDecisionControlledSwitch({ record: true })

assert.equal(getHoldingDecisionPreferredSource(), HOLDING_DECISION_SOURCE_PROJECTION)
assert.equal(isHoldingDecisionControlledSwitchEnabled(), true)

const p3 = mkProj(3)
assert.ok(p3, 'projection built')

// --- projection source normal ---
const withProj = getHoldingDecision({
  holdingAdvice: legacyAdvice,
  riskAdviceProjection: p3,
  symbol: 'sz000001',
})
assert.equal(withProj.source, HOLDING_DECISION_SOURCE_PROJECTION, 'projection source')
assert.equal(withProj.fallbackUsed, false)
assert.ok(withProj.legacyDecision === legacyAdvice, 'legacyDecision retained')
assert.ok(withProj.projectionDecision === p3, 'projectionDecision retained')
assert.ok(withProj.decisionDiff, 'decisionDiff present')
assert.ok(withProj.comparisonCode, 'comparisonCode present')
const mapped = mapProjectionToSimulatedDecision(p3)
assert.equal(withProj.decision.action, mapped.action)
assert.equal(withProj.decision.actionLabel, mapped.actionLabel)

const auth = resolveAuthorityHoldingDecision({ holdingDecision: withProj })
assert.equal(auth.action, mapped.action, 'consumer sees projection decision')

const assembled = assembleWatchlistDecision({
  entry: {
    code: 'sz000001',
    name: '测试',
    tag: null,
    holdingAdvice: legacyAdvice,
    holdingDecision: withProj,
    riskAdviceProjection: p3,
  },
  asOf: '2026-07-30T00:00:00.000Z',
})
assert.equal(assembled.holdingBias.action, mapped.action, 'assemble bias from projection')
assert.notEqual(assembled.holdingBias.actionLabel, '倾向加仓', 'label not legacy when projection differs')

// --- projection unavailable fallback ---
const noProj = getHoldingDecision({
  holdingAdvice: legacyAdvice,
  riskAdviceProjection: null,
  symbol: 'sz000001',
})
assert.equal(noProj.source, HOLDING_DECISION_SOURCE_LEGACY, 'fallback legacy source')
assert.equal(noProj.fallbackUsed, true)
assert.equal(noProj.decision, legacyAdvice)
assert.equal(noProj.preferredSource, HOLDING_DECISION_SOURCE_PROJECTION)
assert.equal(noProj.legacyDecision, legacyAdvice)
assert.equal(noProj.decisionDiff, 'projectionUnavailable')

// --- rollback legacy ---
const obsBefore = getControlledSwitchObservations().length
rollbackHoldingDecisionToLegacy()
assert.equal(getHoldingDecisionPreferredSource(), HOLDING_DECISION_SOURCE_LEGACY)
assert.equal(isHoldingDecisionControlledSwitchEnabled(), false)
assert.ok(getControlledSwitchObservations().length > obsBefore, 'rollback recorded')

const rolled = getHoldingDecision({
  holdingAdvice: legacyAdvice,
  riskAdviceProjection: p3,
  symbol: 'sz000001',
})
assert.equal(rolled.source, HOLDING_DECISION_SOURCE_LEGACY, 'rollback uses legacy')
assert.equal(rolled.decision, legacyAdvice)
assert.equal(rolled.fallbackUsed, false)
assert.ok(rolled.projectionDecision === p3, 'shadow projection still kept')
assert.ok(rolled.legacyDecision === legacyAdvice)

const assembledLegacy = assembleWatchlistDecision({
  entry: {
    code: 'sz000001',
    name: '测试',
    tag: null,
    holdingAdvice: legacyAdvice,
    holdingDecision: rolled,
    riskAdviceProjection: p3,
  },
  asOf: '2026-07-30T00:00:00.000Z',
})
assert.equal(assembledLegacy.holdingBias.action, 'add')
assert.equal(assembledLegacy.action.label, '倾向加仓', 'consumer restored legacy label')

// --- re-enable controlled switch (leave ON as C.2 end state) ---
enableHoldingDecisionControlledSwitch()
assert.equal(getHoldingDecisionPreferredSource(), HOLDING_DECISION_SOURCE_PROJECTION)

const obs = getControlledSwitchObservations()
assert.ok(obs.length >= 2, 'switch observations present')
for (const row of obs) {
  assert.ok(typeof row.source === 'string')
  assert.ok(typeof row.timestamp === 'string')
  assert.ok(typeof row.sampleCount === 'number')
  assert.ok(typeof row.diffRate === 'number')
  assert.ok(typeof row.unavailableRate === 'number')
}

// leave preferred = projection for process (tests that import later may reset)
setHoldingDecisionAuthoritySource(HOLDING_DECISION_SOURCE_PROJECTION, { record: false })

console.log('holdingDecisionControlledSwitch.selftest: PASS')
console.log('last switch obs:', obs[obs.length - 1])
