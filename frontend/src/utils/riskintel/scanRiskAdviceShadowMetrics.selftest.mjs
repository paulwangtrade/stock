/**
 * Phase8-0 Step3-B selftest: shadow observation metrics.
 * Run: node frontend/src/utils/riskintel/scanRiskAdviceShadowMetrics.selftest.mjs
 */

import {
  buildScanRiskAdviceShadow,
} from './scanRiskAdviceShadow.js'
import {
  getScanRiskAdviceShadowMetrics,
  recordScanRiskAdviceShadowFailed,
  resetScanRiskAdviceShadowMetrics,
  setScanRiskAdviceShadowMetricsEnabled,
} from './scanRiskAdviceShadowMetrics.js'
import { scanForbiddenAdviceKeys } from './riskAdvice.js'

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function run() {
  resetScanRiskAdviceShadowMetrics()

  const mk = (level, key) => ({
    stockCode: `sz00000${level}`,
    marketModeKey: key,
    effectiveMarketMode: { key, level, name: `L${level}` },
    asOf: '2026-07-29T00:00:00.000Z',
  })

  const s1 = buildScanRiskAdviceShadow(mk(1, 'level1'))
  const s2 = buildScanRiskAdviceShadow(mk(2, 'level2'))
  const s3 = buildScanRiskAdviceShadow(mk(3, 'level3'))
  assert(s1 && s2 && s3, 'three shadows')

  // Forbidden fields still blocked on outputs
  for (const s of [s1, s2, s3]) {
    assert(scanForbiddenAdviceKeys(s.riskAdvice).length === 0, 'advice clean')
    for (const k of ['mustSell', 'sellRatio', 'positionAction', 'order', 'execution']) {
      assert(!(k in s.riskAdvice), `no ${k}`)
    }
  }

  let m = getScanRiskAdviceShadowMetrics()
  assert(m.generated === 3, `generated=${m.generated}`)
  assert(m.failed === 0, 'no fails yet')
  assert(m.levelDistribution['1'] === 1, 'level1 dist')
  assert(m.levelDistribution['2'] === 1, 'level2 dist')
  assert(m.levelDistribution['3'] === 1, 'level3 dist')
  assert(m.categoryFrequency.market_discipline >= 2, 'discipline tags')
  assert(m.reasonCodeFrequency.MARKET_LEVEL_1 === 1, 'reason tag MARKET_LEVEL_1')
  assert(m.riskTagFrequency.market_discipline >= 2, 'riskTagFrequency category')
  assert(m.riskTagFrequency['reason:MARKET_LEVEL_1'] === 1, 'riskTagFrequency reason')

  // Failure observation (exception path)
  const failed = buildScanRiskAdviceShadow(null)
  assert(failed.failReason === 'context_missing' && !failed.riskAdvice, 'context missing shadow')
  m = getScanRiskAdviceShadowMetrics()
  assert(m.failed === 1, 'failed count')
  assert(m.failReasons.context_missing === 1, 'context_missing reason')
  assert(m.generated === 3, 'generated unchanged on fail')

  // Forbidden-fields blocked path (metrics API; builder never emits these)
  recordScanRiskAdviceShadowFailed('forbidden_fields', ['mustSell', 'sellRatio'])
  m = getScanRiskAdviceShadowMetrics()
  assert(m.failed === 2, 'failed after forbidden')
  assert(m.failReasons.forbidden_fields === 1, 'forbidden reason')
  assert(m.forbiddenBlocked === 1, 'forbiddenBlocked')
  assert(m.riskTagFrequency['reason:FORBIDDEN:mustSell'] === 1, 'forbidden tag obs')

  // Disable: no further bumps
  setScanRiskAdviceShadowMetricsEnabled(false)
  buildScanRiskAdviceShadow(mk(1, 'level1'))
  m = getScanRiskAdviceShadowMetrics()
  assert(m.generated === 3, 'disabled skips generate')
  setScanRiskAdviceShadowMetricsEnabled(true)

  // Snapshot must not look like an execution payload
  const snap = getScanRiskAdviceShadowMetrics()
  for (const k of ['mustSell', 'sellRatio', 'positionAction', 'order', 'execution', 'broker']) {
    assert(!(k in snap), `metrics snapshot no ${k}`)
  }

  resetScanRiskAdviceShadowMetrics()
  assert(getScanRiskAdviceShadowMetrics().generated === 0, 'reset')

  console.log('scanRiskAdviceShadowMetrics.selftest: PASS')
}

run()
