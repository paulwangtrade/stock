/**
 * Selftest for buildRiskAdvice.
 * Run: node frontend/src/utils/riskintel/riskAdvice.selftest.mjs
 */

import { buildRiskContextBundle } from './riskContextBundle.js'
import {
  buildRiskAdvice,
  scanForbiddenAdviceKeys,
  ADVICE_OUTPUT_FORBIDDEN_KEYS,
} from './riskAdvice.js'
import { ADVICE_CATEGORY } from './constants.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function assertAdviceShape(advice) {
  assert('level' in advice, 'level')
  assert(typeof advice.category === 'string', 'category')
  assert(typeof advice.marketDriven === 'boolean', 'marketDriven')
  assert(Array.isArray(advice.reasons) && advice.reasons.length >= 1, 'reasons non-empty')
  for (const r of advice.reasons) {
    assert(typeof r.code === 'string' && r.code, 'reason.code')
    assert(typeof r.message === 'string' && r.message, 'reason.message')
    assert(typeof r.marketDriven === 'boolean', 'reason.marketDriven')
  }
  const hits = scanForbiddenAdviceKeys(advice)
  assert(hits.length === 0, `forbidden: ${hits.join(',')}`)
}

function run() {
  const b1 = buildRiskContextBundle({
    stockCode: 'sh600519',
    marketModeKey: 'level1',
    asOf: '2026-07-29T00:00:00.000Z',
  })
  const a1 = buildRiskAdvice(b1)
  assertAdviceShape(a1)
  assert(a1.level === 1, 'level 1')
  assert(a1.marketDriven === true, 'root marketDriven')
  assert(a1.reasons.some((r) => r.marketDriven), 'has marketDriven reason')
  assert(a1.category === ADVICE_CATEGORY.MARKET_DISCIPLINE, 'category discipline')

  const b2 = buildRiskContextBundle({
    stockCode: 'sz000001',
    effectiveMarketMode: { key: 'level2', level: 2, name: '战略撤退' },
    asOf: '2026-07-29T00:00:00.000Z',
  })
  const a2 = buildRiskAdvice(b2)
  assertAdviceShape(a2)
  assert(a2.level === 2, 'level 2')
  assert(a2.marketDriven === true, 'level2 marketDriven')
  assert(a2.reasons.some((r) => r.code === 'MARKET_LEVEL_2' && r.marketDriven), 'MARKET_LEVEL_2')

  const b3 = buildRiskContextBundle({
    stockCode: 'sz000002',
    marketModeKey: 'level3',
    asOf: '2026-07-29T00:00:00.000Z',
  })
  const a3 = buildRiskAdvice(b3)
  assertAdviceShape(a3)
  assert(a3.level === 3, 'level 3')
  assert(a3.reasons.length >= 1, 'level3 reasons')
  assert(a3.category === ADVICE_CATEGORY.HOLD_OBSERVE, 'level3 hold_observe')
  assert(a3.marketDriven === true, 'level3 marketDriven from MARKET_LEVEL_3')

  const bSparse = buildRiskContextBundle({
    stockCode: 'x',
    marketModeKey: 'unknown',
    asOf: '2026-07-29T00:00:00.000Z',
  })
  const aSparse = buildRiskAdvice(bSparse)
  assertAdviceShape(aSparse)
  assert(aSparse.level == null, 'sparse level')
  assert(aSparse.category === ADVICE_CATEGORY.NONE, 'none category')
  assert(aSparse.reasons.some((r) => r.code === 'CONTEXT_SPARSE'), 'CONTEXT_SPARSE')
  assert(aSparse.marketDriven === aSparse.reasons.some((r) => r.marketDriven), 'root consistency')

  const bRich = buildRiskContextBundle({
    stockCode: 'sz000001',
    effectiveMarketMode: { key: 'level2', level: 2, name: '战略撤退' },
    sectorFlow: { name: '银行', inflow: false, rank: 10 },
    volumeSummary: { volumeRatio: 1.6, priceUp: false },
    asOf: '2026-07-29T00:00:00.000Z',
  })
  const aRich = buildRiskAdvice(bRich)
  assertAdviceShape(aRich)
  assert(aRich.marketDriven === true, 'rich still marketDriven')
  assert(aRich.reasons.some((r) => r.code === 'SECTOR_OUTFLOW' && r.marketDriven === false), 'sector reason')

  // Consistency: root marketDriven <=> any reason
  for (const a of [a1, a2, a3, aSparse, aRich]) {
    const any = a.reasons.some((r) => r.marketDriven)
    assert(a.marketDriven === any, 'marketDriven consistency')
  }

  // Polluted options must not leak into advice
  const polluted = buildRiskAdvice(b1, {
    mustSell: true,
    sellRatio: 1,
    sellPositionPct: 1,
    addPositionPct: 0.2,
    positionAction: 'sell',
    targetPosition: 0,
    order: {},
    execution: {},
    broker: {},
    reduce: 1,
    increase: 1,
    close: true,
    liquidate: true,
  })
  assertAdviceShape(polluted)
  assert(!('sellRatio' in polluted), 'no sellRatio')
  assert(!('mustSell' in polluted), 'no mustSell')
  assert(!('reduce' in polluted), 'no reduce key')

  assert(ADVICE_OUTPUT_FORBIDDEN_KEYS.includes('sellRatio'), 'list')
  assert(ADVICE_OUTPUT_FORBIDDEN_KEYS.includes('positionAction'), 'list2')

  let threw = false
  try {
    buildRiskAdvice(null)
  } catch {
    threw = true
  }
  assert(threw, 'null bundle throws')

  console.log('riskAdvice.selftest: PASS')
}

run()
