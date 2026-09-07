/**
 * Self-contained Step1 checks for buildRiskContextBundle (no jest/vitest required).
 * Run: node frontend/src/utils/riskintel/riskContextBundle.selftest.mjs
 */

import {
  buildRiskContextBundle,
  scanForbiddenBundleKeys,
  BUNDLE_OUTPUT_FORBIDDEN_KEYS,
} from './riskContextBundle.js'
import { POLICY_VERSION, PRODUCER_WATCHLIST_SCAN } from './constants.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function run() {
  const minimal = buildRiskContextBundle({
    stockCode: 'sh600519',
    marketModeKey: 'level1',
    asOf: '2026-07-29T00:00:00.000Z',
  })

  assert(minimal.marketState.level === 1, 'level1 from marketModeKey')
  assert(minimal.marketState.modeKey === 'level1', 'modeKey')
  assert(minimal.policyVersion === POLICY_VERSION, 'policyVersion injected')
  assert(minimal.asOf === '2026-07-29T00:00:00.000Z', 'asOf')
  assert(minimal.source.producer === PRODUCER_WATCHLIST_SCAN, 'producer')
  assert(minimal.source.stockCode === 'sh600519', 'stockCode')
  assert(typeof minimal.confidence === 'number', 'confidence')
  assert(minimal.confidence >= 0 && minimal.confidence <= 1, 'confidence range')

  const rich = buildRiskContextBundle({
    stockCode: 'sz000001',
    effectiveMarketMode: { key: 'level2', level: 2, name: '战略撤退' },
    globalMarketMode: { level: 3 },
    segmentMarketMode: { level: 2 },
    marketSegment: { key: 'szMain', indexCode: '399001.SZ', short: '深' },
    sectorFlow: { name: '银行', inflow: true, rank: 3, netamount: 1e8, ratioamount: 0.02 },
    volumeSummary: { volumeRatio: 1.5, turnover: 5, priceUp: true },
    asOf: '2026-07-29T01:00:00.000Z',
  })

  assert(rich.marketState.level === 2, 'effective level')
  assert(rich.marketState.globalLevel === 3, 'globalLevel')
  assert(rich.marketState.segmentLevel === 2, 'segmentLevel')
  assert(rich.marketState.segmentKey === 'szMain', 'segmentKey')
  assert(rich.sector?.name === '银行', 'sector')
  assert(rich.stock?.volumeRatio === 1.5, 'volume summary')
  assert(rich.confidence > minimal.confidence, 'richer context → higher confidence')

  const unknown = buildRiskContextBundle({ stockCode: 'x', marketModeKey: 'unknown' })
  assert(unknown.marketState.level == null, 'unknown level null')
  assert(unknown.confidence < minimal.confidence, 'unknown lowers confidence')

  for (const sample of [minimal, rich, unknown]) {
    const hits = scanForbiddenBundleKeys(sample)
    assert(hits.length === 0, `forbidden keys found: ${hits.join(',')}`)
  }

  // Ensure builder ignores action-like input keys (must not copy through)
  const polluted = buildRiskContextBundle({
    stockCode: 'sh600000',
    marketModeKey: 'level3',
    sellRatio: 1,
    sellPositionPct: 1,
    addPositionPct: 0.2,
    holdingAdvice: { action: 'reduce', suggestPct: 1 },
    order: { side: 'sell' },
    execution: { id: 'x' },
    broker: {},
    action: 'reduce',
    asOf: '2026-07-29T02:00:00.000Z',
  })
  const polluteHits = scanForbiddenBundleKeys(polluted)
  assert(polluteHits.length === 0, `polluted input leaked: ${polluteHits.join(',')}`)
  assert(!('sellRatio' in polluted), 'no sellRatio on root')
  assert(!('holdingAdvice' in polluted), 'no holdingAdvice on root')
  assert(!('action' in polluted), 'no action on root')

  assert(BUNDLE_OUTPUT_FORBIDDEN_KEYS.includes('sellRatio'), 'forbidden list has sellRatio')
  assert(BUNDLE_OUTPUT_FORBIDDEN_KEYS.includes('sellPositionPct'), 'forbidden list has sellPositionPct')

  console.log('riskContextBundle.selftest: PASS')
}

run()
