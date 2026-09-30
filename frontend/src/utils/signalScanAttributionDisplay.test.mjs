/**
 * Run: node frontend/src/utils/signalScanAttributionDisplay.test.mjs
 */
import assert from 'node:assert/strict'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const __dir = dirname(fileURLToPath(import.meta.url))
const mod = await import(pathToFileURL(join(__dir, 'signalScanAttributionDisplay.js')).href)
const {
  ATTRIBUTION_DISCLAIMER,
  RESEARCH_STAT_LABEL,
  LARGE_SAMPLE_WARNING,
  horizonCellText,
  horizonReasonText,
  horizonSummaryText,
  strategyCell,
  displayName,
  findHorizon,
  buildAttributionSignalTagOptions,
} = mod

assert.equal(ATTRIBUTION_DISCLAIMER, '仅研究对照，不构成交易建议，不进入模拟交易计划')
assert.equal(/买入|推荐|alpha/i.test(ATTRIBUTION_DISCLAIMER + RESEARCH_STAT_LABEL), false)
assert.match(RESEARCH_STAT_LABEL, /不是收益证明/)

assert.equal(horizonCellText(null), '数据不足')
assert.equal(
  horizonCellText({ status: 'insufficient', text: '数据不足', returnRate: 0.5 }),
  '数据不足',
)
assert.equal(horizonCellText({ status: 'ok', text: '+1.00%', returnRate: 0.01 }), '+1.00%')
assert.equal(horizonReasonText({ status: 'insufficient', reason: 'calendar_gap' }), '日线之间有未确认的交易日缺口')

assert.equal(strategyCell({ strategyName: '策略甲', strategyId: 's1' }), '策略甲（s1）')
assert.equal(strategyCell({ strategyId: 's1' }), 's1')
assert.equal(strategyCell({}), '默认参数预设（default）')
assert.equal(strategyCell({ strategyId: 'default' }, { default: '我的预设' }), '我的预设（default）')
assert.equal(strategyCell({ strategyName: '快照名', strategyId: 'default' }, { default: '我的预设' }), '快照名（default）')
assert.equal(LARGE_SAMPLE_WARNING, '样本过大，先收窄信号再归因')
assert.deepEqual(buildAttributionSignalTagOptions(['强', 'XS_MOM_TOP']), [
  { value: '强', label: '强' },
  { value: 'XS_MOM_TOP', label: 'XS_MOM_TOP 截面动量' },
])
assert.equal(displayName(''), '—')
assert.equal(displayName('浦发银行'), '浦发银行')

assert.equal(horizonSummaryText({ complete: 0, meanText: '+9.00%' }), '完整样本 0')
assert.match(horizonSummaryText({ complete: 2, meanText: '+1.00%', medianText: '+1.00%' }), /均值 \+1\.00%/)

const row = { horizons: [{ horizon: 3, status: 'ok', text: '+3.00%' }] }
assert.equal(findHorizon(row, 3).text, '+3.00%')
assert.equal(findHorizon(row, 10), null)
