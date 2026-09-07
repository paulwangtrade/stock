/**
 * Phase9-B.5 migration observation accumulation selftest.
 * Run: node frontend/src/utils/riskintel/holdingDecisionAdapter.selftest.mjs
 */

import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import { buildRiskAdviceProjection } from './riskAdviceProjection.js'
import {
  HOLDING_DECISION_COMPARISON,
  getHoldingDecision,
  getHoldingMigrationReadiness,
  getHoldingMigrationStability,
} from './holdingDecisionAdapter.js'
import {
  getHoldingDecisionAdapterMetrics,
  resetHoldingDecisionAdapterMetrics,
} from './holdingDecisionAdapterMetrics.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function mkProj(code, level) {
  const key = `level${level}`
  const shadow = buildScanRiskAdviceShadow({
    stockCode: code,
    marketModeKey: key,
    effectiveMarketMode: { key, level },
    asOf: '2026-07-29T00:00:00.000Z',
  })
  return buildRiskAdviceProjection(shadow.riskAdvice, {
    symbol: code,
    position: { costPrice: 10, costVolume: 100 },
    riskContext: shadow.riskContext,
  }).projection
}

function run() {
  // --- long consistent samples ---
  resetHoldingDecisionAdapterMetrics()
  const p3 = mkProj('sz000003', 3)
  for (let i = 0; i < 35; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'hold', marketLevel: 3 },
      riskAdviceProjection: p3,
      symbol: `sz00000${i % 3}`,
    })
  }
  let ready = getHoldingMigrationReadiness()
  let stab = getHoldingMigrationStability()
  assert(ready.total >= 30, 'min samples path')
  assert(ready.criteria.minSamplesMet === true, 'minSamplesMet')
  assert(stab.totalSamples >= 30, 'stability total')
  assert(stab.rollingMatchRate >= 0.8, `rolling match ${stab.rollingMatchRate}`)
  assert(ready.sourceLocked === 'riskAdvice_projection', 'C.2 preferred projection')
  assert(ready.controlledSwitchEnabled === true, 'controlled switch on')
  // may be READY or NEED depending on concentration/stable flags
  assert(
    ['NEED_MORE_OBSERVATION', 'READY_FOR_CONTROLLED_SWITCH'].includes(ready.recommendation),
    `rec=${ready.recommendation}`,
  )

  // --- sustained diff ---
  resetHoldingDecisionAdapterMetrics()
  for (let i = 0; i < 20; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'reduce', marketLevel: 1, suggestPct: 1 },
      riskAdviceProjection: p3,
      symbol: 'sh600519',
    })
  }
  ready = getHoldingMigrationReadiness()
  assert(ready.diffRate > 0.5, 'sustained high diff')
  assert(ready.recommendation === 'NOT_READY' || ready.recommendation === 'NEED_MORE_OBSERVATION', 'not ready on high diff')

  // --- diff concentrated on few symbols / types ---
  resetHoldingDecisionAdapterMetrics()
  for (let i = 0; i < 25; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'hold', marketLevel: 3 },
      riskAdviceProjection: p3,
      symbol: 'ok1',
    })
  }
  for (let i = 0; i < 10; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'reduce', marketLevel: 3 },
      riskAdviceProjection: p3,
      symbol: 'diffHeavy',
    })
  }
  const m = getHoldingDecisionAdapterMetrics()
  assert(
    m.shadowMigrationMetrics.highestFrequencyDiffSymbols.some((r) => r.key === 'diffHeavy'),
    'top diff symbol',
  )
  assert(
    m.shadowMigrationMetrics.highestFrequencyDiffTypes.some((r) => r.key === 'ACTION_DIFF'),
    'top diff type ACTION_DIFF',
  )
  ready = getHoldingMigrationReadiness()
  assert(ready.diffTypeConcentration >= 0.5, 'concentrated')
  assert(Array.isArray(ready.highestFrequencyDiffSymbols), 'readiness symbols')

  // --- projection occasional failure ---
  resetHoldingDecisionAdapterMetrics()
  for (let i = 0; i < 28; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'hold', marketLevel: 3 },
      riskAdviceProjection: p3,
      symbol: 'sz000001',
    })
  }
  for (let i = 0; i < 2; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'hold', marketLevel: 3 },
      riskAdviceProjection: null,
      symbol: 'sz000001',
      shadowFailure: 'exception',
    })
  }
  ready = getHoldingMigrationReadiness()
  assert(ready.projectionUnavailableRate > 0 && ready.projectionUnavailableRate < 0.15, 'sporadic unavailable')
  assert(
    ready.projectionUnavailableReasons.shadow_exception >= 1
      || ready.projectionUnavailableReasons.missing_projection >= 1,
    'unavailable reasons recorded',
  )
  stab = getHoldingMigrationStability()
  assert(typeof stab.recentTrend === 'string', 'trend')
  assert(typeof stab.stable === 'boolean', 'stable flag observational')
  assert(stab.rollingDiffRate >= 0, 'rolling diff')

  // C.2: with projection present, authority source is riskAdvice_projection
  for (let i = 0; i < 3; i++) {
    const d = getHoldingDecision({
      holdingAdvice: { action: 'hold', marketLevel: 3 },
      riskAdviceProjection: p3,
    })
    assert(d.source === 'riskAdvice_projection', 'projection authority')
    assert(d.legacyDecision?.action === 'hold', 'legacy retained')
    assert(d.projectionDecision != null, 'projection retained')
    assert(d.fallbackUsed === false, 'no fallback')
  }

  console.log('holdingDecisionAdapter.selftest: PASS')
  console.log('stability sample:', stab)
  console.log('readiness sample:', {
    recommendation: ready.recommendation,
    total: ready.total,
    criteria: ready.criteria,
  })
}

run()
