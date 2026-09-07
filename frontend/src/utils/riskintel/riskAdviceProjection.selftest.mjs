/**
 * Phase8.6 RiskAdvice Projection selftest.
 * Run: node frontend/src/utils/riskintel/riskAdviceProjection.selftest.mjs
 */

import { buildScanRiskAdviceShadow } from './scanRiskAdviceShadow.js'
import {
  buildRiskAdviceProjection,
  compareLegacyHoldingToProjection,
  RISK_ADVICE_PROJECTION_SOURCE,
} from './riskAdviceProjection.js'
import {
  getRiskAdviceProjectionMetrics,
  resetRiskAdviceProjectionMetrics,
} from './riskAdviceProjectionMetrics.js'
import { resetScanRiskAdviceShadowObservation } from './scanRiskAdviceShadowEvaluation.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function run() {
  resetScanRiskAdviceShadowObservation()
  resetRiskAdviceProjectionMetrics()

  const bundleL1 = {
    stockCode: 'sh600519',
    marketModeKey: 'level1',
    effectiveMarketMode: { key: 'level1', level: 1, name: 'level1' },
    asOf: '2026-07-29T00:00:00.000Z',
  }
  const shadow = buildScanRiskAdviceShadow(bundleL1)
  assert(shadow.riskAdvice, 'riskAdvice exists')

  // 1) normal conversion
  const ok = buildRiskAdviceProjection(shadow.riskAdvice, {
    symbol: 'sh600519',
    position: { costPrice: 1800, costVolume: 100, profitPct: 2.5 },
    riskContext: shadow.riskContext,
  })
  assert(ok.ok, 'projection ok')
  assert(ok.projection.source === RISK_ADVICE_PROJECTION_SOURCE, 'source')
  assert(ok.projection.symbol === 'sh600519', 'symbol')
  assert(ok.projection.projectedLevel === 1, 'projectedLevel')
  assert(ok.projection.projectedAction === 'reduce', 'projectedAction reduce')
  assert(Array.isArray(ok.projection.riskReasons) && ok.projection.riskReasons.length >= 1, 'reasons')
  assert(ok.projection.currentHoldingState.hasPosition === true, 'holding state')
  assert(!('mustSell' in ok.projection), 'no mustSell')
  assert(!('sellRatio' in ok.projection), 'no sellRatio')
  assert(!('order' in ok.projection), 'no order')
  assert(!('execution' in ok.projection), 'no execution')

  // 2) missing context (no symbol anywhere)
  const noSymbolAdvice = {
    level: 3,
    category: 'hold_observe',
    marketDriven: true,
    reasons: [{ code: 'X', message: 'x', marketDriven: true }],
    confidence: 0.5,
  }
  const missing = buildRiskAdviceProjection(noSymbolAdvice, { position: {} })
  assert(!missing.ok && missing.failReason === 'missing_symbol', 'missing symbol fail')
  assert(missing.missingFields.includes('symbol'), 'missing fields has symbol')

  // 3) empty riskAdvice
  const empty = buildRiskAdviceProjection(null, { symbol: 'sz000001' })
  assert(!empty.ok && empty.failReason === 'empty_risk_advice', 'empty fail')

  // 4) multi symbol
  const codes = ['sh600519', 'sz000001', 'sz000002']
  for (const code of codes) {
    const level = code.endsWith('002') ? 3 : 1
    const key = `level${level}`
    const s = buildScanRiskAdviceShadow({
      stockCode: code,
      marketModeKey: key,
      effectiveMarketMode: { key, level },
      asOf: '2026-07-29T00:00:00.000Z',
    })
    const p = buildRiskAdviceProjection(s.riskAdvice, {
      symbol: code,
      position: { costPrice: 10, costVolume: 100 },
      riskContext: s.riskContext,
    })
    assert(p.ok && p.projection.symbol === code, `multi ${code}`)
  }

  // 5) legacy vs projection对照
  const legacy = { action: 'reduce', marketLevel: 1, suggestPct: 1, actionLabel: '清仓防守' }
  const cmp = compareLegacyHoldingToProjection(legacy, ok.projection)
  assert(cmp.levelMatch === true, 'legacy level match')
  assert(cmp.actionMatch === true, 'legacy action match')
  // projection must not mutate legacy
  assert(legacy.suggestPct === 1 && legacy.actionLabel === '清仓防守', 'legacy untouched')

  const metrics = getRiskAdviceProjectionMetrics()
  assert(metrics.generated >= 4, `generated=${metrics.generated}`)
  assert(metrics.failed >= 2, `failed=${metrics.failed}`)
  assert(metrics.missingFieldCount >= 1, 'missingFieldCount')

  console.log('riskAdviceProjection.selftest: PASS')
  console.log('projection metrics:', metrics)
}

run()
