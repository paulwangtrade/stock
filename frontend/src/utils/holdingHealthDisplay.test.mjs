import assert from 'node:assert/strict'
import {
  buildHealthReasonSummary,
  healthFactorLabelZH,
  healthGradeListLabel,
  healthGradeTagType,
} from './holdingHealthDisplay.js'

assert.equal(healthGradeListLabel('A'), 'A 健康')
assert.equal(healthGradeListLabel('B'), 'B 观察')
assert.equal(healthGradeListLabel('C'), 'C 关注')
assert.equal(healthGradeListLabel('D'), 'D 风险')
assert.equal(healthGradeTagType('A'), 'success')
assert.equal(healthGradeTagType('D'), 'error')
assert.equal(healthFactorLabelZH('SIGNAL_ACTIVE'), '信号有效')
assert.equal(healthFactorLabelZH('PRICE_STALE'), '数据过期')

const lines = buildHealthReasonSummary({
  supportingFactors: ['SIGNAL_ACTIVE', 'TREND_SUPPORT'],
  riskFactors: ['PRICE_STALE'],
})
assert.deepEqual(
  lines.map((x) => x.text),
  ['✓ 信号有效', '✓ 趋势支持', '⚠ 数据过期'],
)
assert.equal(lines[2].kind, 'warn')

console.log('holdingHealthDisplay.test.mjs: ok')
