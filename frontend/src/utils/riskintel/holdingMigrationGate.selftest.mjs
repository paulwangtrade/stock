/**
 * Phase9-B.6 Holding Migration Gate selftest.
 * Run: node frontend/src/utils/riskintel/holdingMigrationGate.selftest.mjs
 */

import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import { buildRiskAdviceProjection } from './riskAdviceProjection.js'
import { getHoldingDecision } from './holdingDecisionAdapter.js'
import { resetHoldingDecisionAdapterMetrics } from './holdingDecisionAdapterMetrics.js'
import {
  formatHoldingMigrationGateReport,
  getHoldingMigrationGateSnapshot,
  getHoldingMigrationTrend,
  resetHoldingMigrationGateHistory,
} from './holdingMigrationGate.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function mkProj(level) {
  const key = `level${level}`
  const shadow = buildScanRiskAdviceShadow({
    stockCode: 'sz000001',
    marketModeKey: key,
    effectiveMarketMode: { key, level },
    asOf: '2026-07-29T00:00:00.000Z',
  })
  return buildRiskAdviceProjection(shadow.riskAdvice, {
    symbol: 'sz000001',
    position: { costPrice: 10, costVolume: 100 },
    riskContext: shadow.riskContext,
  }).projection
}

function run() {
  resetHoldingDecisionAdapterMetrics()
  resetHoldingMigrationGateHistory()

  const p3 = mkProj(3)

  // insufficient sample
  getHoldingDecision({
    holdingAdvice: { action: 'hold', marketLevel: 3 },
    riskAdviceProjection: p3,
    symbol: 'a',
  })
  const s0 = getHoldingMigrationGateSnapshot()
  assert(s0.sampleCount === 1, 'sample 1')
  assert(s0.recommendation === 'NOT_READY', 'insufficient -> NOT_READY')
  assert(s0.sourceLocked === 'riskAdvice_projection', 'C.2 authority projection')
  assert(s0.controlledSwitchEnabled === true, 'switch enabled flag')

  const t0 = getHoldingMigrationTrend()
  assert(t0.previous == null, 'no previous yet')
  assert(t0.improving === false && t0.degrading === false, 'no trend yet')

  // build improving path: many matches
  for (let i = 0; i < 40; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'hold', marketLevel: 3 },
      riskAdviceProjection: p3,
      symbol: `ok${i % 5}`,
    })
  }
  const sImprove = getHoldingMigrationGateSnapshot()
  assert(sImprove.sampleCount >= 40, 'more samples')
  assert(typeof sImprove.readiness.alignedMatchRate === 'number', 'readiness')
  assert(typeof sImprove.stability.rollingMatchRate === 'number', 'stability')
  assert(Array.isArray(sImprove.topDiffReasons), 'top reasons')
  assert(Array.isArray(sImprove.topDiffSymbols), 'top symbols')

  const tImprove = getHoldingMigrationTrend()
  assert(tImprove.previous != null, 'has previous')
  assert(tImprove.current.sampleCount >= tImprove.previous.sampleCount, 'samples grew')
  // score should improve vs early NOT_READY snapshot
  assert(tImprove.improving === true || tImprove.degrading === false, 'not degrading after matches')

  // degrading: inject sustained diffs
  for (let i = 0; i < 30; i++) {
    getHoldingDecision({
      holdingAdvice: { action: 'reduce', marketLevel: 1, suggestPct: 1 },
      riskAdviceProjection: p3,
      symbol: 'bad',
    })
  }
  const sDegrade = getHoldingMigrationGateSnapshot()
  const tDegrade = getHoldingMigrationTrend()
  assert(sDegrade.readiness.diffRate >= tImprove.current.readiness.diffRate, 'diff up')
  assert(tDegrade.degrading === true || sDegrade.recommendation !== 'READY_FOR_CONTROLLED_SWITCH', 'degrade path')

  // stable match report text
  const report = formatHoldingMigrationGateReport()
  for (const needle of [
    'Holding Migration Gate',
    'Sample:',
    'Match Rate:',
    'Diff Rate:',
    'Unavailable:',
    'Trend:',
    'Recommendation:',
  ]) {
    assert(report.includes(needle), `report ${needle}`)
  }
  assert(!report.includes('switched'), 'no switch language')

  // unstable diff concentration visible
  assert(
    sDegrade.topDiffSymbols.some((s) => s.key === 'bad')
      || sDegrade.topDiffReasons.length >= 1,
    'diff symbols/reasons present',
  )

  // C.2 controlled switch remains preferred projection (Gate does not rollback)
  assert(sDegrade.sourceLocked === 'riskAdvice_projection', 'still projection preferred')
  assert(sImprove.recommendation !== undefined, 'has recommendation')

  console.log('holdingMigrationGate.selftest: PASS')
  console.log(report)
}

run()
